//go:build darwin || linux || freebsd

package engine

import (
	"os"
	"syscall"
)

func mmapFile(f *os.File, size int64) ([]byte, error) {
	return syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
}

func munmap(b []byte) error { return syscall.Munmap(b) }
