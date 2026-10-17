//go:build !windows

package main

import "syscall"

func diskFreeBytes(path string) (uint64, error) {
	var info syscall.Statfs_t
	err := syscall.Statfs(path, &info)
	return info.Bavail * uint64(info.Bsize), err
}
