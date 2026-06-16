[![Build Status](https://github.com/VictoriaMetrics/fastcache/workflows/main/badge.svg)](https://github.com/VictoriaMetrics/fastcache/actions)
[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/fastcache?status.svg)](http://godoc.org/github.com/VictoriaMetrics/fastcache)
[![Go Report](https://goreportcard.com/badge/github.com/VictoriaMetrics/fastcache)](https://goreportcard.com/report/github.com/VictoriaMetrics/fastcache)
[![codecov](https://codecov.io/gh/VictoriaMetrics/fastcache/branch/master/graph/badge.svg)](https://codecov.io/gh/VictoriaMetrics/fastcache)

# fastcache - fast thread-safe inmemory cache for big number of entries in Go

### Features

* Fast. Performance scales on multi-core CPUs. See benchmark results below.
* Thread-safe. Concurrent goroutines may read and write into a single
  cache instance.
* The fastcache is designed for storing big number of entries without
  [GC overhead](https://syslog.ravelin.com/further-dangers-of-large-heaps-in-go-7a267b57d487).
* Fastcache automatically evicts old entries when reaching the maximum cache size
  set on its creation.
* [Simple API](http://godoc.org/github.com/VictoriaMetrics/fastcache).
* Simple source code.
* Cache may be [saved to file](https://godoc.org/github.com/VictoriaMetrics/fastcache#Cache.SaveToFile)
  and [loaded from file](https://godoc.org/github.com/VictoriaMetrics/fastcache#LoadFromFile).
* Works on [Google AppEngine](https://cloud.google.com/appengine/docs/go/).


### Benchmarks

`Fastcache` performance is compared with [BigCache](https://github.com/allegro/bigcache), standard Go map
and [sync.Map](https://golang.org/pkg/sync/#Map).

```
GOMAXPROCS=4 go test github.com/VictoriaMetrics/fastcache -bench='Set|Get' -benchtime=10s
goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/fastcache

BenchmarkBigCacheSet-4      	    2000	  10937855 ns/op	   5.99 MB/s	 4660369 B/op	       6 allocs/op
BenchmarkBigCacheGet-4      	    2000	   6985426 ns/op	   9.38 MB/s	  684169 B/op	  131076 allocs/op
BenchmarkBigCacheSetGet-4   	    1000	  17301294 ns/op	   7.58 MB/s	 5046746 B/op	  131083 allocs/op
BenchmarkCacheSet-4         	    5000	   3975946 ns/op	  16.48 MB/s	    1142 B/op	       2 allocs/op
BenchmarkCacheGet-4         	    5000	   3572679 ns/op	  18.34 MB/s	    1141 B/op	       2 allocs/op
BenchmarkCacheSetGet-4      	    2000	   9337256 ns/op	  14.04 MB/s	    2856 B/op	       5 allocs/op
BenchmarkStdMapSet-4        	    2000	  14684273 ns/op	   4.46 MB/s	  268423 B/op	   65537 allocs/op
BenchmarkStdMapGet-4        	    5000	   2833647 ns/op	  23.13 MB/s	    2561 B/op	      13 allocs/op
BenchmarkStdMapSetGet-4     	     100	 137417861 ns/op	   0.95 MB/s	  387356 B/op	   65558 allocs/op
BenchmarkSyncMapSet-4       	    1000	  23300189 ns/op	   2.81 MB/s	 3417183 B/op	  262277 allocs/op
BenchmarkSyncMapGet-4       	    5000	   2316508 ns/op	  28.29 MB/s	    2543 B/op	      79 allocs/op
BenchmarkSyncMapSetGet-4    	    2000	  10444529 ns/op	  12.55 MB/s	 3412527 B/op	  262210 allocs/op
BenchmarkSaveToFile-4       	      50	 259800249 ns/op	 129.15 MB/s	55739129 B/op	    3091 allocs/op
BenchmarkLoadFromFile-4     	     100	 121189395 ns/op	 276.88 MB/s	98089036 B/op	    8748 allocs/op
BenchmarkCache_VisitAllEntries-4   50000        245913 ns/op      40.66 MB/s         170 B/op          2 allocs/op
```

`MB/s` column here actually means `millions of operations per second`.
As you can see, `fastcache` is faster than the `BigCache` in all the cases.
`fastcache` is faster than the standard Go map and `sync.Map` on workloads
with inserts.


### Limitations

* Keys and values must be byte slices. Other types must be marshaled before
  storing them in the cache.
* Big entries with sizes exceeding 64KB must be stored via [distinct API](http://godoc.org/github.com/VictoriaMetrics/fastcache#Cache.SetBig).
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
Chunks are allocated off-heap if possible. This reduces total memory usage because
GC collects unused memory more frequently without the need in `GOGC` tweaking.


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
