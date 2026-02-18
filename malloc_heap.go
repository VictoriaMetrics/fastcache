//go:build appengine || windows || wasm || tinygo.wasm || js
// +build appengine windows wasm tinygo.wasm js

package fastcache

func getChunk() []byte {
	return make([]byte, chunkSize)
}

func putChunk(chunk []byte) {
	// No-op.
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
	return 0, 0
}
