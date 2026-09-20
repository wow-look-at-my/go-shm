//go:build !cosmo


package shm

import "path/filepath"

func shmPath(name string) string {
	return filepath.Join("/dev/shm", "go-shm-"+name)
}
