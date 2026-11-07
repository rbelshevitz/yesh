//go:build navigator

package main

import (
	"fmt"
	"os"

	"yesh/internal/version"
	"yesh/internal/yeshcore"
)

func main() {
	os.Exit(RunNavigator())
}

func navigatorVersionReport() (string, error) {
	navigatorVersion := version.Version
	coreVersion := yeshcore.Version()
	if err := version.CheckCompatibility(navigatorVersion, coreVersion); err != nil {
		return "", err
	}
	return version.NavigatorOutput(navigatorVersion, coreVersion), nil
}

func navigatorVersionRequested() bool {
	for _, argument := range os.Args[1:] {
		if argument == "--version" {
			return true
		}
	}
	return false
}

func runNavigatorVersion() int {
	report, err := navigatorVersionReport()
	if err != nil {
		fmt.Fprintln(os.Stderr, "yesh navigator:", err)
		return 1
	}
	fmt.Print(report)
	return 0
}
