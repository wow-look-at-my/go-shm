package shm

import (
	"os"
	"path/filepath"
)

func shmPath(name string) string {
	dir := os.TempDir()
	return filepath.Join(dir, "go-shm-"+name)
}
