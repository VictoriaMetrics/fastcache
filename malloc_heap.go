//go:build appengine || windows || wasm || tinygo.wasm || js
// +build appengine windows wasm tinygo.wasm js

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
