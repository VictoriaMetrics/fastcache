// +build appengine windows

package fastcache

func getChunk() []byte {
	return make([]byte, chunkSize)
}

func putChunk(chunk []byte) {
	// No-op.
}

func clearChunks() error {
	// No-op.
	return nil
}
