//go:build appengine || windows || wasm || tinygo.wasm
// +build appengine windows wasm tinygo.wasm

package fastcache

func getChunk() []byte {
	return make([]byte, chunkSize)
}

func putChunk(chunk []byte) {
	// No-op.
}
