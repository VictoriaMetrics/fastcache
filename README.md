[![Build Status](https://travis-ci.org/VictoriaMetrics/fastcache.svg)](https://travis-ci.org/VictoriaMetrics/fastcache)
[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/fastcache?status.svg)](http://godoc.org/github.com/VictoriaMetrics/fastcache)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/fastcache)](https://goreportcard.com/report/github.com/VictoriaMetrics/fastcache)
[![codecov](https://codecov.io/gh/VictoriaMetrics/fastcache/branch/master/graph/badge.svg)](https://codecov.io/gh/VictoriaMetrics/fastcache)

# fastcache - fast off-heap thread-safe inmemory cache for Go

### Features

* Fast. Performance scales on multi-core CPUs. See benchmark results below.
* Thread-safe. Concurrent goroutines may read and write into a single
  cache instance.
* The fastcache is designed for storing big number of items without
  [GC overhead](https://syslog.ravelin.com/further-dangers-of-large-heaps-in-go-7a267b57d487).
* Fastcache automatically evicts old entries when reaching the maximum size
  set during its creation.
* [Simple API](http://godoc.org/github.com/VictoriaMetrics/fastcache).


### Benchmarks

`Fastcache` performance is compared to [BigCache](https://github.com/allegro/bigcache)
performance and to standard Go map performance.

```
GOMAXPROCS=4 go test ./lib/fastcache/ -run=111 -bench=. -benchtime=10s
goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/fastcache
BenchmarkBigCacheSet-4      	    2000	  11534981 ns/op	   5.68 MB/s	 4660371 B/op	       6 allocs/op
BenchmarkBigCacheGet-4      	    2000	   6950758 ns/op	   9.43 MB/s	  684169 B/op	  131076 allocs/op
BenchmarkBigCacheSetGet-4   	    2000	  11381738 ns/op	   6.33 MB/s	 4712794 B/op	   13112 allocs/op
BenchmarkCacheSet-4         	    3000	   3965871 ns/op	  16.52 MB/s	    7459 B/op	       2 allocs/op
BenchmarkCacheGet-4         	    5000	   2822254 ns/op	  23.22 MB/s	    4475 B/op	       1 allocs/op
BenchmarkCacheSetGet-4      	    3000	   5219393 ns/op	  13.81 MB/s	    7461 B/op	       2 allocs/op
BenchmarkStdMapSet-4        	    2000	  12009655 ns/op	   5.46 MB/s	  268430 B/op	   65537 allocs/op
BenchmarkStdMapGet-4        	    5000	   2747309 ns/op	  23.85 MB/s	    2563 B/op	      13 allocs/op
BenchmarkStdMapSetGet-4     	     300	  44032198 ns/op	   1.64 MB/s	  303889 B/op	   65543 allocs/op
PASS
```

As you can see, `fastcache` is faster than the `BigCache` in all cases
and faster than the standard Go map on workloads with inserts.


### Limitations

* Keys and values must be byte slices.
* Summary size of a (key, value) entry cannot exceed 64KB.
* There is no cache expiration. Entries are evicted from the cache only
  on overflow.


### Architecture details

The cache uses ideas from [BigCache](https://github.com/allegro/bigcache):

* The cache consists of many buckets, each with its own lock.
  This helps scaling the performance on multi-core CPUs, since multiple
  CPUs may concurrently access distinct buckets.
* Each bucket consists of a `hash(key) -> (key, value) position` map
  and 64KB-sized byte slices (chunks) holding encoded `(key, value)` entries.
  Each bucket contains only `O(chunksCount)` pointers. For instance, 16GB cache
  would contain ~1M pointers, while similarly-sized `map[string][]byte` for short
  keys and values would contain ~1B more pointers, leading to
  [huge GC overhead](https://syslog.ravelin.com/further-dangers-of-large-heaps-in-go-7a267b57d487).

64KB-sized chunks reduce memory fragmentation and the total memory usage comparing
to a single big chunk per bucket.


### Users

* `Fastcache` has been extracted from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) sources.
  See [this article](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac)
  for more info about `VictoriaMetrics`.
