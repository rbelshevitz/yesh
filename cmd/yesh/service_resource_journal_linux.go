//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	serviceResourceJournalName = "measurements.jsonl"
	maxServiceResourceRecord   = 8 << 20
)

func writeServiceResourceState(r *runtime, stateDir string, state serviceResourceState) (string, error) {
	journalPath := serviceResourceJournalPath(stateDir)
	lockPath := filepath.Join(filepath.Dir(journalPath), ".lock")
	var path string
	err := r.withLock(lockPath, func() error {
		if err := validateExistingServiceResourceJournal(journalPath, state.Manager, state.Unit); err != nil {
			return err
		}
		data, err := json.Marshal(state)
		if err != nil {
			return err
		}
		if len(data) > maxServiceResourceRecord {
			return fmt.Errorf("resource state record exceeds %d bytes", maxServiceResourceRecord)
		}
		file, err := os.OpenFile(journalPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		if err := file.Chmod(0600); err != nil {
			file.Close()
			return err
		}
		record := make([]byte, len(data)+1)
		copy(record, data)
		record[len(data)] = '\n'
		if err := writeServiceResourceRecord(file, record); err != nil {
			file.Close()
			return err
		}
		if err := file.Sync(); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		path = journalPath
		directory, err := os.Open(stateDir)
		if err != nil {
			return err
		}
		defer directory.Close()
		return directory.Sync()
	})
	if err != nil {
		return "", fmt.Errorf("write resource state: %w", err)
	}
	return path, nil
}

func validateExistingServiceResourceJournal(path, manager, unit string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat resource state journal %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("resource state journal %q is not a regular file", path)
	}
	if info.Size() > 0 {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open resource state journal %q: %w", path, err)
		}
		last := make([]byte, 1)
		_, readErr := file.ReadAt(last, info.Size()-1)
		closeErr := file.Close()
		if readErr != nil {
			return fmt.Errorf("read resource state journal %q: %w", path, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close resource state journal %q: %w", path, closeErr)
		}
		if last[0] != '\n' {
			return fmt.Errorf("resource state journal %q ends with an incomplete record; repair it explicitly before measuring", path)
		}
	}
	states, err := readServiceResourceJournal(path)
	if err != nil {
		return err
	}
	for _, existing := range states {
		if existing.Manager != manager || existing.Unit != unit {
			return fmt.Errorf("resource state journal contains %s/%s, expected %s/%s", existing.Manager, existing.Unit, manager, unit)
		}
	}
	return nil
}

func writeServiceResourceRecord(file *os.File, record []byte) error {
	for len(record) > 0 {
		written, err := file.Write(record)
		if written > 0 {
			record = record[written:]
		}
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
