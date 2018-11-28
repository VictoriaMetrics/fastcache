package fastcache

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

func BenchmarkSaveToFile(b *testing.B) {
	const filePath = "BencharkSaveToFile.fastcache"
	defer os.Remove(filePath)
	c := newBenchCache()

	b.ReportAllocs()
	b.ResetTimer()
	b.SetBytes(benchCacheSize)
	for i := 0; i < b.N; i++ {
		if err := c.SaveToFile(filePath); err != nil {
			b.Fatalf("unexpected error when saving to file: %s", err)
		}
	}
}

func BenchmarkLoadFromFile(b *testing.B) {
	const filePath = "BenchmarkLoadFromFile.fastcache"
	defer os.Remove(filePath)

	c := newBenchCache()
	if err := c.SaveToFile(filePath); err != nil {
		b.Fatalf("cannot save cache to file: %s", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.SetBytes(benchCacheSize)
	for i := 0; i < b.N; i++ {
		c, err := LoadFromFile(filePath)
		if err != nil {
			b.Fatalf("cannot load cache from file: %s", err)
		}
		var s Stats
		c.UpdateStats(&s)
		if s.EntriesCount == 0 {
			b.Fatalf("unexpected zero entries")
		}
	}
}

var (
	benchCache     *Cache
	benchCacheOnce sync.Once
)

func newBenchCache() *Cache {
	benchCacheOnce.Do(func() {
		c := New(benchCacheSize)
		itemsCount := benchCacheSize / 20
		for i := 0; i < itemsCount; i++ {
			k := []byte(fmt.Sprintf("key %d", i))
			v := []byte(fmt.Sprintf("value %d", i))
			c.Set(k, v)
		}
		benchCache = c
	})
	return benchCache
}

const benchCacheSize = bucketsCount * chunkSize
