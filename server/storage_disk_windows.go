//go:build windows

package main

import "golang.org/x/sys/windows"

func diskSpace(path string) (uint64, uint64, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return 0, 0, e
	}
	var free, total, available uint64
	e = windows.GetDiskFreeSpaceEx(p, &available, &total, &free)
	return available, total, e
}
func storageMountPresent(path string) bool { return false }
