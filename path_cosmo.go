package shm

import (
	"os"
	"path/filepath"
)

// One binary picks its host at run time, so this asks the file system rather
// than the build: /dev/shm is the tmpfs every process on Linux reaches, and a
// host without one has a per-user temporary directory of the same scope.
func shmPath(name string) string {
	dir := os.TempDir()
	if info, err := os.Stat("/dev/shm"); err == nil && info.IsDir() {
		dir = "/dev/shm"
	}
	return filepath.Join(dir, "go-shm-"+name)
}
