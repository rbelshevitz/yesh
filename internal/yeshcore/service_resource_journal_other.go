//go:build !linux

package yeshcore

import "fmt"

func readServiceResourceJournal(path string) ([]serviceResourceState, error) {
	return nil, fmt.Errorf("service resource journal is supported only on Linux")
}

func writeServiceResourceState(r *runtime, stateDir string, state serviceResourceState) (string, error) {
	return "", fmt.Errorf("service resource journal is supported only on Linux")
}
