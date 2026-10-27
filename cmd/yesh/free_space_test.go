//go:build !windows

package main

import (
	"path/filepath"
	"testing"
)

func TestFreeSpaceConditionUsesFilesystemStatistics(t *testing.T) {
	dir := t.TempDir()
	r := &runtime{values: map[string]value{"path": {scalar: dir}}, filename: "free-space.yesh"}
	ok, err := r.condition("free_space path at_least 1B", 1)
	if err != nil || !ok {
		t.Fatalf("available space: ok=%v err=%v", ok, err)
	}
	ok, err = r.condition("free_space path at_least 999999TiB", 2)
	if err != nil || ok {
		t.Fatalf("unavailable space: ok=%v err=%v", ok, err)
	}
	missing := filepath.Join(dir, "missing")
	r.values["path"] = value{scalar: missing}
	if _, err := r.condition("free_space path at_least 1B", 3); err == nil {
		t.Fatal("accepted missing path")
	}
}

func TestParseByteSizeRejectsAmbiguousValues(t *testing.T) {
	for _, text := range []string{"20GiB", "1MiB"} {
		if _, err := parseByteSize(text); err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
	}
	for _, text := range []string{"0B", "20 GB", "1.5GiB", "-1GiB", "20KB", "GiB"} {
		if _, err := parseByteSize(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}
