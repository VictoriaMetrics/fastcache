package fastcache

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash"
)

const bucketsCount = 256

const chunkSize = 64 * 1024

// Stats represents cache stats.
//
// Use Cache.UpdateStats for obtaining fresh stats from the cache.
type Stats struct {
	// GetCalls is the number of Get calls.
	GetCalls uint64

	// SetCalls is the number of Set calls.
	SetCalls uint64

	// Misses is the number of cache misses.
	Misses uint64

	// Collisions is the number of cache collisions.
	Collisions uint64

	// EntriesCount is the current number of entries in the cache.
	EntriesCount uint64

	// BytesSize is the current size of the cache in bytes.
	BytesSize uint64
}

// Cache is a fast thread-safe inmemory cache optimized for big number
// of entries.
//
// It has much lower impact on GC comparing to a simple `map[string][]byte`.
//
// Use New for creating new cache instance.
// Concurrent goroutines may call any Cache methods on the same cache instance.
type Cache struct {
	buckets [bucketsCount]bucket
}

// New returns new cache with the given maxBytes capacity in bytes.
//
// maxBytes must be smaller than the available RAM size for the app,
// since the cache holds data in memory.
//
// If maxBytes is less than 16MB, then the minimum cache capacity is 16MB.
func New(maxBytes int) *Cache {
	if maxBytes <= 0 {
		panic(fmt.Errorf("maxBytes must be greater than 0; got %d", maxBytes))
	}
	var c Cache
	maxBucketBytes := uint64((maxBytes + bucketsCount - 1) / bucketsCount)
	for i := range c.buckets[:] {
		c.buckets[i].Init(maxBucketBytes)
	}
	return &c
}

// Set stores (k, v) in the cache.
//
// The stored entry may be evicted at any time either due to cache
// overflow or due to unlikely hash collision.
// Pass higher maxBytes value to New if the added items disappear
// frequently.
//
// (k, v) entries with summary size exceeding 64KB aren't stored in the cache.
//
// k and v contents may be modified after returning from Set.
func (c *Cache) Set(k, v []byte) {
	h := xxhash.Sum64(k)
	idx := h % bucketsCount
	c.buckets[idx].Set(k, v, h)
}

// Get appends value by the key k to dst and returns the result.
//
// Get allocates new byte slice for the returned value if dst is nil.
//
// k contents may be modified after returning from Get.
func (c *Cache) Get(dst, k []byte) []byte {
	h := xxhash.Sum64(k)
	idx := h % bucketsCount
	return c.buckets[idx].Get(dst, k, h)
}

// Del deletes value for the given k from the cache.
//
// k contents may be modified after returning from Del.
func (c *Cache) Del(k []byte) {
	h := xxhash.Sum64(k)
	idx := h % bucketsCount
	c.buckets[idx].Del(h)
}

// Reset removes all the items from the cache.
func (c *Cache) Reset() {
	for i := range c.buckets[:] {
		c.buckets[i].Reset()
	}
}

// UpdateStats adds cache stats to s.
func (c *Cache) UpdateStats(s *Stats) {
	for i := range c.buckets[:] {
		c.buckets[i].UpdateStats(s)
	}
}

type bucket struct {
	mu sync.RWMutex

	// chunks is a ring buffer with encoded (k, v) pairs.
	// It consists of 64KB chunks.
	chunks [][]byte

	// m maps hash(k) to idx of (k, v) pair in chunks.
	m map[uint64]uint64

	// idx points to chunks for writing the next (k, v) pair.
	idx uint64

	// gen is the generation of chunks.
	gen uint64

	getCalls   uint64
	setCalls   uint64
	misses     uint64
	collisions uint64
}

func (b *bucket) Init(maxBytes uint64) {
	if maxBytes >= (1 << 40) {
		panic(fmt.Errorf("too big maxBytes=%d; should be smaller than %d", maxBytes, 1<<40))
	}
	maxChunks := (maxBytes + chunkSize - 1) / chunkSize
	b.chunks = make([][]byte, maxChunks)
	b.m = make(map[uint64]uint64)
	b.Reset()
}

func (b *bucket) Reset() {
	b.mu.Lock()
	chunks := b.chunks
	for i := range chunks {
		chunks[i] = chunks[i][:0]
	}
	bm := b.m
	for k := range bm {
		delete(bm, k)
	}
	b.idx = 0
	b.gen = 1
	b.getCalls = 0
	b.setCalls = 0
	b.misses = 0
	b.mu.Unlock()
}

func (b *bucket) Clean() {
	b.mu.Lock()
	bGen := b.gen
	bIdx := b.idx
	bm := b.m
	for k, v := range bm {
		gen := v >> 40
		idx := v & ((1 << 40) - 1)
		if gen == bGen && idx < bIdx || gen+1 == bGen && idx >= bIdx {
			continue
		}
		delete(bm, k)
	}
	b.mu.Unlock()
}

func (b *bucket) UpdateStats(s *Stats) {
	s.GetCalls += atomic.LoadUint64(&b.getCalls)
	s.SetCalls += atomic.LoadUint64(&b.setCalls)
	s.Misses += atomic.LoadUint64(&b.misses)
	s.Collisions += atomic.LoadUint64(&b.collisions)

	b.mu.RLock()
	s.EntriesCount += uint64(len(b.m))
	for _, chunk := range b.chunks {
		s.BytesSize += uint64(cap(chunk))
	}
	b.mu.RUnlock()
}

func (b *bucket) Set(k, v []byte, h uint64) {
	setCalls := atomic.AddUint64(&b.setCalls, 1)
	if setCalls%(1<<14) == 0 {
		b.Clean()
	}

	if len(k) >= (1<<16) || len(v) >= (1<<16) {
		// Too big key or value - its length cannot be encoded
		// with 2 bytes (see below). Skip the entry.
		return
	}
	var kvLenBuf [4]byte
	kvLenBuf[0] = byte(uint16(len(k)) >> 8)
	kvLenBuf[1] = byte(len(k))
	kvLenBuf[2] = byte(uint16(len(v)) >> 8)
	kvLenBuf[3] = byte(len(v))
	kvLen := uint64(len(kvLenBuf) + len(k) + len(v))
	if kvLen >= chunkSize {
		// Do not store too big keys and values, since they do not
		// fit a chunk.
		return
	}

	b.mu.Lock()
	idx := b.idx
	idxNew := idx + kvLen
	chunkIdx := idx / chunkSize
	chunkIdxNew := idxNew / chunkSize
	if chunkIdxNew > chunkIdx {
		if chunkIdxNew >= uint64(len(b.chunks)) {
			idx = 0
			idxNew = kvLen
			chunkIdx = 0
			b.gen++
			if b.gen == 0 {
				b.gen = 1
			}
		} else {
			idx = chunkIdxNew * chunkSize
			idxNew = idx + kvLen
			chunkIdx = chunkIdxNew
		}
		b.chunks[chunkIdx] = b.chunks[chunkIdx][:0]
	}
	chunk := b.chunks[chunkIdx]
	if chunk == nil {
		chunk = make([]byte, 0, chunkSize)
	}
	chunk = append(chunk, kvLenBuf[:]...)
	chunk = append(chunk, k...)
	chunk = append(chunk, v...)
	b.chunks[chunkIdx] = chunk
	b.m[h] = idx | (b.gen << 40)
	b.idx = idxNew
	b.mu.Unlock()
}

func (b *bucket) Get(dst, k []byte, h uint64) []byte {
	atomic.AddUint64(&b.getCalls, 1)
	found := false
	b.mu.RLock()
	v := b.m[h]
	if v > 0 {
		gen := v >> 40
		idx := v & ((1 << 40) - 1)
		if gen == b.gen && idx < b.idx || gen+1 == b.gen && idx >= b.idx {
			chunkIdx := idx / chunkSize
			idx %= chunkSize
			chunk := b.chunks[chunkIdx]
			kvLenBuf := chunk[idx : idx+4]
			keyLen := (uint64(kvLenBuf[0]) << 8) | uint64(kvLenBuf[1])
			valLen := (uint64(kvLenBuf[2]) << 8) | uint64(kvLenBuf[3])
			idx += 4
			if string(k) == string(chunk[idx:idx+keyLen]) {
				idx += keyLen
				dst = append(dst, chunk[idx:idx+valLen]...)
				found = true
			} else {
				atomic.AddUint64(&b.collisions, 1)
			}
		}
	}
	b.mu.RUnlock()
	if !found {
		atomic.AddUint64(&b.misses, 1)
	}
	return dst
}

func (b *bucket) Del(h uint64) {
	b.mu.Lock()
	delete(b.m, h)
	b.mu.Unlock()
}
