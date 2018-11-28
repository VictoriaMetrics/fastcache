package fastcache

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/snappy"
)

// SaveToFile atomically saves cache data to the given filePath.
//
// SaveToFile may be called concurrently with other operations on the cache.
func (c *Cache) SaveToFile(filePath string) error {
	dir := filepath.Dir(filePath)
	tmpFile, err := ioutil.TempFile(dir, "fastcache.*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create temporary file for cache data: %s", err)
	}
	tmpFilePath := tmpFile.Name()
	if err := c.save(tmpFile); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFilePath)
		return fmt.Errorf("cannot save cache data to temporary file: %s", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFilePath)
		return fmt.Errorf("cannot close temporary file %s: %s", tmpFilePath, err)
	}
	if err := os.Rename(tmpFilePath, filePath); err != nil {
		_ = os.Remove(tmpFilePath)
		return fmt.Errorf("cannot move temporary file %s to %s: %s", tmpFilePath, filePath, err)
	}
	return nil
}

func (c *Cache) save(w io.Writer) error {
	bw := bufio.NewWriterSize(w, 1024*1024)
	zw := snappy.NewBufferedWriter(bw)
	maxBucketChunks := uint64(cap(c.buckets[0].chunks))
	if err := writeUint64(zw, maxBucketChunks); err != nil {
		return fmt.Errorf("cannot write maxBucketBytes=%d: %s", maxBucketChunks, err)
	}
	for i := range c.buckets[:] {
		if err := c.buckets[i].Save(zw); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot close snappy.Writer: %s", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cannot flush bufio.Writer: %s", err)
	}
	return nil
}

// LoadFromFile loads cache data from the given filePath.
func LoadFromFile(filePath string) (*Cache, error) {
	return loadFromFile(filePath, 0)
}

func loadFromFile(filePath string, maxBytes int) (*Cache, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %s", filePath, err)
	}
	defer file.Close()

	br := bufio.NewReaderSize(file, 1024*1024)
	zr := snappy.NewReader(br)
	maxBucketChunks, err := readUint64(zr)
	if err != nil {
		return nil, fmt.Errorf("cannot read maxBucketChunks: %s", err)
	}
	if maxBytes > 0 {
		maxBucketBytes := uint64((maxBytes + bucketsCount - 1) / bucketsCount)
		expectedBucketChunks := (maxBucketBytes + chunkSize - 1) / chunkSize
		if maxBucketChunks != expectedBucketChunks {
			return nil, fmt.Errorf("cache file %s contains maxBytes=%d; want %d", filePath, maxBytes, expectedBucketChunks*chunkSize*bucketsCount)
		}
	}
	var c Cache
	for i := range c.buckets[:] {
		if err := c.buckets[i].Load(zr, maxBucketChunks); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

// LoadFromFileOrNew tries loading cache data from the given filePath.
//
// The function falls back to creating new cache with the given maxBytes
// capacity if error occurs during loading the cache from file.
func LoadFromFileOrNew(filePath string, maxBytes int) *Cache {
	c, err := loadFromFile(filePath, maxBytes)
	if err == nil {
		return c
	}
	return New(maxBytes)
}

func (b *bucket) Save(w io.Writer) error {
	b.Clean()

	// Store b.idx, b.gen and b.m to w.
	b.mu.RLock()
	bIdx := b.idx
	bGen := b.gen
	chunksLen := 0
	for _, chunk := range b.chunks {
		if chunk == nil {
			break
		}
		chunksLen++
	}
	kvs := make([]byte, 0, 2*8*len(b.m))
	var u64Buf [8]byte
	for k, v := range b.m {
		binary.LittleEndian.PutUint64(u64Buf[:], k)
		kvs = append(kvs, u64Buf[:]...)
		binary.LittleEndian.PutUint64(u64Buf[:], v)
		kvs = append(kvs, u64Buf[:]...)
	}
	b.mu.RUnlock()

	if err := writeUint64(w, bIdx); err != nil {
		return fmt.Errorf("cannot write b.idx: %s", err)
	}
	if err := writeUint64(w, bGen); err != nil {
		return fmt.Errorf("cannot write b.gen: %s", err)
	}
	if err := writeUint64(w, uint64(len(kvs))/2/8); err != nil {
		return fmt.Errorf("cannot write len(b.m): %s", err)
	}
	if _, err := w.Write(kvs); err != nil {
		return fmt.Errorf("cannot write b.m: %s", err)
	}

	// Store b.chunks to w.
	if err := writeUint64(w, uint64(chunksLen)); err != nil {
		return fmt.Errorf("cannot write len(b.chunks): %s", err)
	}
	chunk := make([]byte, chunkSize)
	for chunkIdx := 0; chunkIdx < chunksLen; chunkIdx++ {
		b.mu.RLock()
		copy(chunk, b.chunks[chunkIdx][:chunkSize])
		b.mu.RUnlock()
		if _, err := w.Write(chunk); err != nil {
			return fmt.Errorf("cannot write b.chunks[%d]: %s", chunkIdx, err)
		}
	}

	return nil
}

func (b *bucket) Load(r io.Reader, maxChunks uint64) error {
	bIdx, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read b.idx: %s", err)
	}
	bGen, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read b.gen: %s", err)
	}
	kvsLen, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read len(b.m): %s", err)
	}
	kvsLen *= 2 * 8
	kvs := make([]byte, kvsLen)
	if _, err := io.ReadFull(r, kvs); err != nil {
		return fmt.Errorf("cannot read b.m: %s", err)
	}
	m := make(map[uint64]uint64, kvsLen/2/8)
	for len(kvs) > 0 {
		k := binary.LittleEndian.Uint64(kvs)
		kvs = kvs[8:]
		v := binary.LittleEndian.Uint64(kvs)
		kvs = kvs[8:]
		m[k] = v
	}

	maxBytes := maxChunks * chunkSize
	if maxBytes >= (1 << 40) {
		return fmt.Errorf("too big maxBytes=%d; should be smaller than %d", maxBytes, 1<<40)
	}
	chunks := make([][]byte, maxChunks)
	chunksLen, err := readUint64(r)
	if err != nil {
		return fmt.Errorf("cannot read len(b.chunks): %s", err)
	}
	if chunksLen > uint64(maxChunks) {
		return fmt.Errorf("chunksLen=%d cannot exceed maxChunks=%d", chunksLen, maxChunks)
	}
	for chunkIdx := uint64(0); chunkIdx < chunksLen; chunkIdx++ {
		chunk := make([]byte, chunkSize)
		if _, err := io.ReadFull(r, chunk); err != nil {
			return fmt.Errorf("cannot read b.chunks[%d]: %s", chunkIdx, err)
		}
		chunks[chunkIdx] = chunk
	}

	b.mu.Lock()
	b.chunks = chunks
	b.m = m
	b.idx = bIdx
	b.gen = bGen
	b.mu.Unlock()

	return nil
}

func writeUint64(w io.Writer, u uint64) error {
	var u64Buf [8]byte
	binary.LittleEndian.PutUint64(u64Buf[:], u)
	_, err := w.Write(u64Buf[:])
	return err
}

func readUint64(r io.Reader) (uint64, error) {
	var u64Buf [8]byte
	if _, err := io.ReadFull(r, u64Buf[:]); err != nil {
		return 0, err
	}
	u := binary.LittleEndian.Uint64(u64Buf[:])
	return u, nil
}
