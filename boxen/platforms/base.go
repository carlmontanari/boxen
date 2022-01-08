package platforms

import "github.com/carlmontanari/boxen/boxen/instance"

type Platform interface {
	// Base embeds Install, Start, Stop and RunUntilSigInt methods that should be common for any
	// Platform. Platforms may (probably) will need to override Install and Start in order to pass
	// the appropriate options to modify launch commands and the like.
	instance.Base

	// Package builds any necessary files for instance installation/start such as a config file or
	// a similar and checks that the srcDir contains any files that must be included for the
	// instance such as bios files or bootloaders etc. The `pkgFiles` are files that should be
	// included during initial packaging (of either the initial build container, or the local temp
	// directory in the case of local VMs). The `runFiles` are files that are required to actually
	// run the device, and therefore should be collocated with the final disk -- since all files get
	// copied to the container in the case of a "package" build, this is specific to local VM mode.
	Package(srcDir, pkgDir string) (pkgFiles []string, runFiles []string, err error)

	// Config sends some lines of configs to the device -- *probably* via console, but up to the
	// platform how that happens. Hopefully this will be satisfied by ScrapliConsole being embedded
	// in most platform cases.
	Config(lines []string) error
	// InstallConfig "installs" a (usually) startup config on the device. This is once again up to
	// the platform how this is implemented, but for "core" platforms this will be done via scrapli
	// "cfg" functionality to handle config *replaces* or, optionally, merge operations. The config
	// to be installed is whatever is in the file path `f`.
	InstallConfig(f string, replace bool) error

	// Detach closes any connections to the instance -- *probably* this means it closes the console
	// connection but the base qemu instance doesn't need to know/care if its console or something
	// else entirely. Hopefully this will be satisfied by ScrapliConsole being embedded in most
	// platform cases.
	Detach() error

	// SaveConfig saves the config of the instance. Platforms *should* implement some kind of check
	// and/or backoff that ensures that configs are able to be saved -- meaning that some devices do
	// not allow for configurations to be saved immediately after startup -- this method *should*
	// handle that and sleep for some duration before trying again!
	SaveConfig() error

	// SetUserPass sets the username/password -- used for "package start" mode.
	SetUserPass(usr, pwd string) error
	// SetHostname sets the hostname -- used for "package start" mode.
	SetHostname(h string) error

	// GetPid returns the instances pid or -1.
	GetPid() int
}
