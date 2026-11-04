//go:build navigator

package main

import (
	"fmt"
	"time"
)

type navigatorMeasurementHistory struct {
	path    string
	count   int
	first   time.Time
	last    time.Time
	manager string
	unit    string
	rows    []navigatorMeasurementRow
}

type navigatorMeasurementRow struct {
	finished        time.Time
	duration        time.Duration
	complete        bool
	reason          string
	invocationID    string
	cgroup          string
	memoryStart     uint64
	memoryEnd       uint64
	memoryDelta     int64
	memoryAvailable bool
	comparisonDelta int64
	comparisonOK    bool
	marker          string
}

func navigatorMeasurementHistoryText(history navigatorMeasurementHistory) string {
	return fmt.Sprintf("%d saved measurement(s), from %s to %s", history.count, history.first.UTC().Format("2006-01-02 15:04:05Z"), history.last.UTC().Format("2006-01-02 15:04:05Z"))
}
