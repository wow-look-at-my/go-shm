//go:build cosmo

package shm

import (
	"os"
	"path/filepath"
)

// One binary picks its host at run time. This asks the file system rather
// than the build: /dev/shm is the tmpfs every process on Linux reaches.
func shmPath(name string) string {
	dir := os.TempDir()
	if info, err := os.Stat("/dev/shm"); err == nil && info.IsDir() {
		dir = "/dev/shm"
	}
	return filepath.Join(dir, "go-shm-"+name)
}
