package platforms

import (
	"fmt"
	"regexp"

	"github.com/carlmontanari/boxen/boxen/util"
)

func diskToVendorPlatformMap() map[*regexp.Regexp][]string {
	return map[*regexp.Regexp][]string{
		regexp.MustCompile(`csr1000v-.*.qcow2`):              {"cisco", "csr1000v"},
		regexp.MustCompile(`(?i)xrv9k-fullk9-x.*.qcow2`):     {"cisco", "xrv9k"},
		regexp.MustCompile(`(?i)(nexus9300v|nxosv).*.qcow2`): {"cisco", "n9kv"},
		regexp.MustCompile(`vEOS-lab-.*.vmdk`):               {"arista", "veos"},
		regexp.MustCompile(`media-vsrx-vmdisk.*.qcow2`):      {"juniper", "vsrx"},
		regexp.MustCompile(`PA-VM-KVM.*.qcow2`):              {"paloalto", "panos"},
	}
}

func GetPlatformTypeFromDisk(f string) (vendor, platform string, err error) {
	dToPtMap := diskToVendorPlatformMap()

	for pattern, pT := range dToPtMap {
		if pattern.MatchString(f) {
			// target disk matches this vendor/platform pair
			return pT[0], pT[1], nil
		}
	}

	return "", "", fmt.Errorf(
		"%w: cannot resolve target platform type from provided disk",
		util.ErrInspectionError,
	)
}

func pTDiskToVersionMap() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		PlatformTypeAristaVeos:    regexp.MustCompile(`(?i)(\d+\.\d+\.[a-z0-9\-]+(\.\d+[a-z]?)?)`),
		PlatformTypeCiscoCsr1000v: regexp.MustCompile(`(?i)(?:universalk9.*?)(\d+\.\d+\.\d+)`),
		PlatformTypeCiscoXrv9k: regexp.MustCompile(
			`(?i)(?:xrv9k-fullk9-x\.vrr-)(\d+\.\d+\.\d+)`,
		),
		PlatformTypeCiscoN9kv: regexp.MustCompile(
			`(?i)(?:(?:nexus9300v|nxosv)\.)(\d+\.\d+\.\d+)`,
		),
		PlatformTypeJuniperVsrx: regexp.MustCompile(
			`(?i)(?:media-vsrx-vmdisk-)(\d+\.\w+\.\d+).qcow2`,
		),
		PlatformTypePaloAltoPanos: regexp.MustCompile(
			`(?i)(?:pa-vm-kvm-)(\d+\.\d+\.\d+).qcow2`),
	}
}

func GetDiskVersion(
	f, pT string,
) (string, error) {
	targetVersionMap := pTDiskToVersionMap()

	pattern := targetVersionMap[pT]

	diskVersionMatches := pattern.FindStringSubmatch(f)

	if len(diskVersionMatches) == 0 {
		return "", fmt.Errorf(
			"%w: cannot determine version from provided disk",
			util.ErrInspectionError,
		)
	}

	return diskVersionMatches[1], nil
}
