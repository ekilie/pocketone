package main

import "syscall"

// maxRSS is this process's peak resident set size in bytes.
func maxRSS() int64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return ru.Maxrss // bytes on darwin
}
