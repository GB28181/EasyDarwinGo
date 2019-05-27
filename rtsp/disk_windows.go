package rtsp

import (
	"syscall"
	"unsafe"
)

type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
}

func DiskUsage(path string) (disk *DiskStatus, err error) {
	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	FreeBytesAvailable := uint64(0)
	TotalNumberOfBytes := uint64(0)
	TotalNumberOfFreeBytes := uint64(0)

	var r uintptr
	r, _, err = c.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&FreeBytesAvailable)),
		uintptr(unsafe.Pointer(&TotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&TotalNumberOfFreeBytes)),
	)

	if 0 == r {
		return nil, err
	}

	disk = &DiskStatus{
		All:  TotalNumberOfBytes,
		Used: TotalNumberOfBytes - FreeBytesAvailable,
		Free: FreeBytesAvailable,
	}

	return
}

// Storage constants
const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)
