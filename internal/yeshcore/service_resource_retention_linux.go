//go:build linux

package yeshcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func pruneServiceResourceHistory(r *runtime, stateDir, manager, unit string, keepFor time.Duration) (int, error) {
	journalPath := serviceResourceJournalPath(stateDir)
	if _, err := os.Stat(journalPath); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	err := r.withLock(filepath.Join(filepath.Dir(journalPath), ".lock"), func() error {
		states, err := readServiceResourceJournal(journalPath)
		if err != nil {
			return err
		}
		cutoff := time.Now().UTC().Add(-keepFor)
		kept := make([]serviceResourceState, 0, len(states))
		for _, state := range states {
			if state.Manager != manager || state.Unit != unit {
				return fmt.Errorf("resource state journal contains %s/%s, expected %s/%s", state.Manager, state.Unit, manager, unit)
			}
			if state.FinishedAt.Before(cutoff) {
				removed++
				continue
			}
			kept = append(kept, state)
		}
		if removed == 0 {
			return nil
		}
		file, err := os.CreateTemp(stateDir, ".measurements-*.tmp")
		if err != nil {
			return err
		}
		temporary := file.Name()
		defer os.Remove(temporary)
		if err := file.Chmod(0600); err != nil {
			file.Close()
			return err
		}
		for _, state := range kept {
			data, err := json.Marshal(state)
			if err != nil {
				file.Close()
				return err
			}
			record := append(data, '\n')
			if len(record) > maxServiceResourceRecord {
				file.Close()
				return fmt.Errorf("resource state record exceeds %d bytes", maxServiceResourceRecord)
			}
			if err := writeServiceResourceRecord(file, record); err != nil {
				file.Close()
				return err
			}
		}
		if err := file.Sync(); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Rename(temporary, journalPath); err != nil {
			return err
		}
		directory, err := os.Open(stateDir)
		if err != nil {
			return err
		}
		defer directory.Close()
		return directory.Sync()
	})
	if err != nil {
		return 0, fmt.Errorf("prune resource history: %w", err)
	}
	return removed, nil
}
