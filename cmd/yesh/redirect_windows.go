//go:build windows

package main

import "os"

func openRedirect(name string, output, appendMode bool) (*os.File, error) {
	if !output {
		return os.Open(name)
	}
	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	}
	return os.OpenFile(name, flags, 0644)
}
