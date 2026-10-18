# go-shm

Cross-platform shared memory library for Go, suitable for interprocess communication.

## Supported Platforms

- **Linux** — memory-mapped files in `/dev/shm`
- **macOS** — memory-mapped files in the system temp directory
- **Windows** — named file mappings backed by the system page file

No cgo required on any platform.

## Install

```
go get github.com/wow-look-at-my/go-shm
```

## Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/wow-look-at-my/go-shm"
)

func main() {
    // Process 1: create a shared memory segment
    seg, err := shm.Create("my-segment", 4096)
    if err != nil {
        log.Fatal(err)
    }
    defer seg.Close()
    defer seg.Unlink()

    seg.Write([]byte("hello from process 1"), 0)

    // Process 2 (in another program): open the same segment
    seg2, err := shm.Open("my-segment")
    if err != nil {
        log.Fatal(err)
    }
    defer seg2.Close()

    buf := make([]byte, 20)
    seg2.Read(buf, 0)
    fmt.Println(string(buf)) // "hello from process 1"
}
```

## API

- `shm.Create(name string, size int) (*SharedMemory, error)` — create a new segment
- `shm.Open(name string) (*SharedMemory, error)` — open an existing segment
- `(*SharedMemory).Read(buf []byte, offset int64) (int, error)` — read from segment
- `(*SharedMemory).Write(data []byte, offset int64) (int, error)` — write to segment
- `(*SharedMemory).Data() []byte` — direct access to the shared memory region
- `(*SharedMemory).Size() int` — size in bytes
- `(*SharedMemory).Close() error` — unmap from this process
- `(*SharedMemory).Unlink() error` — remove the segment (other mappings remain valid)
