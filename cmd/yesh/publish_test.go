package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishRenamesWithoutMerge(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "temporary")
	dest := filepath.Join(dir, "current")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "file"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	items, err := parse("publish $source to $dest\n", "test.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r.values["source"] = value{scalar: source}
	r.values["dest"] = value{scalar: dest}
	if err := r.execute(items); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "file"))
	if err != nil || string(data) != "new" {
		t.Fatalf("published data: %q %v", data, err)
	}
}

func TestPublishPlanDoesNotRename(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "temporary")
	dest := filepath.Join(dir, "current")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	r := &runtime{values: map[string]value{}, filename: "test.yesh", plan: true}
	items, err := parse("publish $source to $dest\n", "test.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r.values["source"] = value{scalar: source}
	r.values["dest"] = value{scalar: dest}
	if err := r.execute(items); err != nil {
		t.Fatalf("publish plan: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("plan changed source: %v", err)
	}
}

func TestPublishRejectsExistingDestinationDirectory(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "temporary")
	dest := filepath.Join(dir, "current")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0700); err != nil {
		t.Fatal(err)
	}
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	items, err := parse("publish $source to $dest\n", "test.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r.values["source"] = value{scalar: source}
	r.values["dest"] = value{scalar: dest}
	if err := r.execute(items); err == nil {
		t.Fatal("publish unexpectedly merged into existing directory")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source disappeared after failed publish: %v", err)
	}
}

func TestPublishRejectsCrossFilesystemMove(t *testing.T) {
	if _, err := os.Stat("/dev/shm"); err != nil {
		t.Skip("/dev/shm is unavailable")
	}
	source := filepath.Join(t.TempDir(), "temporary")
	destDir, err := os.MkdirTemp("/dev/shm", "yesh-publish-")
	if err != nil {
		t.Skipf("cannot create tmpfs fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(destDir) })
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destDir, "current")
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	items, err := parse("publish $source to $dest\n", "test.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r.values["source"] = value{scalar: source}
	r.values["dest"] = value{scalar: dest}
	if err := r.execute(items); err == nil {
		t.Fatal("cross-filesystem publish unexpectedly succeeded")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source disappeared after cross-filesystem failure: %v", err)
	}
}
