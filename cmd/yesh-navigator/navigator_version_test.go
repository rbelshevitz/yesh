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
	os.Args = []string{"yesh-navigator", "--version"}
	if !navigatorVersionRequested() {
		t.Fatal("--version was not recognized")
	}
}
