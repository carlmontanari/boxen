package boxen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/carlmontanari/boxen/boxen"
	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/docker"
	"github.com/carlmontanari/boxen/boxen/logging"
	"github.com/carlmontanari/boxen/boxen/platforms"
	"github.com/carlmontanari/boxen/boxen/util"
)

type dockerfileTemplateData struct {
	LocalHost         string
	RequiredFiles     []string
	ExposedTCPPorts   []int
	ExposedUDPPorts   []int
	TimeoutMultiplier int
	LogLevel          string
	Sparsify          int
	BoxenVersion      string
	BinaryOverride    bool
}

type dockerignoreTemplateData struct {
	RequiredFiles []string
}

func writeDockerfiles(
	dir string,
	inclFiles []string,
	exposedTCPPorts, exposedUDPPorts []int,
	timeoutModifier int,
	binaryOverride bool,
) error {
	inclFiles = append(inclFiles, "disk.qcow2")

	buildDockerfileTpl, err := template.ParseFS(
		boxen.Assets,
		"assets/packaging/build.Dockerfile.template",
	)
	if err != nil {
		return err
	}

	dockerfileTpl, err := template.ParseFS(boxen.Assets, "assets/packaging/Dockerfile.template")
	if err != nil {
		return err
	}

	dockerignoreTpl, err := template.ParseFS(boxen.Assets, "assets/packaging/dockerignore.template")
	if err != nil {
		return err
	}

	dockerfileData := &dockerfileTemplateData{
		LocalHost:         util.GetPreferredIP(),
		RequiredFiles:     inclFiles,
		ExposedTCPPorts:   exposedTCPPorts,
		ExposedUDPPorts:   exposedUDPPorts,
		TimeoutMultiplier: timeoutModifier,
		LogLevel:          util.GetEnvStrOrDefault("BOXEN_LOG_LEVEL", "info"),
		Sparsify:          util.GetEnvIntOrDefault("BOXEN_SPARSIFY_DISK", 0),
		BoxenVersion:      Version,
		BinaryOverride:    binaryOverride,
	}

	dockerignoreData := &dockerignoreTemplateData{
		RequiredFiles: inclFiles,
	}

	buildDockerfileDest, err := os.Create(fmt.Sprintf("%s/build.Dockerfile", dir))
	if err != nil {
		return err
	}

	dockerfileDest, err := os.Create(fmt.Sprintf("%s/Dockerfile", dir))
	if err != nil {
		return err
	}

	dockerignoreDest, err := os.Create(fmt.Sprintf("%s/.dockerignore", dir))
	if err != nil {
		return err
	}

	_ = buildDockerfileTpl.Execute(buildDockerfileDest, dockerfileData)
	_ = dockerfileTpl.Execute(dockerfileDest, dockerfileData)
	_ = dockerignoreTpl.Execute(dockerignoreDest, dockerignoreData)

	return nil
}

func (b *Boxen) packageBundle(
	i *installInfo,
	inst platforms.Platform,
) error {
	packageFiles, _, err := inst.Package(filepath.Dir(i.inDisk), i.tmpDir)
	if err != nil {
		return err
	}

	c := config.NewPackageConfig()

	platformDefaultProfile, err := GetDefaultProfile(i.srcDisk.PlatformType)
	if err != nil {
		return err
	}

	tcpNats, udpNats := ZipPlatformProfileNats(
		platformDefaultProfile.TPCNatPorts,
		platformDefaultProfile.UDPNatPorts,
	)

	c.Instances[i.srcDisk.PlatformType] = &config.Instance{
		Name:         i.srcDisk.PlatformType,
		PlatformType: i.srcDisk.PlatformType,
		Disk:         "disk.qcow2",
		ID:           1,
		PID:          0,
		Profile:      "",
		Credentials:  config.NewDefaultCredentials(),
		Hardware:     platformDefaultProfile.Hardware.ToHardware(),
		MgmtIntf: &config.MgmtIntf{
			Nat: &config.Nat{
				TCP: tcpNats,
				UDP: udpNats,
			},
			Bridge: nil,
		},
		DataPlaneIntf: nil,
		Advanced:      platformDefaultProfile.Advanced,
	}

	if i.username != "" {
		c.Instances[i.srcDisk.PlatformType].Credentials.Username = i.username
	}

	if i.password != "" {
		c.Instances[i.srcDisk.PlatformType].Credentials.Password = i.password
	}

	c.Instances[i.srcDisk.PlatformType].Hardware.MonitorPort = b.allocateMonitorPort(1)

	c.Instances[i.srcDisk.PlatformType].Hardware.SerialPorts, err = b.allocateSerialPorts(
		platformDefaultProfile.Hardware.SerialPortCount,
		1,
	)
	if err != nil {
		return err
	}

	err = util.CopyAsset("packaging/tc-tap-ifup", fmt.Sprintf("%s/tc-tap-ifup", i.tmpDir))
	if err != nil {
		return err
	}

	usrBoxenBinary := util.GetEnvStrOrDefault("BOXEN_PACKAGE_BINARY", "")
	binaryOverride := false

	if usrBoxenBinary != "" {
		b.Logger.Debugf(
			"user provided boxen binary at path '%s', copying to temp dir",
			usrBoxenBinary,
		)

		err = util.CopyFile(usrBoxenBinary, fmt.Sprintf("%s/boxen", i.tmpDir))
		if err != nil {
			return err
		}

		binaryOverride = true
	}

	err = writeDockerfiles(
		i.tmpDir,
		packageFiles,
		platformDefaultProfile.TPCNatPorts,
		platformDefaultProfile.UDPNatPorts,
		util.GetTimeoutMultiplier(),
		binaryOverride,
	)
	if err != nil {
		return err
	}

	err = c.Dump(fmt.Sprintf("%s/boxen.yaml", i.tmpDir))
	if err != nil {
		return err
	}

	return os.Rename(i.newDisk, fmt.Sprintf("%s/disk.qcow2", i.tmpDir))
}

func (b *Boxen) fileServer(wg *sync.WaitGroup, packageDir string) *http.Server {
	srv := &http.Server{Addr: ":6666"}

	http.Handle("/", http.FileServer(http.Dir(packageDir)))

	go func() {
		defer wg.Done()

		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			b.Logger.Criticalf("file server error: %v", err)
		}
	}()

	return srv
}

func (b *Boxen) buildBaseImage(i *installInfo, repo, tag string) error {
	f, err := os.OpenFile(
		fmt.Sprintf("%s/initial_build.log", i.tmpDir),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		util.FilePerms,
	)
	if err != nil {
		b.Logger.Criticalf("error opening initial build log file: %s", err)

		return err
	}

	defer f.Close()

	err = docker.Build(
		docker.WithWorkDir(i.tmpDir),
		docker.WithDockerfile("build.Dockerfile"),
		docker.WithRepo(repo),
		docker.WithTag(tag),
		docker.WithStdErr(f),
		docker.WithStdOut(f),
		docker.WithNoCache(true),
	)
	if err != nil {
		b.Logger.Criticalf("error starting build container: %s", err)

		return err
	}

	err = os.Remove(fmt.Sprintf("%s/disk.qcow2", i.tmpDir))
	if err != nil {
		b.Logger.Criticalf("error removing initial install disk: %s", err)
	}

	return err
}

func (b *Boxen) runInstallImage(i *installInfo, repo, tag string) error {
	f, err := os.OpenFile(
		fmt.Sprintf("%s/install_build.log", i.tmpDir),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		util.FilePerms,
	)
	if err != nil {
		b.Logger.Criticalf("error opening install build log file: %s", err)

		return err
	}

	defer f.Close()

	// SocketReceiver accepts the "normal" (boxen process, not instance stuff) logs and pumps that
	// into the "normal" local boxen logger.
	sr, err := logging.NewSocketReceiver(util.GetPreferredIP(), 6667, b.Logger) //nolint: gomnd
	if err != nil {
		b.Logger.Criticalf("error starting socket log receiver: %s", err)

		return err
	}

	defer sr.Close()

	r, err := docker.Run(
		docker.WithCidFile("install_cidfile"),
		docker.WithPrivileged(true),
		docker.WithRepo(repo),
		docker.WithTag(tag),
		docker.WithWorkDir(i.tmpDir),
		docker.WithStdErr(f),
		docker.WithStdOut(f),
	)
	if err != nil {
		return err
	}

	err = r.CheckStdErr()
	if err != nil {
		return err
	}

	b.Logger.Infof(
		"install logs available at '%s/install_build.log', or by inspect container '%s' logs",
		i.tmpDir,
		docker.ReadCidFile(fmt.Sprintf("%s/install_cidfile", i.tmpDir)),
	)

	waitStdoutW := &bytes.Buffer{}

	err = docker.Wait(
		docker.WithContainer(docker.ReadCidFile(fmt.Sprintf("%s/install_cidfile", i.tmpDir))),
		docker.WithStdOut(waitStdoutW),
	)
	if err != nil {
		b.Logger.Criticalf("error waiting for installation container to exit: %s", err)

		return err
	}

	if waitStdoutW.Len() > 0 {
		// we got some stdout, we want to check if its a zero (48) or a 1 (49) to know what the
		// containers exit code was (we exit with 1 in main.go if we encounter an error, so that
		// would be what the container returns). we should also always get a newline (10).
		if !bytes.Equal(waitStdoutW.Bytes(), []byte{48, 10}) {
			return fmt.Errorf(
				"%w: docker wait indicates install container exited with non-zero exit code",
				util.ErrCommandError,
			)
		}
	}

	err = docker.CopyFromContainer(
		"disk.qcow2",
		"disk.qcow2",
		docker.WithContainer(docker.ReadCidFile(fmt.Sprintf("%s/install_cidfile", i.tmpDir))),
		docker.WithWorkDir(i.tmpDir),
	)
	if err != nil {
		b.Logger.Criticalf("error copying disk from installation container: %s", err)

		return err
	}

	err = docker.RmContainer(docker.ReadCidFile(fmt.Sprintf("%s/install_cidfile", i.tmpDir)))
	if err != nil {
		b.Logger.Criticalf("error removing installation container: %s", err)
	}

	// want to nuke the initial build image now that we have copied all the stuff out of the
	// "install" container.
	err = docker.RmImage(repo, tag)
	if err != nil {
		b.Logger.Criticalf("error removing initial build image: %s", err)
	}

	return err
}

func (b *Boxen) buildFinalImage(i *installInfo, repo, tag string) error {
	f, err := os.OpenFile(
		fmt.Sprintf("%s/final_build.log", i.tmpDir),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		util.FilePerms,
	)
	if err != nil {
		return err
	}

	defer f.Close()

	err = docker.Build(
		docker.WithWorkDir(i.tmpDir),
		docker.WithDockerfile("Dockerfile"),
		docker.WithRepo(repo),
		docker.WithTag(tag),
		docker.WithStdErr(f),
		docker.WithStdOut(f),
		docker.WithNoCache(true),
	)

	b.Logger.Infof(
		"final image build logs available at '%s/final_build.log'",
		i.tmpDir,
	)

	return err
}

func (b *Boxen) packageBuildPreContainer(i *installInfo) error {
	var err error

	i.tmpDir, err = ioutil.TempDir(os.TempDir(), "boxen")
	if err != nil {
		b.Logger.Criticalf("error creating temporary working directory: %s\n", err)

		return err
	}

	b.Logger.Debugf("temporary directory '%s' created successfully", i.tmpDir)

	err = b.installAllocateDisks(i)
	if err != nil {
		b.Logger.Criticalf("error allocating disks for packaging: %s", err)

		return err
	}

	b.Logger.Debug("disks allocated for packaging")

	inst, err := platforms.GetPlatformEmptyStruct(i.srcDisk.PlatformType)
	if err != nil {
		b.Logger.Criticalf("error instantiating new instance for packaging: %s", err)

		return err
	}

	b.Logger.Debug("packaging instance created")

	err = b.packageBundle(i, inst)
	if err != nil {
		b.Logger.Criticalf("error bundling required packaging files: %s", err)

		return err
	}

	b.Logger.Debug("bundling required packaging files complete")

	return nil
}

func (b *Boxen) packageBuildContainer(i *installInfo, repo, tag string) error {
	var err error

	if util.GetEnvIntOrDefault("BOXEN_DEV_MODE", 0) > 0 {
		b.Logger.Info(
			"boxen dev mode enabled, not deleting temporary directory after installation",
		)
	} else {
		defer os.RemoveAll(i.tmpDir)
	}

	b.Logger.Debug("pre packaging complete, begin docker-ization!")

	b.Logger.Infof("docker build output available at '%s/initial_build.log'", i.tmpDir)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	srv := b.fileServer(wg, i.tmpDir)

	err = b.buildBaseImage(i, repo, tag)
	if err != nil {
		b.Logger.Criticalf("error building base image: %s", err)

		return err
	}

	b.Logger.Debug("base image building complete!")

	err = b.runInstallImage(i, repo, tag)
	if err != nil {
		b.Logger.Criticalf("error running instance installation: %s", err)

		return err
	}

	b.Logger.Debug("instance installation complete!")

	err = b.buildFinalImage(i, repo, tag)
	if err != nil {
		b.Logger.Criticalf("error building final image: %s", err)

		return err
	}

	_ = srv.Shutdown(context.TODO())

	wg.Wait()

	b.Logger.Debug("packaging complete!")

	return nil
}

// PackageBuild is the initial entrypoint for "packaging" an instance as a container image. This
// function will create a copy of the provided disk, build an install container image, then run that
// image which will kick off the PackageInstall function. After installation (dealing with the
// initial prompts and installing a base config and such) is complete, the disk image is copied out
// of the install container, the install container is then destroyed, with the initial build image.
// Finally, this function will build a final container image, copying in the provisioned disk into
// the final image.
func (b *Boxen) PackageBuild(
	disk, username, password, repo, tag, vendor, platform, version string,
) error {
	b.Logger.Infof("package requested for disk '%s'", disk)

	i := &installInfo{inDisk: disk, username: username, password: password}

	err := b.handleProvidedPlatformInfo(i, vendor, platform, version)
	if err != nil {
		return err
	}

	err = b.packageBuildPreContainer(i)
	if err != nil {
		return err
	}

	if repo == "" {
		repo = fmt.Sprintf("boxen_%s", i.srcDisk.PlatformType)
	} else {
		repo = strings.ToLower(repo)
	}

	if tag == "" {
		tag = i.srcDisk.Version
	}

	return b.packageBuildContainer(i, repo, tag)
}
