//go:build windows

package main

import "fmt"

func freeBytes(path string) (uint64, error) {
	return 0, fmt.Errorf("free_space is not supported on Windows")
}
