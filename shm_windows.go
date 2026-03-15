//go:build windows

package shm

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// platformHandle holds Windows-specific resources for a shared memory segment.
type platformHandle struct {
	mapHandle windows.Handle
}

const (
	fileMapAllAccess = 0xF001F
)

// Create allocates a new shared memory segment with the given name and size.
// On Windows, the segment is backed by the system page file using named file mappings.
func Create(name string, size int) (*SharedMemory, error) {
	if err := validateArgs(name, size); err != nil {
		return nil, err
	}

	mapName, err := windows.UTF16PtrFromString("Local\\go-shm-" + name)
	if err != nil {
		return nil, fmt.Errorf("shm: invalid name %q: %w", name, err)
	}

	hi := uint32(int64(size) >> 32)
	lo := uint32(int64(size) & 0xFFFFFFFF)

	h, err := windows.CreateFileMapping(
		windows.InvalidHandle,
		nil,
		windows.PAGE_READWRITE,
		hi, lo,
		mapName,
	)
	if err != nil {
		return nil, fmt.Errorf("shm: create %q: %w", name, err)
	}

	addr, err := windows.MapViewOfFile(h, fileMapAllAccess, 0, 0, uintptr(size))
	if err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("shm: map view %q: %w", name, err)
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)

	return &SharedMemory{
		name: name,
		size: size,
		data: data,
		handle: platformHandle{
			mapHandle: h,
		},
	}, nil
}

// Open attaches to an existing shared memory segment created by another process.
func Open(name string) (*SharedMemory, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	mapName, err := windows.UTF16PtrFromString("Local\\go-shm-" + name)
	if err != nil {
		return nil, fmt.Errorf("shm: invalid name %q: %w", name, err)
	}

	h, err := windows.OpenFileMapping(fileMapAllAccess, false, mapName)
	if err != nil {
		return nil, fmt.Errorf("shm: open %q: %w", name, err)
	}

	// Map the entire segment. We don't know the size yet, so map with size 0
	// which maps the whole object.
	addr, err := windows.MapViewOfFile(h, fileMapAllAccess, 0, 0, 0)
	if err != nil {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("shm: map view %q: %w", name, err)
	}

	// Query the size of the mapping.
	var info windows.MemoryBasicInformation
	if err := windows.VirtualQuery(addr, &info, unsafe.Sizeof(info)); err != nil {
		windows.UnmapViewOfFile(addr)
		windows.CloseHandle(h)
		return nil, fmt.Errorf("shm: query size %q: %w", name, err)
	}

	size := int(info.RegionSize)
	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)

	return &SharedMemory{
		name: name,
		size: size,
		data: data,
		handle: platformHandle{
			mapHandle: h,
		},
	}, nil
}

// Close unmaps the shared memory segment from this process.
func (s *SharedMemory) Close() error {
	if s.closed {
		return ErrClosed
	}
	s.closed = true

	var firstErr error
	addr := uintptr(unsafe.Pointer(&s.data[0]))
	if err := windows.UnmapViewOfFile(addr); err != nil {
		firstErr = fmt.Errorf("shm: unmap: %w", err)
	}
	s.data = nil

	if err := windows.CloseHandle(s.handle.mapHandle); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("shm: close handle: %w", err)
	}
	return firstErr
}

// Unlink is a no-op on Windows. Named file mappings are automatically removed
// when all handles to them are closed.
func (s *SharedMemory) Unlink() error {
	return nil
}
