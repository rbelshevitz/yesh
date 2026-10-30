//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

func (r *runtime) withLock(path string, body func() error) error {
	if path == "" {
		return fmt.Errorf("lock path is empty")
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return fmt.Errorf("open lock file")
	}
	defer file.Close()
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("already held: %w", err)
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	return body()
}
