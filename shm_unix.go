//go:build unix

package shm

import (
	"fmt"
	"os"
	"syscall"

	"github.com/wow-look-at-my/go-mmap"
)

// platformHandle holds Unix-specific resources for a shared memory segment.
type platformHandle struct {
	fd   int
	path string
	mm   mmap.MMap
}

// Create allocates a new shared memory segment with the given name and size.
// The segment is backed by a memory-mapped file and accessible to other processes
// that Open the same name.
func Create(name string, size int) (*SharedMemory, error) {
	if err := validateArgs(name, size); err != nil {
		return nil, err
	}

	path := shmPath(name)

	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("shm: create %q: %w", name, err)
	}

	if err := syscall.Ftruncate(fd, int64(size)); err != nil {
		syscall.Close(fd)
		os.Remove(path)
		return nil, fmt.Errorf("shm: truncate %q: %w", name, err)
	}

	mm, err := mmap.MapRegion(fd, size, mmap.ProtRead|mmap.ProtWrite, mmap.MapShared, 0)
	if err != nil {
		syscall.Close(fd)
		os.Remove(path)
		return nil, fmt.Errorf("shm: mmap %q: %w", name, err)
	}

	return &SharedMemory{
		name: name,
		size: size,
		data: []byte(mm),
		handle: platformHandle{
			fd:   fd,
			path: path,
			mm:   mm,
		},
	}, nil
}

// Open attaches to an existing shared memory segment created by another process.
func Open(name string) (*SharedMemory, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	path := shmPath(name)

	fd, err := syscall.Open(path, syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("shm: open %q: %w", name, err)
	}

	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("shm: stat %q: %w", name, err)
	}

	size := int(stat.Size)
	if size == 0 {
		syscall.Close(fd)
		return nil, fmt.Errorf("shm: %q has zero size", name)
	}

	mm, err := mmap.MapRegion(fd, size, mmap.ProtRead|mmap.ProtWrite, mmap.MapShared, 0)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("shm: mmap %q: %w", name, err)
	}

	return &SharedMemory{
		name: name,
		size: size,
		data: []byte(mm),
		handle: platformHandle{
			fd:   fd,
			path: path,
			mm:   mm,
		},
	}, nil
}

// Close unmaps the shared memory segment from this process.
// Other processes with the segment open are not affected.
func (s *SharedMemory) Close() error {
	if s.closed {
		return ErrClosed
	}
	s.closed = true

	var firstErr error
	if err := s.handle.mm.Unmap(); err != nil {
		firstErr = fmt.Errorf("shm: munmap: %w", err)
	}
	s.data = nil

	if err := syscall.Close(s.handle.fd); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("shm: close fd: %w", err)
	}
	return firstErr
}

// Unlink removes the shared memory segment's backing file. After unlinking,
// no new processes can Open the segment, but existing mappings remain valid
// until closed.
func (s *SharedMemory) Unlink() error {
	return os.Remove(s.handle.path)
}
