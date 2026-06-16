//go:build !appengine && !windows && !wasm && !tinygo.wasm && !js
// +build !appengine,!windows,!wasm,!tinygo.wasm,!js

package fastcache

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

const chunksPerAlloc = 1024

var (
	freeChunks     []*[chunkSize]byte
	freeChunksLock sync.Mutex

	// Statistics of memory allocated via off-heap, guarded by lock.
	allocatedSize uint64 // The total memory size allocated from the offheap
	freeChunkSize uint64 // The total memory size allocated but not used
)

func getChunk() []byte {
	freeChunksLock.Lock()
	if len(freeChunks) == 0 {
		// Allocate offheap memory, so GOGC won't take into account cache size.
		// This should reduce free memory waste.
		data, err := unix.Mmap(-1, 0, chunkSize*chunksPerAlloc, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
		if err != nil {
			panic(fmt.Errorf("cannot allocate %d bytes via mmap: %s", chunkSize*chunksPerAlloc, err))
		}
		for len(data) > 0 {
			p := (*[chunkSize]byte)(unsafe.Pointer(&data[0]))
			freeChunks = append(freeChunks, p)
			data = data[chunkSize:]
			allocatedSize += chunkSize
			freeChunkSize += chunkSize
		}
	}
	n := len(freeChunks) - 1
	p := freeChunks[n]
	freeChunks[n] = nil
	freeChunks = freeChunks[:n]
	freeChunkSize -= chunkSize
	freeChunksLock.Unlock()
	return p[:]
}

func putChunk(chunk []byte) {
	if chunk == nil {
		return
	}
	chunk = chunk[:chunkSize]
	p := (*[chunkSize]byte)(unsafe.Pointer(&chunk[0]))

	freeChunksLock.Lock()
	freeChunks = append(freeChunks, p)
	freeChunkSize += chunkSize
	freeChunksLock.Unlock()
}

// GetOffHeapMemoryStats returns the memory allocation statistics from off-heap
// memory. This function returns two numbers:
//
//   - The first number represents the total memory allocated from the off-heap,
//     including both used and free memory.
//
//   - The second number represents the memory size which is allocated from
//     the off-heap but currently not in use (free memory).
//
// Note that these statistics provide insights into the off-heap memory usage,
// which is memory managed directly by the application and not subject to golang
// garbage collection.
func GetOffHeapMemoryStats() (uint64, uint64) {
	freeChunksLock.Lock()
	defer freeChunksLock.Unlock()

	return allocatedSize, freeChunkSize
}
