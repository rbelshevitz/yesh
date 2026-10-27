//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

// Open nonblocking so a FIFO or device cannot stall before the job deadline.
// Validate the opened inode before truncation, rather than relying on Lstat.
func openRedirect(name string, output, appendMode bool) (*os.File, error) {
	flags := syscall.O_RDONLY | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	if output {
		flags = syscall.O_WRONLY | syscall.O_CREAT | syscall.O_NONBLOCK | syscall.O_CLOEXEC | syscall.O_NOFOLLOW
		if appendMode {
			flags |= syscall.O_APPEND
		}
	}
	fd, err := syscall.Open(name, flags, 0644)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	reject := func(err error) (*os.File, error) { f.Close(); return nil, err }
	info, err := f.Stat()
	if err != nil {
		return reject(err)
	}
	isNull := false
	if info.Mode()&os.ModeCharDevice != 0 {
		null, err := os.Stat(os.DevNull)
		isNull = err == nil && os.SameFile(info, null)
	}
	if !info.Mode().IsRegular() && !isNull {
		return reject(fmt.Errorf("redirection requires a regular file or /dev/null"))
	}
	if output && info.Mode().IsRegular() {
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
			return reject(fmt.Errorf("output redirection to multiple hard links is disabled"))
		}
	}
	return f, nil
}
