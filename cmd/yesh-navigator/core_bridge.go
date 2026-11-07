//go:build navigator

package main

import "yesh/internal/yeshcore"

type serviceResourceState = yeshcore.ServiceResourceState
type serviceResourceStateSample = yeshcore.ServiceResourceStateSample

const serviceResourceStateSchema = yeshcore.ServiceResourceStateSchema

func defaultServiceResourceStateDir(unit string) (string, error) {
	return yeshcore.DefaultServiceResourceStateDir(unit)
}

func serviceResourceJournalPath(statePath string) string {
	return yeshcore.ServiceResourceJournalPath(statePath)
}

func readServiceResourceJournal(path string) ([]serviceResourceState, error) {
	return yeshcore.ReadServiceResourceJournal(path)
}
