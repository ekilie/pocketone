//go:build linux || freebsd

package main

import "syscall"

func maxRSS() int64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return int64(ru.Maxrss) * 1024 // KB on linux
}
