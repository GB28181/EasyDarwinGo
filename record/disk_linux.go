package record

import (
	"syscall"
)

type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
}

// disk usage of path/disk
func DiskUsage(path string) (disk *DiskStatus, err error) {
	fs := syscall.Statfs_t{}
	err = syscall.Statfs(path, &fs)
	if err != nil {
		return
	}
	disk = &DiskStatus{
		All:  fs.Blocks * uint64(fs.Bsize),
		Free: fs.Bfree * uint64(fs.Bsize),
		Used: disk.All - disk.Free,
	}
	return
}

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)
