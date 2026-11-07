package yeshcore

import (
	"path/filepath"
	"strings"
)

const serviceResourceJournalName = "measurements.jsonl"

func serviceResourceJournalPath(statePath string) string {
	if strings.HasSuffix(statePath, ".jsonl") {
		return statePath
	}
	return filepath.Join(statePath, serviceResourceJournalName)
}

func serviceResourceStateDirectory(statePath string) string {
	return filepath.Dir(serviceResourceJournalPath(statePath))
}
