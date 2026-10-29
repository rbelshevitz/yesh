//go:build !linux

package main

import "fmt"

func (r *runtime) queryJournal(manager, unit, sinceText string, lines int) ([]byte, error) {
	return nil, fmt.Errorf("journal is supported only on Linux")
}
