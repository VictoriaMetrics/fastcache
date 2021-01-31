package fastcache

import (
	"sync"
)

type iteratorError string

func (e iteratorError) Error() string {
	return string(e)
}

// ErrIterationFinished is reported when Value() is called after reached to the end of the iterator
const ErrIterationFinished = iteratorError("iterator reached the last element, Value() should not be called")

var emptyEntry = Entry{}

// Entry represents a key-value pair in fastcache
type Entry struct {
	key       []byte
	value     []byte
}

// Key returns entry's key
func (e Entry) Key() []byte {
	return e.key
}

// Value returns entry's value
func (e Entry) Value() []byte {
	return e.value
}

func newIterator(c *Cache) *Iterator {
	elements, count := c.buckets[0].copyKeys()

	return &Iterator{
		cache:          c,
		currBucketIdx:  0,
		currKeyIdx:     -1,
		currBucketKeys: elements,
		currBucketSize: count,
	}
}

// Iterator allows to iterate over entries in the cache
type Iterator struct {
	mu               sync.Mutex
	cache            *Cache
	currBucketSize   int
	currBucketIdx    int
	currBucketKeys   [][]byte
	currKeyIdx       int
	currentEntryInfo Entry

	valid bool
}

// SetNext moves to the next element and returns true if the value exists.
func (it *Iterator) SetNext() bool {
	it.mu.Lock()

	it.valid = false
	it.currKeyIdx++

	// In case there are remaining currBucketKeys in the current bucket.
	if it.currBucketSize > it.currKeyIdx {
		it.valid = true
		found := it.setCurrentEntry()
		it.mu.Unlock()

		// if not found, check the next entry
		if !found {
			return it.SetNext()
		}
		return true
	}

	// If we reached the end of a bucket, check the next one for further iteration.
	for i := it.currBucketIdx + 1; i < len(it.cache.buckets); i++ {
		it.currBucketKeys, it.currBucketSize = it.cache.buckets[i].copyKeys()

		// bucket is not an empty one, use it for iteration
		if it.currBucketSize > 0 {
			it.currKeyIdx = 0
			it.currBucketIdx = i
			it.valid = true
			found := it.setCurrentEntry()
			it.mu.Unlock()

			// if not found, check the next entry
			if !found {
				return it.SetNext()
			}
			return true
		}
	}
	it.mu.Unlock()
	return false
}

func (it *Iterator) setCurrentEntry() bool {
	key := it.currBucketKeys[it.currKeyIdx]
	val, found := it.cache.HasGet(nil, key)

	if found {
		it.currentEntryInfo = Entry{
			key:   key,
			value: val,
		}
	} else {
		it.currentEntryInfo = emptyEntry
	}

	return found
}

// Value returns the current entry of an iterator.
func (it *Iterator) Value() (Entry, error) {
	if !it.valid {
		return emptyEntry, ErrIterationFinished
	}

	return it.currentEntryInfo, nil
}
