//go:build js || wasip1

package shm

import (
	"fmt"
	"os"
	"sync"
)

// WebAssembly runs one process with one linear memory, and has no mapping
// call. A segment is a heap buffer in a registry of this process, and every
// Open of its name gets the same bytes.

// platformHandle is empty, because the registry holds the segment.
type platformHandle struct{}

var (
	segmentsMu sync.Mutex
	segments   = make(map[string][]byte)
)

// Create allocates a new shared memory segment with the given name and size.
// A segment with the same name is replaced. Its open copies keep their bytes.
func Create(name string, size int) (*SharedMemory, error) {
	if err := validateArgs(name, size); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	segmentsMu.Lock()
	segments[name] = data
	segmentsMu.Unlock()
	return &SharedMemory{name: name, size: size, data: data}, nil
}

// Open attaches to an existing shared memory segment of this process.
func Open(name string) (*SharedMemory, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	segmentsMu.Lock()
	data, ok := segments[name]
	segmentsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("shm: open %q: %w", name, os.ErrNotExist)
	}
	return &SharedMemory{name: name, size: len(data), data: data}, nil
}

// Close detaches the segment. Other handles to it are not affected.
func (s *SharedMemory) Close() error {
	if s.closed {
		return ErrClosed
	}
	s.closed = true
	s.data = nil
	return nil
}

// Unlink removes the name of the segment. After unlinking, Open fails, but
// existing handles stay valid until closed.
func (s *SharedMemory) Unlink() error {
	segmentsMu.Lock()
	defer segmentsMu.Unlock()
	if _, ok := segments[s.name]; !ok {
		return fmt.Errorf("shm: unlink %q: %w", s.name, os.ErrNotExist)
	}
	delete(segments, s.name)
	return nil
}
