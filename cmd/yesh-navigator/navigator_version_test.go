//go:build navigator

package main

import (
	"fmt"
	"os"
	"testing"

	"yesh/internal/version"
)

func TestNavigatorVersionReportFormatting(t *testing.T) {
	got, err := navigatorVersionReport()
	if err != nil {
		t.Fatalf("version report: %v", err)
	}
	want := fmt.Sprintf("yesh navigator  %s\nparser core     %s\n", version.Version, version.Version)
	if got != want {
		t.Fatalf("version report = %q, want %q", got, want)
	}
}

func TestNavigatorVersionRequested(t *testing.T) {
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	for _, argument := range []string{"--version", "-v", "-version"} {
		os.Args = []string{"yesh-navigator", argument}
		if !navigatorVersionRequested() {
			t.Fatalf("%s was not recognized", argument)
		}
	}
}
