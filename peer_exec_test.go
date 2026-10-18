//go:build !(js || wasip1)

package shm

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// runPeer runs crossProcessChild in a new process of this test binary.
func runPeer(t *testing.T, name, expect string) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrossProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "SHM_TEST_CHILD=1", "SHM_TEST_NAME="+name, "SHM_TEST_EXPECT="+expect)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
}
