//go:build !linux

package main

import "fmt"

func (r *runtime) withLock(path string, body func() error) error {
	return fmt.Errorf("with lock is supported only on Linux")
}
