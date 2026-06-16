package fastcache

import (
	"sync/atomic"
	"time"

	xxhash "github.com/cespare/xxhash/v2"
)

// TTLStats contains stats for {G,S}etWithTTL methods.
type TTLStats struct {
	// GetWithTTLCalls is the number of GetWithTTL calls.
	GetWithTTLCalls uint64

	// SetWithTTLCalls is the number of SetWithTTL calls.
	SetWithTTLCalls uint64

	// MissesWithTTL is the number of GetWithTTL calls to missing items.
	MissesWithTTL uint64
}

func (ts *TTLStats) reset() {
	atomic.StoreUint64(&ts.GetWithTTLCalls, 0)
	atomic.StoreUint64(&ts.SetWithTTLCalls, 0)
	atomic.StoreUint64(&ts.MissesWithTTL, 0)
}

// SetWithTTL stores (k, v) in the cache with TTL.
//
// GetWithTTL must be used for reading the stored entry.
//
// The stored entry may be evicted at any time either due to cache
// overflow or due to unlikely hash collision.
// Pass higher maxBytes value to New if the added items disappear
// frequently.
//
// (k, v) entries with summary size exceeding 64KB aren't stored in the cache.
// SetBig can be used for storing entries exceeding 64KB, but we do not support
// TTLs for big entries currently.
//
// k and v contents may be modified after returning from SetWithTTL.
func (c *Cache) SetWithTTL(k, v []byte, ttl time.Duration) {
	atomic.AddUint64(&c.ttlStats.SetWithTTLCalls, 1)

	h := xxhash.Sum64(k)
	deadBySec := time.Now().Add(ttl).Unix()
	c.ttlmu.Lock()
	c.ttl[h] = deadBySec
	c.ttlmu.Unlock()
	c.setWithHash(k, v, h)
}

// GetWithTTL appends value by the key k to dst and returns the result.
//
// GetWithTTL allocates new byte slice for the returned value if dst is nil.
//
// GetWithTTL returns only values stored in c via SetWithTTL.
//
// k contents may be modified after returning from GetWithTTL.
func (c *Cache) GetWithTTL(dst, k []byte) []byte {
	atomic.AddUint64(&c.ttlStats.GetWithTTLCalls, 1)
	h := xxhash.Sum64(k)
	c.ttlmu.RLock()
	deadBySec, exists := c.ttl[h]
	c.ttlmu.RUnlock()
	if !exists || !ttlValid(deadBySec) {
		atomic.AddUint64(&c.ttlStats.MissesWithTTL, 1)
		return dst
	}
	return c.getWithHash(dst, k, h)
}

// HasWithTTL returns true if entry for the given key k exists in the cache and not dead yet.
func (c *Cache) HasWithTTL(k []byte) bool {
	h := xxhash.Sum64(k)
	c.ttlmu.RLock()
	deadBySec, exists := c.ttl[h]
	c.ttlmu.RUnlock()
	return exists && ttlValid(deadBySec)
}

func ttlValid(deadBySec int64) bool {
	return time.Until(time.Unix(deadBySec, 0)) > 0
}

func (c *Cache) runTTLGC() {
	c.ttlgc.Do(c.ttlGCRoutine)
}

func (c *Cache) ttlGCRoutine() {
	go func() {
		for {
			c.ttlmu.Lock()
			for k, deadBySec := range c.ttl {
				if !ttlValid(deadBySec) {
					delete(c.ttl, k)
				}
			}
			c.ttlmu.Unlock()

			time.Sleep(30 * time.Second)
		}
	}()
}
