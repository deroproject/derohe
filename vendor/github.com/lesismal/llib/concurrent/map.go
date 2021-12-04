package concurrent

import (
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash"
)

type bucket struct {
	mux    sync.RWMutex
	values map[string]interface{}
}

func (b *bucket) Get(k string) (interface{}, bool) {
	b.mux.RLock()
	v, ok := b.values[k]
	b.mux.RUnlock()
	return v, ok
}

func (b *bucket) Set(k string, v interface{}) bool {
	b.mux.Lock()
	_, exsist := b.values[k]
	b.values[k] = v
	b.mux.Unlock()
	return !exsist
}

func (b *bucket) Delete(k string) bool {
	b.mux.Lock()
	_, exsist := b.values[k]
	delete(b.values, k)
	b.mux.Unlock()
	return exsist
}

func (b *bucket) forEach(f func(k string, v interface{}) bool) bool {
	success := false
	b.mux.RLock()
	for k, v := range b.values {
		success = f(k, v)
		if !success {
			break
		}
	}
	b.mux.RUnlock()
	return success
}

type Map struct {
	size    int64
	buckets []*bucket
}

func (m *Map) Get(k string) (interface{}, bool) {
	i := hash(k) % uint64(len(m.buckets))
	return m.buckets[i].Get(k)
}

func (m *Map) Set(k string, v interface{}) {
	i := hash(k) % uint64(len(m.buckets))
	if m.buckets[i].Set(k, v) {
		atomic.AddInt64(&m.size, 1)
	}
}

func (m *Map) Delete(k string) {
	i := hash(k) % uint64(len(m.buckets))
	if m.buckets[i].Delete(k) {
		atomic.AddInt64(&m.size, -1)
	}
}

func (m *Map) Size() int64 {
	return atomic.LoadInt64(&m.size)
}

func (m *Map) ForEach(f func(k string, v interface{}) bool) {
	for _, b := range m.buckets {
		if !b.forEach(f) {
			return
		}
	}
}

func NewMap(bucketNum int) *Map {
	if bucketNum <= 0 {
		bucketNum = 64
	}
	m := &Map{buckets: make([]*bucket, bucketNum)}
	for i := 0; i < bucketNum; i++ {
		m.buckets[i] = &bucket{values: map[string]interface{}{}}
	}
	return m
}

func hash(k string) uint64 {
	return xxhash.Sum64String(k)
}
