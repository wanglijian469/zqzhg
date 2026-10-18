//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func diskSpace(path string) (uint64, uint64, error) {
	var info syscall.Statfs_t
	e := syscall.Statfs(path, &info)
	return info.Bavail * uint64(info.Bsize), info.Blocks * uint64(info.Bsize), e
}
func storageMountPresent(path string) bool {
	raw, e := os.ReadFile("/proc/self/mountinfo")
	if e != nil {
		return false
	}
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		parts := strings.Fields(line)
		if len(parts) > 4 {
			p := strings.ReplaceAll(parts[4], `\040`, " ")
			if resolved == p {
				if p != "/" {
					return true
				}
			}
		}
	}
	return false
}
