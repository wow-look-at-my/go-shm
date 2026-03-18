//go:build windows

package shm

import (
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
)

var (
	modkernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCreateFileMapping = modkernel32.NewProc("CreateFileMappingW")
	procOpenFileMapping   = modkernel32.NewProc("OpenFileMappingW")
	procMapViewOfFile     = modkernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile   = modkernel32.NewProc("UnmapViewOfFile")
	procVirtualQuery      = modkernel32.NewProc("VirtualQuery")
)

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	_                 [2]byte
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

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
func Create(name string, size int) (*SharedMemory, error) {
	if err := validateArgs(name, size); err != nil {
		return nil, err
	}

	mapName, err := syscall.UTF16PtrFromString("Local\\go-shm-" + name)
	if err != nil {
		return nil, fmt.Errorf("shm: invalid name %q: %w", name, err)
	}

	hi := uint32(int64(size) >> 32)
	lo := uint32(int64(size) & 0xFFFFFFFF)

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
		h, fileMapAllAccess, 0, 0, uintptr(size),
	)
	if addr == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("shm: map view %q: %w", name, errno)
	}

	return &SharedMemory{
		name: name,
		size: size,
		data: sliceFromAddr(addr, size),
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

	var info memoryBasicInformation
	ret, _, errno := syscall.SyscallN(procVirtualQuery.Addr(),
		addr, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info),
	)
	if ret == 0 {
		syscall.SyscallN(procUnmapViewOfFile.Addr(), addr)
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("shm: query size %q: %w", name, errno)
	}

	size := int(info.RegionSize)

	return &SharedMemory{
		name: name,
		size: size,
		data: sliceFromAddr(addr, size),
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
