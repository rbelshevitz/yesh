//go:build windows

package main

import (
	"fmt"
	"io"
)

type processInfo struct{}

func readProcessInfo(pid int) (processInfo, error) {
	return processInfo{}, fmt.Errorf("process snapshots are not supported on Windows")
}

func listProcessInfo(limit int) ([]processInfo, error) {
	return nil, fmt.Errorf("process snapshots are not supported on Windows")
}

func findProcessInfo(name string, limit int) ([]processInfo, error) {
	return nil, fmt.Errorf("process snapshots are not supported on Windows")
}

func writeProcessTable(out io.Writer, processes []processInfo, fields []string) error {
	return fmt.Errorf("process snapshots are not supported on Windows")
}

func writeProcessSummary(out io.Writer, processes []processInfo, metric string) error {
	return fmt.Errorf("process snapshots are not supported on Windows")
}

func processRSSTotal(processes []processInfo) (uint64, error) {
	return 0, fmt.Errorf("process snapshots are not supported on Windows")
}
