package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

var completedSnapshotName = regexp.MustCompile(`^snapshot\.(0|[1-9][0-9]*)$`)

type retentionCandidate struct {
	Name     string
	Path     string
	Sequence uint64
}

func retentionSnapshotEntries(root string) ([]retentionCandidate, error) {
	if root == "" {
		return nil, fmt.Errorf("retention root is empty")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("retention root must be a real directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var snapshots []retentionCandidate
	for _, entry := range entries {
		match := completedSnapshotName.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("snapshot candidate %q is a symlink", entry.Name())
		}
		if !entry.IsDir() {
			return nil, fmt.Errorf("snapshot candidate %q is not a directory", entry.Name())
		}
		sequence, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("snapshot candidate %q has invalid sequence: %w", entry.Name(), err)
		}
		snapshots = append(snapshots, retentionCandidate{Name: entry.Name(), Path: filepath.Join(root, entry.Name()), Sequence: sequence})
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Sequence > snapshots[j].Sequence })
	return snapshots, nil
}

// retentionCandidates returns only completed snapshot directories that are
// older than the newest keep entries. It never mutates the filesystem.
func retentionCandidates(root string, keep int, protected ...string) ([]retentionCandidate, error) {
	if keep < 1 || keep > 10000 {
		return nil, fmt.Errorf("retention keep must be between 1 and 10000")
	}
	protectedSet := make(map[string]bool, len(protected))
	for _, name := range protected {
		if name != "" {
			protectedSet[name] = true
		}
	}
	snapshots, err := retentionSnapshotEntries(root)
	if err != nil {
		return nil, err
	}
	filtered := snapshots[:0]
	for _, snapshot := range snapshots {
		if !protectedSet[snapshot.Name] {
			filtered = append(filtered, snapshot)
		}
	}
	snapshots = filtered
	if len(snapshots) <= keep {
		return nil, nil
	}
	return snapshots[keep:], nil
}

func retentionQuarantineCandidates(root, quarantine string, keep int) ([]retentionCandidate, error) {
	if quarantine == "" {
		return nil, fmt.Errorf("quarantine directory is empty")
	}
	root = filepath.Clean(root)
	quarantine = filepath.Clean(quarantine)
	if filepath.Dir(quarantine) != root {
		return nil, fmt.Errorf("quarantine must be a direct child of retention root")
	}
	info, err := os.Lstat(quarantine)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("quarantine must be a real directory")
	}
	return retentionCandidates(root, keep)
}
