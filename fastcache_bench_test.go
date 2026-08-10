package fastcache

import (
	"testing"
	"time"
)

// BenchmarkBucketSetAtNextGen compares writes that create a next gen with
// writes that remain in the current gen. A next gen is created when the key
// cannot fit into the last chunk in this bucket.
//
//	go test -run '^$' -bench '^BenchmarkBucketSetAtNextGen$' -benchtime=5000000x -count=5 -benchmem
func BenchmarkBucketSetAtNextGen(b *testing.B) {
	var shard bucket
	// each chunk is 64 * 1024 bytes, so 1024 * 1024 bytes has 16 chunks
	shard.Init(1024 * 1024)
	defer shard.Reset()

	key := []byte("12345678")
	val := []byte(nil)
	// 4+len(key) because kvLen := uint64(len(kvLenBuf) + len(k) + len(v))
	// in the fastcache implementation.
	// - len(kvLenBuf) == 4,
	// - len(k) == len(Key)
	// - len(v) == len(val) == 0
	kvLen := uint64(4 + len(key))
	// do some warmups
	var h uint64 = 1
	for shard.gen < 3 {
		shard.Set(key, val, h)
		h++
	}

	var nextGenTotal time.Duration
	var nextGenMax time.Duration
	var nextGenCount uint64
	var currentGenTotal time.Duration
	var currentGenMax time.Duration
	var currentGenCount uint64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		if nextGen(&shard, kvLen) {
			shard.Set(key, val, h)
			d := time.Since(start)
			nextGenTotal += d
			if d > nextGenMax {
				nextGenMax = d
			}
			nextGenCount++
		} else {
			shard.Set(key, val, h)
			d := time.Since(start)
			currentGenTotal += d
			if d > currentGenMax {
				currentGenMax = d
			}
			currentGenCount++
		}
		h++
	}
	b.StopTimer()

	if nextGenCount > 0 {
		b.ReportMetric(float64(nextGenTotal.Nanoseconds())/float64(nextGenCount), "next-gen/ns/avg")
		b.ReportMetric(float64(nextGenMax.Nanoseconds()), "next-gen/ns/max")
		b.ReportMetric(float64(nextGenCount), "next-gen/count")
	}
	if currentGenCount > 0 {
		b.ReportMetric(float64(currentGenTotal.Nanoseconds())/float64(currentGenCount), "cur-gen/ns/avg")
		b.ReportMetric(float64(currentGenMax.Nanoseconds()), "cur-gen/ns/max")
		b.ReportMetric(float64(currentGenCount), "cur-gen/count")
	}
}

func nextGen(b *bucket, kvLen uint64) bool {
	idxNew := b.idx + kvLen
	chunkIdx := b.idx / chunkSize
	chunkIdxNew := idxNew / chunkSize
	return chunkIdxNew > chunkIdx && chunkIdxNew >= uint64(len(b.chunks))
}
