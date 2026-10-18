//go:build js || wasip1

package shm

import "testing"

// WebAssembly cannot start a process, and every segment lives in this one.
// The peer is therefore this process, and it opens the segment by name.
func runPeer(t *testing.T, name, expect string) {
	crossProcessChild(t, name, expect)
}
