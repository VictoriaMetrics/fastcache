package fastcache

import (
	"bytes"
	"strconv"
	"testing"
)

func TestGenerationOverflow(t *testing.T) {
	c := New(1) // each bucket has 64 *1024 bytes capacity

	// Initial generation is 1
	genVal(t, c, 1)

	// These two keys has to the same bucket (100), so we can push the
	// generations up much faster.  The keys and values are sized so that
	// every time we push them into the cache they will completely fill the
	// bucket
	key1 := []byte(strconv.Itoa(26))
	bigVal1 := make([]byte, (32*1024)-(len(key1)+4))
	for i := range bigVal1 {
		bigVal1[i] = 1
	}
	key2 := []byte(strconv.Itoa(8))
	bigVal2 := make([]byte, (32*1024)-(len(key2)+5))
	for i := range bigVal2 {
		bigVal2[i] = 2
	}

	// Do some initial Set/Get demonstrate that this works
	for i := 0; i < 10; i++ {
		c.Set(key1, bigVal1)
		c.Set(key2, bigVal2)
		getVal(t, c, key1, bigVal1)
		getVal(t, c, key2, bigVal2)
		genVal(t, c, uint64(1+i))
	}

	// This is a hack to simulate calling Set 2^24-3 times
	// Actually doing this takes ~24 seconds, making the test slow
	c.buckets[100].gen = (1 << 24) - 2

	// c.buckets[100].gen == 16,777,215
	// Set/Get still works

	c.Set(key1, bigVal1)
	c.Set(key2, bigVal2)

	getVal(t, c, key1, bigVal1)
	getVal(t, c, key2, bigVal2)

	genVal(t, c, (1<<24)-1)

	// After the next Set operations
	// c.buckets[100].gen == 16,777,216

	// This set creates an index where `idx | (b.gen << bucketSizeBits)` == 0
	// The value is in the cache but is unreadable by Get
	c.Set(key1, bigVal1)

	// The Set above overflowed the bucket's generation. This means that
	// key2 is still in the cache, but can't get read because key2 has a
	// _very large_ generation value and appears to be from the future
	getVal(t, c, key2, bigVal2)

	// This Set creates an index where `(b.gen << bucketSizeBits)>>bucketSizeBits)==0`
	// The value is in the cache but is unreadable by Get
	c.Set(key2, bigVal2)

	// Ensure generations are working as we expect
	// NB: Here we skip the 2^24 generation, because the bucket carefully
	// avoids `generation==0`
	genVal(t, c, (1<<24)+1)

	getVal(t, c, key1, bigVal1)
	getVal(t, c, key2, bigVal2)

	// Do it a few more times to show that this bucket is now unusable
	for i := 0; i < 10; i++ {
		c.Set(key1, bigVal1)
		c.Set(key2, bigVal2)
		getVal(t, c, key1, bigVal1)
		getVal(t, c, key2, bigVal2)
		genVal(t, c, uint64((1<<24)+2+i))
	}
}

func getVal(t *testing.T, c *Cache, key, expected []byte) {
	t.Helper()
	get := c.Get(nil, key)
	if !bytes.Equal(get, expected) {
		t.Errorf("Expected value (%v) was not returned from the cache, instead got %v", expected[:10], get)
	}
}

func genVal(t *testing.T, c *Cache, expected uint64) {
	t.Helper()
	actual := c.buckets[100].gen
	// Ensure generations are working as we expect
	if actual != expected {
		t.Fatalf("Expected generation to be %d found %d instead", expected, actual)
	}
}
