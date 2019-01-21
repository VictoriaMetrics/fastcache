package fastcache

import (
	"testing"
)

func BenchmarkSetBig(b *testing.B) {
	key := []byte("key12345")
	value := createValue(256*1024, 0)
	c := New(1024 * 1024)
	b.SetBytes(int64(len(value)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.SetBig(key, value)
		}
	})
}

func BenchmarkGetBig(b *testing.B) {
	key := []byte("key12345")
	value := createValue(265*1024, 0)
	c := New(1024 * 1024)
	c.SetBig(key, value)
	b.SetBytes(int64(len(value)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			buf = c.GetBig(buf[:0], key)
		}
	})
}
