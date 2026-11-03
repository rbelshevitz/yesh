//go:build linux

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func navigatorFindMeasurementHistory(manager, unit string) (navigatorMeasurementHistory, error) {
	statePath, err := defaultServiceResourceStateDir(unit)
	if err != nil {
		return navigatorMeasurementHistory{}, err
	}
	path := serviceResourceJournalPath(statePath)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return navigatorMeasurementHistory{}, nil
	}
	if err != nil {
		return navigatorMeasurementHistory{}, fmt.Errorf("stat measurement history %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return navigatorMeasurementHistory{}, fmt.Errorf("measurement history %q is not a regular file", path)
	}
	if info.Size() == 0 {
		return navigatorMeasurementHistory{}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return navigatorMeasurementHistory{}, fmt.Errorf("open measurement history %q: %w", path, err)
	}
	last := make([]byte, 1)
	_, readErr := file.ReadAt(last, info.Size()-1)
	closeErr := file.Close()
	if readErr != nil {
		return navigatorMeasurementHistory{}, fmt.Errorf("read measurement history %q: %w", path, readErr)
	}
	if closeErr != nil {
		return navigatorMeasurementHistory{}, fmt.Errorf("close measurement history %q: %w", path, closeErr)
	}
	if last[0] != '\n' {
		return navigatorMeasurementHistory{}, fmt.Errorf("measurement history %q ends with an incomplete record", path)
	}
	states, err := readServiceResourceJournal(path)
	if err != nil {
		return navigatorMeasurementHistory{}, err
	}
	history := navigatorMeasurementHistory{path: path, manager: manager, unit: unit}
	for _, state := range states {
		if state.Manager != manager || state.Unit != unit {
			return navigatorMeasurementHistory{}, fmt.Errorf("measurement history contains %s/%s, expected %s/%s", state.Manager, state.Unit, manager, unit)
		}
		history.count++
		if history.first.IsZero() || state.StartedAt.Before(history.first) {
			history.first = state.StartedAt
		}
		if history.last.IsZero() || state.FinishedAt.After(history.last) {
			history.last = state.FinishedAt
		}
		history.rows = append(history.rows, navigatorMeasurementRowFromState(state, history.rows))
	}
	if history.count == 0 {
		return navigatorMeasurementHistory{}, nil
	}
	return history, nil
}

func navigatorMeasurementRowFromState(state serviceResourceState, previous []navigatorMeasurementRow) navigatorMeasurementRow {
	row := navigatorMeasurementRow{
		finished: state.FinishedAt, duration: state.FinishedAt.Sub(state.StartedAt),
		complete: state.Complete, reason: strings.Join(state.IncompleteReasons, ","),
		invocationID: state.InvocationID, cgroup: state.Cgroup, marker: "?",
	}
	if len(state.Samples) > 0 {
		start, startOK := navigatorMemoryValue(state.Samples[0])
		last, lastOK := navigatorMemoryValue(state.Samples[len(state.Samples)-1])
		row.memoryStart, row.memoryEnd, row.memoryAvailable = start, last, startOK && lastOK
		if row.memoryAvailable {
			row.memoryDelta = int64(last) - int64(start)
		}
	}
	if len(previous) > 0 {
		prior := previous[len(previous)-1]
		if prior.invocationID == row.invocationID && prior.cgroup == row.cgroup && prior.memoryAvailable && row.memoryAvailable {
			row.comparisonDelta = int64(row.memoryEnd) - int64(prior.memoryEnd)
			row.comparisonOK = true
			switch {
			case row.comparisonDelta > 0:
				row.marker = "↑"
			case row.comparisonDelta < 0:
				row.marker = "↓"
			default:
				row.marker = "="
			}
		}
	}
	return row
}

func navigatorMemoryValue(sample serviceResourceStateSample) (uint64, bool) {
	value, ok := sample.Metrics["memory.current"]
	if !ok || value == "unavailable" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}
