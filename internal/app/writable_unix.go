//go:build !windows

package app

import "syscall"

func checkDirectoryWritable(path string) error { return syscall.Access(path, 2) }
