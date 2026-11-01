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
	lockPath := filepath.Join(stateDir, ".lock")
	var path string
	err := r.withLock(lockPath, func() error {
		data, err := json.Marshal(state)
		if err != nil {
			return err
		}
		if len(data) > maxServiceResourceRecord {
			return fmt.Errorf("resource state record exceeds %d bytes", maxServiceResourceRecord)
		}
		file, err := os.OpenFile(filepath.Join(stateDir, serviceResourceJournalName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
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
		path = filepath.Join(stateDir, serviceResourceJournalName)
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
