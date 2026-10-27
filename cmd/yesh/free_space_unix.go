//go:build !windows

package main

import (
	"fmt"
	"math"
	"syscall"
)

func freeBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	if stat.Bsize <= 0 {
		return 0, fmt.Errorf("filesystem has invalid block size")
	}
	blockSize := uint64(stat.Bsize)
	availableBlocks := uint64(stat.Bavail)
	if availableBlocks > math.MaxUint64/blockSize {
		return 0, fmt.Errorf("free space value overflows")
	}
	return availableBlocks * blockSize, nil
}
