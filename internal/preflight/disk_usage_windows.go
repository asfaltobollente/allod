//go:build windows

package preflight

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GetDiskUsage returns total and used bytes for the specified filesystem path on Windows.
func GetDiskUsage(path string) (totalBytes int64, usedBytes int64, err error) {
	if path == "" || path == "/mnt/allod-storage" || path == "/" {
		// Default to current volume / drive root on Windows
		cwd, _ := os.Getwd()
		path = filepath.VolumeName(cwd) + `\`
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes int64
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}

	r1, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		return 0, 0, callErr
	}
	return totalNumberOfBytes, totalNumberOfBytes - totalNumberOfFreeBytes, nil
}
