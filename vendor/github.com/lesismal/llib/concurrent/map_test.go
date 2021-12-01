// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package concurrent

import (
	"fmt"
	"log"
	"testing"
)

func TestMap(t *testing.T) {
	m := NewMap(64)
	size := 100000
	for i := 0; i < size; i++ {
		k := fmt.Sprintf("key_%d", i)
		v := fmt.Sprintf("value_%d", i)
		vv, ok := m.Get(k)
		if ok {
			log.Fatalf("[%v] exists: '%v'", k, vv)
		}
		m.Set(k, v)
		vv, ok = m.Get(k)
		if !ok {
			log.Fatalf("[%v] does not exist: '%v'", k, vv)
		}
		if v != vv {
			log.Fatalf("invalid value: '%v' for key [%v] ", vv, k)
		}
	}
	cnt := 0
	m.ForEach(func(k string, v interface{}) bool {
		if k[3:] != (v.(string))[5:] {
			log.Fatalf("invalid key-value: '%v', '%v'", k, v)
		}
		cnt++
		return true
	})
	if cnt != size {
		log.Fatalf("invalid ForEach num: %v, want: %v", cnt, size)
	}
	if m.Size() != int64(size) {
		log.Fatalf("invalid size: %v, want: %v", m.Size(), size)
	}
	for i := 0; i < size; i++ {
		k := fmt.Sprintf("key_%d", i)
		v := fmt.Sprintf("value_%d", i)
		vv, ok := m.Get(k)
		if !ok {
			log.Fatalf("[%v] does not exist: '%v'", k, vv)
		}
		if v != vv {
			log.Fatalf("invalid value: '%v' for key [%v]", vv, k)
		}
		m.Delete(k)
		if m.Size() != int64(size-i-1) {
			log.Fatalf("invalid size: %v, want: %v", m.Size(), int64(size-i-1))
		}
	}
	for i := 0; i < size; i++ {
		k := fmt.Sprintf("key_%d", i)
		vv, ok := m.Get(k)
		if ok {
			log.Fatalf("[%v] exists: '%v'", k, vv)
		}
		if m.Size() != 0 {
			log.Fatalf("invalid size: %v, want: %v", m.Size(), 0)
		}
	}
}

func BenchmarkMapSet(b *testing.B) {
	m := NewMap(64)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		k := fmt.Sprintf("key_%d", i)
		v := fmt.Sprintf("value_%d", i)
		m.Set(k, v)
	}
}

func BenchmarkMapGet(b *testing.B) {
	m := NewMap(64)

	for i := 0; i < b.N; i++ {
		k := fmt.Sprintf("key_%d", i)
		v := fmt.Sprintf("value_%d", i)
		m.Set(k, v)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		k := fmt.Sprintf("key_%d", i)
		v := fmt.Sprintf("value_%d", i)
		vv, ok := m.Get(k)
		if !ok {
			log.Fatalf("[%v] does not exist: '%v'", k, vv)
		}
		if v != vv {
			log.Fatalf("invalid value: '%v' for key [%v], want: %v", vv, k, v)
		}
	}
}
