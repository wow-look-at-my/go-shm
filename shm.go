// Package shm provides cross-platform shared memory for interprocess communication.
//
// Shared memory segments are identified by name and can be accessed by multiple
// processes simultaneously.
//
// Supported platforms: Linux, macOS (Darwin), Windows.
package shm

import (
	"errors"
	"strings"
)

// Errors returned by shared memory operations.
var (
	ErrClosed      = errors.New("shm: segment is closed")
	ErrInvalidSize = errors.New("shm: size must be positive")
	ErrInvalidName = errors.New("shm: name must not be empty")
	ErrOutOfBounds = errors.New("shm: offset out of bounds")
)

// SharedMemory represents a shared memory segment that can be accessed
// by multiple processes.
type SharedMemory struct {
	name   string
	size   int
	data   []byte
	closed bool
	handle platformHandle
}

// Name returns the name of the shared memory segment.
func (s *SharedMemory) Name() string {
	return s.name
}

// Size returns the size of the shared memory segment in bytes.
func (s *SharedMemory) Size() int {
	return s.size
}

// Data returns a direct reference to the shared memory region as a byte slice.
// Modifications to this slice are visible to all processes attached to the segment.
// The slice must not be used after Close is called.
func (s *SharedMemory) Data() []byte {
	return s.data
}

// Read copies data from the shared memory segment starting at offset into buf.
// It returns the number of bytes copied and any error encountered.
func (s *SharedMemory) Read(buf []byte, offset int64) (int, error) {
	if s.closed {
		return 0, ErrClosed
	}
	if offset < 0 || int(offset) >= s.size {
		return 0, ErrOutOfBounds
	}
	n := copy(buf, s.data[offset:])
	return n, nil
}

// Write copies data into the shared memory segment starting at offset.
// It returns the number of bytes written and any error encountered.
func (s *SharedMemory) Write(data []byte, offset int64) (int, error) {
	if s.closed {
		return 0, ErrClosed
	}
	if offset < 0 || int(offset) >= s.size {
		return 0, ErrOutOfBounds
	}
	n := copy(s.data[offset:], data)
	return n, nil
}

func validateArgs(name string, size int) error {
	if err := validateName(name); err != nil {
		return err
	}
	if size <= 0 {
		return ErrInvalidSize
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	if strings.ContainsAny(name, "/\\") {
		return ErrInvalidName
	}
	return nil
}
