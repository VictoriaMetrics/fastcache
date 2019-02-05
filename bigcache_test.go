package fastcache

import (
	"bytes"
	"fmt"
	"testing"
)

func TestSetGetBig(t *testing.T) {
	c := New(256 * 1024 * 1024)
	const valuesCount = 10
	for _, valueSize := range []int{1, 100, 1<<16 - 1, 1 << 16, 1<<16 + 1, 1 << 17, 1<<17 + 1, 1<<17 - 1, 1 << 19} {
		t.Run(fmt.Sprintf("valueSize_%d", valueSize), func(t *testing.T) {
			for seed := 0; seed < 3; seed++ {
				testSetGetBig(t, c, valueSize, valuesCount, seed)
			}
		})
	}
}

func testSetGetBig(t *testing.T, c *Cache, valueSize, valuesCount, seed int) {
	m := make(map[string][]byte)
	var buf []byte
	for i := 0; i < valuesCount; i++ {
		key := []byte(fmt.Sprintf("key %d", i))
		value := createValue(valueSize, seed)
		c.SetBig(key, value)
		m[string(key)] = value
		buf = c.GetBig(buf[:0], key)
		if !bytes.Equal(buf, value) {
			t.Fatalf("seed=%d; unexpected value obtained for key=%q; got len(value)=%d; want len(value)=%d", seed, key, len(buf), len(value))
		}
	}
	var s Stats
	c.UpdateStats(&s)
	if s.SetBigCalls < uint64(valuesCount) {
		t.Fatalf("expecting SetBigCalls >= %d; got %d", valuesCount, s.SetBigCalls)
	}
	if s.GetBigCalls < uint64(valuesCount) {
		t.Fatalf("expecting GetBigCalls >= %d; got %d", valuesCount, s.GetBigCalls)
	}

	// Verify that values stil exist
	for key, value := range m {
		buf = c.GetBig(buf[:0], []byte(key))
		if !bytes.Equal(buf, value) {
			t.Fatalf("seed=%d; unexpected value obtained for key=%q; got len(value)=%d; want len(value)=%d", seed, key, len(buf), len(value))
		}
	}
}

func createValue(size, seed int) []byte {
	var buf []byte
	for i := 0; i < size; i++ {
		buf = append(buf, byte(i+seed))
	}
	return buf
}
