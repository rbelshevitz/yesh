package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	stdruntime "runtime"
	"testing"
)

func TestRetentionCandidatesOnlyOldCompletedDirectories(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"snapshot.0", "snapshot.1", "snapshot.2", "snapshot.4.tmp", "snapshot.latest"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "snapshot.3"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := retentionCandidates(root, 2); err == nil {
		t.Fatal("non-directory candidate was accepted")
	}
	if err := os.Remove(filepath.Join(root, "snapshot.3")); err != nil {
		t.Fatal(err)
	}

	got, err := retentionCandidates(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"snapshot.0"}
	var names []string
	for _, candidate := range got {
		names = append(names, candidate.Name)
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("candidates=%v want %v", names, want)
	}
}

func TestRetentionCandidatesRejectsSymlinkAndRootSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "snapshot.0"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("snapshot.0", filepath.Join(root, "snapshot.1")); err != nil {
		t.Skipf("symlink test requires symlink privileges: %v", err)
	}
	if _, err := retentionCandidates(root, 1); err == nil {
		t.Fatal("snapshot symlink was accepted")
	}

	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("root symlink test requires symlink privileges: %v", err)
	}
	if _, err := retentionCandidates(link, 1); err == nil {
		t.Fatal("root symlink was accepted")
	}
}

func TestRetentionCandidatesProtectsNamedSnapshots(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"snapshot.0", "snapshot.1", "snapshot.2"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	got, err := retentionCandidates(root, 1, "snapshot.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "snapshot.1" {
		t.Fatalf("protected candidates=%v", got)
	}
}

func TestRetentionCandidatesRejectsInvalidKeepAndRoot(t *testing.T) {
	root := t.TempDir()
	for _, keep := range []int{0, -1, 10001} {
		if _, err := retentionCandidates(root, keep); err == nil {
			t.Fatalf("keep=%d was accepted", keep)
		}
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := retentionCandidates(file, 1); err == nil {
		t.Fatal("file root was accepted")
	}
	if _, err := retentionCandidates("", 1); err == nil {
		t.Fatal("empty root was accepted")
	}
}

func TestRetentionQuarantinePlanAndApply(t *testing.T) {
	if stdruntime.GOOS != "linux" {
		t.Skip("retention quarantine execution uses Linux advisory locks")
	}
	root := t.TempDir()
	quarantine := filepath.Join(root, "quarantine")
	if err := os.Mkdir(quarantine, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"snapshot.0", "snapshot.1", "snapshot.2"} {
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "marker"), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	lock := filepath.Join(root, ".lock")
	source := fmt.Sprintf("root = %q\nquarantine = %q\nlock = %q\nwith lock $lock:\n    rsync retention quarantine in $root keep 2 to $quarantine\n", root, quarantine, lock)
	items, err := parse(source, "quarantine.yesh")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&runtime{values: map[string]value{}, plan: true, filename: "quarantine.yesh"}).execute(items); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "snapshot.0")); err != nil {
		t.Fatalf("plan changed source: %v", err)
	}
	r := &runtime{values: map[string]value{}, filename: "quarantine.yesh"}
	if err := r.execute(items); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "snapshot.0")); !os.IsNotExist(err) {
		t.Fatalf("snapshot.0 remains after quarantine: %v", err)
	}
	if _, err := os.Stat(filepath.Join(quarantine, "snapshot.0", "marker")); err != nil {
		t.Fatalf("quarantine marker: %v", err)
	}
	for _, name := range []string{"snapshot.1", "snapshot.2"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("protected snapshot %s: %v", name, err)
		}
	}
}

func TestRetentionQuarantineRejectsUnsafeDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "snapshot.0"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := retentionQuarantineCandidates(root, filepath.Join(root, "missing"), 1); err == nil {
		t.Fatal("missing quarantine was accepted")
	}
	other := t.TempDir()
	if _, err := retentionQuarantineCandidates(root, other, 1); err == nil {
		t.Fatal("non-child quarantine was accepted")
	}
}

func retentionQuarantineSource(root, quarantine, lock string) string {
	return fmt.Sprintf("root = %q\nquarantine = %q\nlock = %q\nwith lock $lock:\n    rsync retention quarantine in $root keep 2 to $quarantine\n", root, quarantine, lock)
}

func makeRetentionSnapshots(t *testing.T, root, quarantine string) {
	t.Helper()
	if err := os.MkdirAll(quarantine, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"snapshot.0", "snapshot.1", "snapshot.2"} {
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "marker"), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func executeRetentionQuarantine(t *testing.T, root, quarantine, lock string) error {
	t.Helper()
	items, err := parse(retentionQuarantineSource(root, quarantine, lock), "quarantine-matrix.yesh")
	if err != nil {
		t.Fatal(err)
	}
	return (&runtime{values: map[string]value{}, filename: "quarantine-matrix.yesh"}).execute(items)
}

func TestRetentionQuarantineRuntimeFailureMatrix(t *testing.T) {
	if stdruntime.GOOS != "linux" {
		t.Skip("retention quarantine execution uses Linux advisory locks")
	}
	t.Run("destination collision is preflight failure", func(t *testing.T) {
		root := t.TempDir()
		quarantine := filepath.Join(root, "quarantine")
		lock := filepath.Join(root, ".lock")
		makeRetentionSnapshots(t, root, quarantine)
		collision := filepath.Join(quarantine, "snapshot.0")
		if err := os.Mkdir(collision, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(collision, "marker"), []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := executeRetentionQuarantine(t, root, quarantine, lock); err == nil {
			t.Fatal("destination collision was accepted")
		}
		if _, err := os.Stat(filepath.Join(root, "snapshot.0", "marker")); err != nil {
			t.Fatalf("source changed after collision: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(collision, "marker"))
		if err != nil || string(got) != "existing" {
			t.Fatalf("collision destination changed: %q %v", got, err)
		}
	})

	t.Run("symlink candidate is rejected before move", func(t *testing.T) {
		root := t.TempDir()
		quarantine := filepath.Join(root, "quarantine")
		lock := filepath.Join(root, ".lock")
		makeRetentionSnapshots(t, root, quarantine)
		if err := os.RemoveAll(filepath.Join(root, "snapshot.2")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("snapshot.1", filepath.Join(root, "snapshot.2")); err != nil {
			t.Skipf("symlink test requires symlink privileges: %v", err)
		}
		if err := executeRetentionQuarantine(t, root, quarantine, lock); err == nil {
			t.Fatal("snapshot symlink was accepted")
		}
		if _, err := os.Stat(filepath.Join(root, "snapshot.0")); err != nil {
			t.Fatalf("source changed after symlink rejection: %v", err)
		}
	})

	t.Run("invalid names are ignored and rerun is safe", func(t *testing.T) {
		root := t.TempDir()
		quarantine := filepath.Join(root, "quarantine")
		lock := filepath.Join(root, ".lock")
		makeRetentionSnapshots(t, root, quarantine)
		if err := os.Mkdir(filepath.Join(root, "not-a-snapshot"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("snapshot.2", filepath.Join(root, "snapshot.bad")); err != nil {
			t.Skipf("symlink test requires symlink privileges: %v", err)
		}
		if err := executeRetentionQuarantine(t, root, quarantine, lock); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(quarantine, "snapshot.0", "marker")); err != nil {
			t.Fatalf("quarantine result: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "not-a-snapshot")); err != nil {
			t.Fatalf("invalid name was changed: %v", err)
		}
		if err := executeRetentionQuarantine(t, root, quarantine, lock); err != nil {
			t.Fatalf("safe rerun: %v", err)
		}
	})

	t.Run("held lock blocks operation", func(t *testing.T) {
		root := t.TempDir()
		quarantine := filepath.Join(root, "quarantine")
		lock := filepath.Join(root, ".lock")
		makeRetentionSnapshots(t, root, quarantine)
		r := &runtime{values: map[string]value{}, filename: "quarantine-matrix.yesh"}
		if err := r.withLock(lock, func() error {
			if err := executeRetentionQuarantine(t, root, quarantine, lock); err == nil {
				t.Fatal("held lock was ignored")
			}
			if _, err := os.Stat(filepath.Join(root, "snapshot.0")); err != nil {
				t.Fatalf("source changed under lock conflict: %v", err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})
}
