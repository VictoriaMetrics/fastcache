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
* Fastcache automatically evicts old entries when reaching the maximum cache size
  set on its creation.
* [Simple API](http://godoc.org/github.com/VictoriaMetrics/fastcache).
* Simple source code.


### Benchmarks

`Fastcache` performance is compared to [BigCache](https://github.com/allegro/bigcache)
performance and to standard Go map performance.

```
GOMAXPROCS=4 go test github.com/VictoriaMetrics/fastcache -bench=. -benchtime=10s
goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/fastcache
BenchmarkBigCacheSet-4      	    2000	  10282267 ns/op	   6.37 MB/s	 4660372 B/op	       6 allocs/op
BenchmarkBigCacheGet-4      	    2000	   7001948 ns/op	   9.36 MB/s	  684170 B/op	  131076 allocs/op
BenchmarkBigCacheSetGet-4   	    1000	  17394537 ns/op	   7.54 MB/s	 5046744 B/op	  131083 allocs/op
BenchmarkCacheSet-4         	    5000	   3820051 ns/op	  17.16 MB/s	    4477 B/op	       1 allocs/op
BenchmarkCacheGet-4         	    5000	   2761515 ns/op	  23.73 MB/s	    4474 B/op	       1 allocs/op
BenchmarkCacheSetGet-4      	    2000	   9445288 ns/op	  13.88 MB/s	   11186 B/op	       4 allocs/op
BenchmarkStdMapSet-4        	    1000	  12417046 ns/op	   5.28 MB/s	  274692 B/op	   65538 allocs/op
BenchmarkStdMapGet-4        	    5000	   2898100 ns/op	  22.61 MB/s	    2560 B/op	      13 allocs/op
BenchmarkStdMapSetGet-4     	     100	 150924781 ns/op	   0.87 MB/s	  387478 B/op	   65559 allocs/op
```

`MB/s` column here actually means `millions of operations per second`.
As you can see, `fastcache` is faster than the `BigCache` in all cases
and faster than the standard Go map on workloads with inserts.


### Limitations

* Keys and values must be byte slices. Other types must be marshaled before
  storing them in the cache.
* Summary size of a (key, value) entry cannot exceed 64KB. Bigger values must be
  split into smaller values before storing in the cache.
* There is no cache expiration. Entries are evicted from the cache only
  on cache size overflow. Entry deadline may be stored inside the value in order
  to implement cache expiration.


### Architecture details

The cache uses ideas from [BigCache](https://github.com/allegro/bigcache):

* The cache consists of many buckets, each with its own lock.
  This helps scaling the performance on multi-core CPUs, since multiple
  CPUs may concurrently access distinct buckets.
* Each bucket consists of a `hash(key) -> (key, value) position` map
  and 64KB-sized byte slices (chunks) holding encoded `(key, value)` entries.
  Each bucket contains only `O(chunksCount)` pointers. For instance, 64GB cache
  would contain ~1M pointers, while similarly-sized `map[string][]byte`
  would contain ~1B pointers for short keys and values. This would lead to
  [huge GC overhead](https://syslog.ravelin.com/further-dangers-of-large-heaps-in-go-7a267b57d487).

64KB-sized chunks reduce memory fragmentation and the total memory usage comparing
to a single big chunk per bucket.


### Users

* `Fastcache` has been extracted from [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics) sources.
  See [this article](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac)
  for more info about `VictoriaMetrics`.


### FAQ

#### What is the difference between `fastcache` and other similar caches like [BigCache](https://github.com/allegro/bigcache) or [FreeCache](https://github.com/coocood/freecache)?

* `Fastcache` is faster. See benchmark results above.
* `Fastcache` uses less memory due to lower heap fragmentation. This allows
  saving many GBs of memory on multi-GB caches.
* `Fastcache` API [is simpler](http://godoc.org/github.com/VictoriaMetrics/fastcache).
  The API is designed to be used in zero-allocation mode.


#### Why `fastcache` doesn't support cache expiration?

Because we don't need cache expiration in [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics).
Cached entries inside `VictoriaMetrics` never expire. They are automatically evicted on cache size overflow.

It is easy to implement cache expiration on top of `fastcache` by caching values
with marshaled deadlines and verifying deadlines after reading these values
from the cache.


#### Why `fastcache` doesn't support advanced features such as [thundering herd protection](https://en.wikipedia.org/wiki/Thundering_herd_problem) or callbacks on entries' eviction?

Because these features would complicate the code and would make it slower.
`Fastcache` source code is simple - just copy-paste it and implement the feature you want
on top of it.
