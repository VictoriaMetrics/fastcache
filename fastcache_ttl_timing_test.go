package fastcache

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkCacheSetWithTTL(b *testing.B) {
	const items = 1 << 16
	const ttl = 3 * time.Second
	c := New(12 * items)
	defer c.Reset()
	b.ReportAllocs()
	b.SetBytes(items)
	b.RunParallel(func(pb *testing.PB) {
		k := []byte("\x00\x00\x00\x00")
		v := []byte("xyza")
		for pb.Next() {
			for i := 0; i < items; i++ {
				k[0]++
				if k[0] == 0 {
					k[1]++
				}
				c.SetWithTTL(k, v, ttl)
			}
		}
	})
}

func BenchmarkCacheGetWithTTL(b *testing.B) {
	const items = 1 << 16
	const ttl = 3 * time.Second

	c := New(12 * items)
	defer c.Reset()
	k := []byte("\x00\x00\x00\x00")
	v := []byte("xyza")
	for i := 0; i < items; i++ {
		k[0]++
		if k[0] == 0 {
			k[1]++
		}
		c.SetWithTTL(k, v, ttl)
	}

	b.ReportAllocs()
	b.SetBytes(items)
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		k := []byte("\x00\x00\x00\x00")
		for pb.Next() {
			for i := 0; i < items; i++ {
				k[0]++
				if k[0] == 0 {
					k[1]++
				}
				buf = c.GetWithTTL(buf[:0], k)
				if string(buf) != string(v) {
					panic(fmt.Errorf("BUG: invalid value obtained; got %q; want %q", buf, v))
				}
			}
		}
	})
}

func BenchmarkCacheHasWithTTL(b *testing.B) {
	const items = 1 << 16
	const ttl = 3 * time.Second
	c := New(12 * items)
	defer c.Reset()
	k := []byte("\x00\x00\x00\x00")
	for i := 0; i < items; i++ {
		k[0]++
		if k[0] == 0 {
			k[1]++
		}
		c.SetWithTTL(k, nil, ttl)
	}

	b.ReportAllocs()
	b.SetBytes(items)
	b.RunParallel(func(pb *testing.PB) {
		k := []byte("\x00\x00\x00\x00")
		for pb.Next() {
			for i := 0; i < items; i++ {
				k[0]++
				if k[0] == 0 {
					k[1]++
				}
				if !c.HasWithTTL(k) {
					panic(fmt.Errorf("BUG: missing value for key %q", k))
				}
			}
		}
	})
}
