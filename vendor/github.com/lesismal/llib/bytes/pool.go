// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package bytes

import (
	"sync"
)

// maxAppendSize represents the max size to append to a slice.
const maxAppendSize = 1024 * 1024 * 4

// Pool is the default instance of []byte pool.
// User can customize a Pool implementation and reset this instance if needed.
var Pool interface {
	Get() []byte
	GetN(size int) []byte
	Put(b []byte)
} = NewPool(64)

// bufferPool is a default implementatiion of []byte Pool.
type bufferPool struct {
	sync.Pool
	MinSize int
}

// NewPool creates and returns a bufferPool instance.
// All slice created by this instance has an initial cap of minSize.
func NewPool(minSize int) *bufferPool {
	if minSize <= 0 {
		minSize = 64
	}
	bp := &bufferPool{
		MinSize: minSize,
	}
	bp.Pool.New = func() interface{} {
		buf := make([]byte, bp.MinSize)
		return &buf
	}
	return bp
}

// Get gets a slice from the pool and returns it with length 0.
// User can append the slice and should Put it back to the pool after being used over.
func (bp *bufferPool) Get() []byte {
	pbuf := bp.Pool.Get().(*[]byte)
	return (*pbuf)[0:0]
}

// GetN returns a slice with length size.
// To reuse slices as possible,
// if the cap of the slice got from the pool is not enough,
// It will append the slice,
// or put the slice back to the pool and create a new slice with cap of size.
//
// User can use the slice both by the size or append it,
// and should Put it back to the pool after being used over.
func (bp *bufferPool) GetN(size int) []byte {
	pbuf := bp.Pool.Get().(*[]byte)
	need := size - cap(*pbuf)
	if need > 0 {
		if need <= maxAppendSize {
			*pbuf = (*pbuf)[:cap(*pbuf)]
			*pbuf = append(*pbuf, make([]byte, need)...)
		} else {
			bp.Pool.Put(pbuf)
			newBuf := make([]byte, size)
			pbuf = &newBuf
		}
	}

	return (*pbuf)[:size]
}

// Put puts a slice back to the pool.
// If the slice's cap is smaller than MinSize,
// it will not be put back to the pool but dropped.
func (bp *bufferPool) Put(b []byte) {
	if cap(b) < bp.MinSize {
		return
	}
	bp.Pool.Put(&b)
}
