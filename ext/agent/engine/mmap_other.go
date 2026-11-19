//go:build !(darwin || linux || freebsd)

package engine

import (
	"io"
	"os"
)

// Fallback: read the whole file into memory.
func mmapFile(f *os.File, size int64) ([]byte, error) {
	b := make([]byte, size)
	_, err := io.ReadFull(f, b)
	return b, err
}

func munmap(b []byte) error { return nil }
