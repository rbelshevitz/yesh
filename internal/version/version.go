package version

import (
	_ "embed"
	"fmt"
	"strings"
)

// versionFile is the one canonical project version. It is embedded into every
// binary through this package; neither command has its own version constant.
//
//go:embed VERSION
var versionFile string

// Version is shared by every yesh binary built from this source tree.
var Version = strings.TrimSpace(versionFile)

const versionLabelWidth = 16

// NavigatorOutput returns the stable, column-aligned navigator version report.
func NavigatorOutput(navigatorVersion, coreVersion string) string {
	return fmt.Sprintf("%-*s%s\n%-*s%s\n",
		versionLabelWidth, "yesh navigator",
		navigatorVersion,
		versionLabelWidth, "parser core",
		coreVersion,
	)
}

// CheckCompatibility rejects a navigator and parser core built with different
// versions before the navigator starts interacting with the terminal.
func CheckCompatibility(navigatorVersion, coreVersion string) error {
	if navigatorVersion == coreVersion {
		return nil
	}
	return fmt.Errorf("yesh navigator/parser core version mismatch: navigator %q, parser core %q", navigatorVersion, coreVersion)
}
