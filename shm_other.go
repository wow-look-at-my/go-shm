//go:build !unix && !windows

package shm

import "errors"

// This platform has no shared memory. Every call fails, so a caller runs without it.
type platformHandle struct{}

// supported reports whether this platform has shared memory.
const supported = false

// Create fails: this platform has no shared memory.
func Create(name string, size int) (*SharedMemory, error) {
	if err := validateArgs(name, size); err != nil {
		return nil, err
	}
	return nil, errors.ErrUnsupported
}

// Open fails: this platform has no shared memory.
func Open(name string) (*SharedMemory, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	return nil, errors.ErrUnsupported
}

// Close marks the segment closed.
func (s *SharedMemory) Close() error {
	if s.closed {
		return ErrClosed
	}
	s.closed = true
	s.data = nil
	return nil
}

// Unlink fails: this platform has no shared memory.
func (s *SharedMemory) Unlink() error {
	return errors.ErrUnsupported
}
