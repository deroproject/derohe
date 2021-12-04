package histogram

import (
	"sync"
	"testing"
)

func BenchmarkFastUpdate(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		f := NewFast()
		var v float64
		for pb.Next() {
			f.Update(v)
			v += 1.5
		}
		SinkLock.Lock()
		Sink += f.Quantile(0.5)
		SinkLock.Unlock()
	})
}

var Sink float64
var SinkLock sync.Mutex
