package main

import (
	"os"
	"path/filepath"
	"reflect"
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
