package shm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestCreateAndClose(t *testing.T) {
	seg, err := Create("test-create", 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer seg.Unlink()

	if seg.Name() != "test-create" {
		t.Errorf("Name() = %q, want %q", seg.Name(), "test-create")
	}
	if seg.Size() != 4096 {
		t.Errorf("Size() = %d, want 4096", seg.Size())
	}
	if len(seg.Data()) != 4096 {
		t.Errorf("len(Data()) = %d, want 4096", len(seg.Data()))
	}

	if err := seg.Close(); err != nil {
		t.Fatal(err)
	}
	if err := seg.Close(); err != ErrClosed {
		t.Errorf("double Close() = %v, want ErrClosed", err)
	}
}

func TestReadWrite(t *testing.T) {
	seg, err := Create("test-rw", 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	message := []byte("hello shared memory")
	n, err := seg.Write(message, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(message) {
		t.Errorf("Write returned %d, want %d", n, len(message))
	}

	buf := make([]byte, len(message))
	n, err = seg.Read(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(message) {
		t.Errorf("Read returned %d, want %d", n, len(message))
	}
	if !bytes.Equal(buf, message) {
		t.Errorf("Read = %q, want %q", buf, message)
	}
}

func TestWriteAtOffset(t *testing.T) {
	seg, err := Create("test-offset", 256)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	_, err = seg.Write([]byte("AAAA"), 100)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 4)
	_, err = seg.Read(buf, 100)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "AAAA" {
		t.Errorf("Read at offset = %q, want %q", buf, "AAAA")
	}
}

func TestBoundsChecking(t *testing.T) {
	seg, err := Create("test-bounds", 64)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	if _, err := seg.Read(make([]byte, 1), -1); err != ErrOutOfBounds {
		t.Errorf("Read at -1 = %v, want ErrOutOfBounds", err)
	}
	if _, err := seg.Read(make([]byte, 1), 64); err != ErrOutOfBounds {
		t.Errorf("Read at 64 = %v, want ErrOutOfBounds", err)
	}
	if _, err := seg.Write([]byte{1}, -1); err != ErrOutOfBounds {
		t.Errorf("Write at -1 = %v, want ErrOutOfBounds", err)
	}
	if _, err := seg.Write([]byte{1}, 64); err != ErrOutOfBounds {
		t.Errorf("Write at 64 = %v, want ErrOutOfBounds", err)
	}

	// Write that extends beyond the end should be truncated.
	n, err := seg.Write(make([]byte, 100), 32)
	if err != nil {
		t.Fatal(err)
	}
	if n != 32 {
		t.Errorf("truncated Write = %d, want 32", n)
	}
}

func TestClosedOperations(t *testing.T) {
	seg, err := Create("test-closed", 64)
	if err != nil {
		t.Fatal(err)
	}
	defer seg.Unlink()
	seg.Close()

	if _, err := seg.Read(make([]byte, 1), 0); err != ErrClosed {
		t.Errorf("Read on closed = %v, want ErrClosed", err)
	}
	if _, err := seg.Write([]byte{1}, 0); err != ErrClosed {
		t.Errorf("Write on closed = %v, want ErrClosed", err)
	}
}

func TestValidation(t *testing.T) {
	if _, err := Create("", 1024); err != ErrInvalidName {
		t.Errorf("Create empty name = %v, want ErrInvalidName", err)
	}
	if _, err := Create("test", 0); err != ErrInvalidSize {
		t.Errorf("Create zero size = %v, want ErrInvalidSize", err)
	}
	if _, err := Create("test", -1); err != ErrInvalidSize {
		t.Errorf("Create negative size = %v, want ErrInvalidSize", err)
	}
	if _, err := Open(""); err != ErrInvalidName {
		t.Errorf("Open empty name = %v, want ErrInvalidName", err)
	}
}

func TestOpenExisting(t *testing.T) {
	seg1, err := Create("test-open", 512)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		seg1.Close()
		seg1.Unlink()
	}()

	_, err = seg1.Write([]byte("shared data"), 0)
	if err != nil {
		t.Fatal(err)
	}

	seg2, err := Open("test-open")
	if err != nil {
		t.Fatal(err)
	}
	defer seg2.Close()

	buf := make([]byte, 11)
	_, err = seg2.Read(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "shared data" {
		t.Errorf("Read from opened segment = %q, want %q", buf, "shared data")
	}

	if seg2.Size() != 512 {
		t.Errorf("opened Size() = %d, want 512", seg2.Size())
	}
}

func TestDataSliceSharing(t *testing.T) {
	seg1, err := Create("test-sharing", 256)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		seg1.Close()
		seg1.Unlink()
	}()

	seg2, err := Open("test-sharing")
	if err != nil {
		t.Fatal(err)
	}
	defer seg2.Close()

	// Write through seg1's Data slice.
	copy(seg1.Data(), []byte("direct write"))

	// Read through seg2's Data slice.
	got := string(seg2.Data()[:12])
	if got != "direct write" {
		t.Errorf("cross-segment Data() = %q, want %q", got, "direct write")
	}
}

func TestOpenNonExistent(t *testing.T) {
	_, err := Open("does-not-exist-shm-segment")
	if err == nil {
		t.Error("Open non-existent should return error")
	}
}

// TestCrossProcess verifies that shared memory works across OS processes.
// It spawns a child process that reads from a segment created by the parent.
func TestCrossProcess(t *testing.T) {
	if os.Getenv("SHM_TEST_CHILD") == "1" {
		crossProcessChild(t)
		return
	}

	seg, err := Create("test-xproc", 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		seg.Close()
		seg.Unlink()
	}()

	secret := []byte("cross-process-works!")
	if _, err := seg.Write(secret, 0); err != nil {
		t.Fatal(err)
	}

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
	if err != nil {
		t.Fatalf("child: Open(%q) failed: %v", name, err)
	}
	defer seg.Close()

	buf := make([]byte, len(expect))
	if _, err := seg.Read(buf, 0); err != nil {
		t.Fatalf("child: Read failed: %v", err)
	}

	if string(buf) != expect {
		t.Fatalf("child: got %q, want %q", buf, expect)
	}
}
