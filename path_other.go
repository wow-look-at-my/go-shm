//go:build !linux && !darwin && !windows && !cosmo

package shm

import (
	"os"
	"path/filepath"
)

func shmPath(name string) string {
	return filepath.Join(os.TempDir(), "go-shm-"+name)
}
