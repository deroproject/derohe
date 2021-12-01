package bytes

import "testing"

func TestMemPool(t *testing.T) {
	const minMemSize = 64
	pool := NewPool(minMemSize)
	for i := 0; i < 1024*1024; i++ {
		buf := pool.GetN(i)
		if len(buf) != i {
			t.Fatalf("invalid length: %v != %v", len(buf), i)
		}
		pool.Put(buf)
	}
	for i := 1024 * 1024; i < 1024*1024*1024; i += 1024 * 1024 {
		buf := pool.GetN(i)
		if len(buf) != i {
			t.Fatalf("invalid length: %v != %v", len(buf), i)
		}
		pool.Put(buf)
	}
}
