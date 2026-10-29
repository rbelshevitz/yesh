//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopedLockReleasesOnBodyError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.lock")
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	want := os.ErrPermission
	if err := r.withLock(path, func() error { return want }); err != want {
		t.Fatalf("body error: got %v want %v", err, want)
	}
	if err := r.withLock(path, func() error { return nil }); err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
}

func TestScopedLockRejectsHeldLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.lock")
	r := &runtime{values: map[string]value{}, filename: "test.yesh"}
	if err := r.withLock(path, func() error {
		err := r.withLock(path, func() error { return nil })
		if err == nil || !strings.Contains(err.Error(), "already held") {
			t.Fatalf("expected held-lock error, got %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("outer lock: %v", err)
	}
}
