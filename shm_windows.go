//go:build windows

package shm

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"syscall"
	"unsafe"
)

// platformHandle holds Windows-specific resources for a shared memory segment.
type platformHandle struct {
	mapHandle syscall.Handle
	addr      uintptr
}

const (
	fileMapAllAccess = 0xF001F
	// headerSize is the number of bytes reserved at the start of the mapping
	// to store the original user-requested size as a little-endian uint64.
	headerSize = 8
)

var (
	modkernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCreateFileMapping = modkernel32.NewProc("CreateFileMappingW")
	procOpenFileMapping   = modkernel32.NewProc("OpenFileMappingW")
	procMapViewOfFile     = modkernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile   = modkernel32.NewProc("UnmapViewOfFile")
)

// sliceFromAddr creates a byte slice backed by the memory at addr.
// This must only be called with addresses returned from Windows memory-mapping syscalls.
// We use reflect.SliceHeader to avoid a uintptr→unsafe.Pointer conversion
// that go vet flags as "possible misuse of unsafe.Pointer".
func sliceFromAddr(addr uintptr, size int) []byte {
	var b []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = addr
	hdr.Len = size
	hdr.Cap = size
	return b
}

// Create allocates a new shared memory segment with the given name and size.
// On Windows, the segment is backed by the system page file using named file mappings.
// An 8-byte header stores the original size so that Open can recover it exactly
// (VirtualQuery only returns page-rounded sizes).
func Create(name string, size int) (*SharedMemory, error) {
	if err := validateArgs(name, size); err != nil {
		return nil, err
	}

	mapName, err := syscall.UTF16PtrFromString("Local\\go-shm-" + name)
	if err != nil {
		return nil, fmt.Errorf("shm: invalid name %q: %w", name, err)
	}

	totalSize := int64(size + headerSize)
	hi := uint32(totalSize >> 32)
	lo := uint32(totalSize & 0xFFFFFFFF)

	h, _, errno := syscall.SyscallN(procCreateFileMapping.Addr(),
		uintptr(syscall.InvalidHandle),
		0,
		uintptr(syscall.PAGE_READWRITE),
		uintptr(hi),
		uintptr(lo),
		uintptr(unsafe.Pointer(mapName)),
	)
	if h == 0 {
		return nil, fmt.Errorf("shm: create %q: %w", name, errno)
	}

	addr, _, errno := syscall.SyscallN(procMapViewOfFile.Addr(),
		h, fileMapAllAccess, 0, 0, uintptr(totalSize),
	)
	if addr == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("shm: map view %q: %w", name, errno)
	}

	// Write the original size into the header.
	binary.LittleEndian.PutUint64(sliceFromAddr(addr, headerSize), uint64(size))

	return &SharedMemory{
		name: name,
		size: size,
		data: sliceFromAddr(addr+headerSize, size),
		handle: platformHandle{
			mapHandle: syscall.Handle(h),
			addr:      addr,
		},
	}, nil
}

// Open attaches to an existing shared memory segment created by another process.
func Open(name string) (*SharedMemory, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	mapName, err := syscall.UTF16PtrFromString("Local\\go-shm-" + name)
	if err != nil {
		return nil, fmt.Errorf("shm: invalid name %q: %w", name, err)
	}

	h, _, errno := syscall.SyscallN(procOpenFileMapping.Addr(),
		fileMapAllAccess, 0, uintptr(unsafe.Pointer(mapName)),
	)
	if h == 0 {
		return nil, fmt.Errorf("shm: open %q: %w", name, errno)
	}

	addr, _, errno := syscall.SyscallN(procMapViewOfFile.Addr(),
		h, fileMapAllAccess, 0, 0, 0,
	)
	if addr == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("shm: map view %q: %w", name, errno)
	}

	// Read the original size from the header.
	size := int(binary.LittleEndian.Uint64(sliceFromAddr(addr, headerSize)))

	return &SharedMemory{
		name: name,
		size: size,
		data: sliceFromAddr(addr+headerSize, size),
		handle: platformHandle{
			mapHandle: syscall.Handle(h),
			addr:      addr,
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
	ret, _, errno := syscall.SyscallN(procUnmapViewOfFile.Addr(), s.handle.addr)
	if ret == 0 {
		firstErr = fmt.Errorf("shm: unmap: %w", errno)
	}
	s.data = nil

	if err := syscall.CloseHandle(s.handle.mapHandle); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("shm: close handle: %w", err)
	}
	return firstErr
}

// Unlink is a no-op on Windows. Named file mappings are automatically removed
// when all handles to them are closed.
func (s *SharedMemory) Unlink() error {
	return nil
}
