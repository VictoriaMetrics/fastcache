package fastcache

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestTTLCacheSmall(t *testing.T) {
	c := New(1)
	defer c.Reset()

	v := c.GetWithTTL(nil, []byte("aaa"))
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from small cache: %q", v)
	}

	c.SetWithTTL([]byte("key"), []byte("value"), 3*time.Second)
	v = c.GetWithTTL(nil, []byte("key"))
	if string(v) != "value" {
		t.Fatalf("unexpected value obtained; got %q; want %q", v, "value")
	}

	v = c.GetWithTTL(nil, nil)
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from small cache: %q", v)
	}
	v = c.GetWithTTL(nil, []byte("aaa"))
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from small cache: %q", v)
	}

	c.SetWithTTL([]byte("aaa"), []byte("bbb"), 3*time.Second)
	v = c.GetWithTTL(nil, []byte("aaa"))
	if string(v) != "bbb" {
		t.Fatalf("unexpected value obtained; got %q; want %q", v, "bbb")
	}

	c.Reset()
	v = c.GetWithTTL(nil, []byte("aaa"))
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from empty cache: %q", v)
	}

	// Test empty value
	k := []byte("empty")
	c.SetWithTTL(k, nil, 3*time.Second)
	v = c.GetWithTTL(nil, k)
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from empty entry: %q", v)
	}
	if !c.Has(k) {
		t.Fatalf("cannot find empty entry for key %q", k)
	}
	if c.Has([]byte("foobar")) {
		t.Fatalf("non-existing entry found in the cache")
	}
}

func TestTTLCacheWrap(t *testing.T) {
	c := New(bucketsCount * chunkSize * 1.5)
	defer c.Reset()

	calls := uint64(5e6)

	for i := uint64(0); i < calls; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.SetWithTTL(k, v, 70*time.Second)
		vv := c.GetWithTTL(nil, k)
		if string(vv) != string(v) {
			t.Fatalf("unexpected value for key %q; got %q; want %q", k, vv, v)
		}
	}
	for i := uint64(0); i < calls/10; i++ {
		x := i * 10
		k := []byte(fmt.Sprintf("key %d", x))
		v := []byte(fmt.Sprintf("value %d", x))
		vv := c.GetWithTTL(nil, k)
		if len(vv) > 0 && string(v) != string(vv) {
			t.Fatalf("unexpected value for key %q; got %q; want %q", k, vv, v)
		}
	}

	var s Stats
	c.UpdateStats(&s)
	getCalls := calls + calls/10
	if s.GetWithTTLCalls != getCalls {
		t.Fatalf("unexpected number of getCalls; got %d; want %d", s.GetWithTTLCalls, getCalls)
	}
	if s.SetWithTTLCalls != calls {
		t.Fatalf("unexpected number of setCalls; got %d; want %d", s.SetWithTTLCalls, calls)
	}
	// items will not be evicted here.
	if s.MissesWithTTL != 0 {
		t.Fatalf("unexpected number of misses; got %d; want 0", s.MissesWithTTL)
	}
	if s.Collisions != 0 {
		t.Fatalf("unexpected number of collisions; got %d; want 0", s.Collisions)
	}
	if s.EntriesCount < calls/5 {
		t.Fatalf("unexpected number of items; got %d; cannot be smaller than %d", s.EntriesCount, calls/5)
	}
	if s.BytesSize < 1024 {
		t.Fatalf("unexpected number of bytesSize; got %d; cannot be smaller than %d", s.BytesSize, 1024)
	}
}

func TestTTLCacheDel(t *testing.T) {
	c := New(1024)
	defer c.Reset()
	for i := 0; i < 100; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.SetWithTTL(k, v, 3*time.Second)
		vv := c.GetWithTTL(nil, k)
		if string(vv) != string(v) {
			t.Fatalf("unexpected value for key %q; got %q; want %q", k, vv, v)
		}
		c.Del(k)
		vv = c.GetWithTTL(nil, k)
		if len(vv) > 0 {
			t.Fatalf("unexpected non-empty value got for key %q: %q", k, vv)
		}
	}
}

func TestTTLCacheBigKeyValue(t *testing.T) {
	c := New(1024)
	defer c.Reset()

	// Both key and value exceed 64Kb
	k := make([]byte, 90*1024)
	v := make([]byte, 100*1024)
	c.SetWithTTL(k, v, 3*time.Second)
	vv := c.GetWithTTL(nil, k)
	if len(vv) > 0 {
		t.Fatalf("unexpected non-empty value got for key %q: %q", k, vv)
	}

	// len(key) + len(value) > 64Kb
	k = make([]byte, 40*1024)
	v = make([]byte, 40*1024)
	c.SetWithTTL(k, v, 3*time.Second)
	vv = c.GetWithTTL(nil, k)
	if len(vv) > 0 {
		t.Fatalf("unexpected non-empty value got for key %q: %q", k, vv)
	}
}

func TestTTLCacheSetGetSerial(t *testing.T) {
	itemsCount := 10000
	c := New(30 * itemsCount)
	defer c.Reset()
	if err := testTTLCacheGetSet(c, itemsCount); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestTTLCacheGetSetConcurrent(t *testing.T) {
	itemsCount := 10000
	const gorotines = 10
	c := New(30 * itemsCount * gorotines)
	defer c.Reset()

	ch := make(chan error, gorotines)
	for i := 0; i < gorotines; i++ {
		go func() {
			ch <- testTTLCacheGetSet(c, itemsCount)
		}()
	}
	for i := 0; i < gorotines; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func testTTLCacheGetSet(c *Cache, itemsCount int) error {
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.SetWithTTL(k, v, 3*time.Second)
		vv := c.GetWithTTL(nil, k)
		if string(vv) != string(v) {
			return fmt.Errorf("unexpected value for key %q after insertion; got %q; want %q", k, vv, v)
		}
	}
	misses := 0
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		vExpected := fmt.Sprintf("value %d", i)
		v := c.GetWithTTL(nil, k)
		if string(v) != string(vExpected) {
			if len(v) > 0 {
				return fmt.Errorf("unexpected value for key %q after all insertions; got %q; want %q", k, v, vExpected)
			}
			misses++
		}
	}
	if misses >= itemsCount/100 {
		return fmt.Errorf("too many cache misses; got %d; want less than %d", misses, itemsCount/100)
	}

	time.Sleep(3 * time.Second)
	misses = 0
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		vExpected := fmt.Sprintf("value %d", i)
		v := c.GetWithTTL(nil, k)
		if string(v) != string(vExpected) {
			if len(v) > 0 {
				return fmt.Errorf("unexpected value for key %q after all insertions; got %q; want %q", k, v, vExpected)
			}
			misses++
		}
	}
	if misses != itemsCount {
		return fmt.Errorf("at least one dead item returned: expected %d missses after sleep, got %d", itemsCount, misses)
	}
	return nil
}

func TestTTLCacheResetUpdateStatsSetConcurrent(t *testing.T) {
	c := New(12334)

	stopCh := make(chan struct{})

	// run workers for cache reset
	var resettersWG sync.WaitGroup
	for i := 0; i < 10; i++ {
		resettersWG.Add(1)
		go func() {
			defer resettersWG.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					c.Reset()
					runtime.Gosched()
				}
			}
		}()
	}

	// run workers for update cache stats
	var statsWG sync.WaitGroup
	for i := 0; i < 10; i++ {
		statsWG.Add(1)
		go func() {
			defer statsWG.Done()
			var s Stats
			for {
				select {
				case <-stopCh:
					return
				default:
					c.UpdateStats(&s)
					runtime.Gosched()
				}
			}
		}()
	}

	// run workers for setting data to cache
	var settersWG sync.WaitGroup
	for i := 0; i < 10; i++ {
		settersWG.Add(1)
		go func() {
			defer settersWG.Done()
			for j := 0; j < 100; j++ {
				key := []byte(fmt.Sprintf("key_%d", j))
				value := []byte(fmt.Sprintf("value_%d", j))
				c.SetWithTTL(key, value, 3*time.Second)
				runtime.Gosched()
			}
		}()
	}

	// wait for setters
	settersWG.Wait()
	close(stopCh)
	statsWG.Wait()
	resettersWG.Wait()
}
