package fastcache

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSaveLoadSmall(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(tmpDir, "TestSaveLoadSmall.fastcache")
	defer os.RemoveAll(filePath)

	c := New(1)
	defer c.Reset()

	key := []byte("foobar")
	value := []byte("abcdef")
	c.Set(key, value)
	if err := c.SaveToFile(filePath); err != nil {
		t.Fatalf("SaveToFile error: %s", err)
	}

	c1, err := LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadFromFile error: %s", err)
	}
	vv := c1.Get(nil, key)
	if string(vv) != string(value) {
		t.Fatalf("unexpected value obtained from cache; got %q; want %q", vv, value)
	}

	// Verify that key can be overwritten.
	newValue := []byte("234fdfd")
	c1.Set(key, newValue)
	vv = c1.Get(nil, key)
	if string(vv) != string(newValue) {
		t.Fatalf("unexpected new value obtained from cache; got %q; want %q", vv, newValue)
	}
}

func TestSaveLoadFile(t *testing.T) {
	for _, concurrency := range []int{0, 1, 2, 4, 10} {
		t.Run(fmt.Sprintf("concurrency_%d", concurrency), func(t *testing.T) {
			testSaveLoadFile(t, concurrency)
		})
	}
}

func testSaveLoadFile(t *testing.T, concurrency int) {
	var s Stats
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(tmpDir, fmt.Sprintf("TestSaveLoadFile.%d.fastcache", concurrency))
	defer os.RemoveAll(filePath)

	const itemsCount = 10000
	const maxBytes = bucketsCount * chunkSize * 2
	c := New(maxBytes)
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.Set(k, v)
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}
	if concurrency == 1 {
		if err := c.SaveToFile(filePath); err != nil {
			t.Fatalf("SaveToFile error: %s", err)
		}
	} else {
		if err := c.SaveToFileConcurrent(filePath, concurrency); err != nil {
			t.Fatalf("SaveToFileConcurrent(%d) error: %s", concurrency, err)
		}
	}
	s.Reset()
	c.UpdateStats(&s)
	if s.EntriesCount != itemsCount {
		t.Fatalf("unexpected entriesCount; got %d; want %d", s.EntriesCount, itemsCount)
	}
	c.Reset()

	// Verify LoadFromFile
	c, err = LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	s.Reset()
	c.UpdateStats(&s)
	if s.EntriesCount != itemsCount {
		t.Fatalf("unexpected entriesCount; got %d; want %d", s.EntriesCount, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}
	c.Reset()

	// Verify LoadFromFileOrNew
	c = LoadFromFileOrNew(filePath, maxBytes)
	s.Reset()
	c.UpdateStats(&s)
	if s.EntriesCount != itemsCount {
		t.Fatalf("unexpected entriesCount; got %d; want %d", s.EntriesCount, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}
	c.Reset()

	// Overwrite existing keys
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.Set(k, v)
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}

	// Add new keys
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("new key %d", i))
		v := []byte(fmt.Sprintf("new value %d", i))
		c.Set(k, v)
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}

	// Verify all the keys exist
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
		k = []byte(fmt.Sprintf("new key %d", i))
		v = []byte(fmt.Sprintf("new value %d", i))
		vv = c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}

	// Verify incorrect maxBytes passed to LoadFromFileOrNew
	c = LoadFromFileOrNew(filePath, maxBytes*10)
	s.Reset()
	c.UpdateStats(&s)
	if s.EntriesCount != 0 {
		t.Fatalf("unexpected non-zero entriesCount; got %d", s.EntriesCount)
	}
	c.Reset()
}

func TestSaveLoadConcurrent(t *testing.T) {
	c := New(1024)
	defer c.Reset()
	c.Set([]byte("foo"), []byte("bar"))

	stopCh := make(chan struct{})

	// Start concurrent workers that run Get and Set on c.
	var wgWorkers sync.WaitGroup
	for i := 0; i < 5; i++ {
		wgWorkers.Add(1)
		go func() {
			defer wgWorkers.Done()
			var buf []byte
			j := 0
			for {
				k := []byte(fmt.Sprintf("key %d", j))
				v := []byte(fmt.Sprintf("value %d", j))
				c.Set(k, v)
				buf = c.Get(buf[:0], k)
				if string(buf) != string(v) {
					panic(fmt.Errorf("unexpected value for key %q; got %q; want %q", k, buf, v))
				}
				j++
				select {
				case <-stopCh:
					return
				default:
				}
			}
		}()
	}

	// Start concurrent SaveToFile and LoadFromFile calls.
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	var wgSavers sync.WaitGroup
	for i := 0; i < 4; i++ {
		wgSavers.Add(1)
		filePath := filepath.Join(tmpDir, fmt.Sprintf("TestSaveLoadFile.%d.fastcache", i))
		go func() {
			defer wgSavers.Done()
			defer os.RemoveAll(filePath)
			for j := 0; j < 3; j++ {
				if err := c.SaveToFileConcurrent(filePath, 3); err != nil {
					panic(fmt.Errorf("cannot save cache to %q: %s", filePath, err))
				}
				cc, err := LoadFromFile(filePath)
				if err != nil {
					panic(fmt.Errorf("cannot load cache from %q: %s", filePath, err))
				}
				var s Stats
				cc.UpdateStats(&s)
				if s.EntriesCount == 0 {
					panic(fmt.Errorf("unexpected empty cache loaded from %q", filePath))
				}
				cc.Reset()
			}
		}()
	}

	wgSavers.Wait()

	close(stopCh)
	wgWorkers.Wait()
}
