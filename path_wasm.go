//go:build js || wasip1

package shm

import (
	"os"
	"path/filepath"
)

// The registry in shm_wasm.go holds every segment. No file backs one, and this
// path names the file another platform would use.
func shmPath(name string) string {
	return filepath.Join(os.TempDir(), "go-shm-"+name)
}
