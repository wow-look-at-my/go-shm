package shm

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func TestCreateAndClose(t *testing.T) {
	seg, err := Create("test-create", 4096)
	require.Nil(t, err)

	defer seg.Unlink()

	assert.Equal(t, "test-create", seg.Name())
	assert.Equal(t, 4096, seg.Size())
	assert.Equal(t, 4096, len(seg.Data()))

	require.NoError(t, seg.Close())

	err = seg.Close()
	assert.Equal(t, ErrClosed, err)
}

func TestReadWrite(t *testing.T) {
	seg, err := Create("test-rw", 1024)
	require.Nil(t, err)

	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	message := []byte("hello shared memory")
	n, err := seg.Write(message, 0)
	require.Nil(t, err)
	assert.Equal(t, len(message), n)

	buf := make([]byte, len(message))
	n, err = seg.Read(buf, 0)
	require.Nil(t, err)
	assert.Equal(t, len(message), n)
	assert.Equal(t, message, buf)
}

func TestWriteAtOffset(t *testing.T) {
	seg, err := Create("test-offset", 256)
	require.Nil(t, err)

	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	_, err = seg.Write([]byte("AAAA"), 100)
	require.Nil(t, err)

	buf := make([]byte, 4)
	_, err = seg.Read(buf, 100)
	require.Nil(t, err)
	assert.Equal(t, "AAAA", string(buf))
}

func TestBoundsChecking(t *testing.T) {
	seg, err := Create("test-bounds", 64)
	require.Nil(t, err)

	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	_, err = seg.Read(make([]byte, 1), -1)
	assert.Equal(t, ErrOutOfBounds, err)

	_, err = seg.Read(make([]byte, 1), 64)
	assert.Equal(t, ErrOutOfBounds, err)

	_, err = seg.Write([]byte{1}, -1)
	assert.Equal(t, ErrOutOfBounds, err)

	_, err = seg.Write([]byte{1}, 64)
	assert.Equal(t, ErrOutOfBounds, err)

	// Write that extends beyond the end should be truncated.
	n, err := seg.Write(make([]byte, 100), 32)
	require.Nil(t, err)
	assert.Equal(t, 32, n)
}

func TestClosedOperations(t *testing.T) {
	seg, err := Create("test-closed", 64)
	require.Nil(t, err)

	defer seg.Unlink()
	seg.Close()

	_, err = seg.Read(make([]byte, 1), 0)
	assert.Equal(t, ErrClosed, err)

	_, err = seg.Write([]byte{1}, 0)
	assert.Equal(t, ErrClosed, err)
}

func TestValidation(t *testing.T) {
	_, err := Create("", 1024)
	assert.Equal(t, ErrInvalidName, err)

	_, err = Create("test", 0)
	assert.Equal(t, ErrInvalidSize, err)

	_, err = Create("test", -1)
	assert.Equal(t, ErrInvalidSize, err)

	_, err = Open("")
	assert.Equal(t, ErrInvalidName, err)
}

func TestOpenExisting(t *testing.T) {
	seg1, err := Create("test-open", 512)
	require.Nil(t, err)

	defer func() {
		seg1.Close()
		seg1.Unlink()
	}()

	_, err = seg1.Write([]byte("shared data"), 0)
	require.Nil(t, err)

	seg2, err := Open("test-open")
	require.Nil(t, err)

	defer seg2.Close()

	buf := make([]byte, 11)
	_, err = seg2.Read(buf, 0)
	require.Nil(t, err)
	assert.Equal(t, "shared data", string(buf))
	assert.Equal(t, 512, seg2.Size())
}

func TestDataSliceSharing(t *testing.T) {
	seg1, err := Create("test-sharing", 256)
	require.Nil(t, err)

	defer func() {
		seg1.Close()
		seg1.Unlink()
	}()

	seg2, err := Open("test-sharing")
	require.Nil(t, err)

	defer seg2.Close()

	// Write through seg1's Data slice.
	copy(seg1.Data(), []byte("direct write"))

	// Read through seg2's Data slice.
	got := string(seg2.Data()[:12])
	assert.Equal(t, "direct write", got)
}

func TestOpenNonExistent(t *testing.T) {
	_, err := Open("does-not-exist-shm-segment")
	assert.NotNil(t, err)
}

// TestCrossProcess verifies that shared memory works across OS processes.
// It spawns a child process that reads from a segment created by the parent.
func TestCrossProcess(t *testing.T) {
	if os.Getenv("SHM_TEST_CHILD") == "1" {
		crossProcessChild(t)
		return
	}

	seg, err := Create("test-xproc", 4096)
	require.Nil(t, err)

	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	secret := []byte("cross-process-works!")
	_, err = seg.Write(secret, 0)
	require.Nil(t, err)

	cmd := exec.Command(os.Args[0], "-test.run=^TestCrossProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "SHM_TEST_CHILD=1",
		fmt.Sprintf("SHM_TEST_NAME=%s", "test-xproc"),
		fmt.Sprintf("SHM_TEST_EXPECT=%s", secret),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child process failed: %v\noutput:\n%s", err, out)
	}
}

func crossProcessChild(t *testing.T) {
	name := os.Getenv("SHM_TEST_NAME")
	expect := os.Getenv("SHM_TEST_EXPECT")

	seg, err := Open(name)
	require.Nil(t, err)

	defer seg.Close()

	buf := make([]byte, len(expect))
	_, err = seg.Read(buf, 0)
	require.Nil(t, err)
	require.Equal(t, expect, string(buf))
}
