// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package concurrent

import (
	"sync"
)

var (
	_defaultBatch = NewBatch()
)

type call struct {
	mux sync.RWMutex
	ret interface{}
	err error
}

// Batch .
type Batch struct {
	_mux      sync.Mutex
	_callings map[interface{}]*call
}

// Do .
func (o *Batch) Do(key interface{}, f func() (interface{}, error)) (interface{}, error) {
	o._mux.Lock()
	c, ok := o._callings[key]
	if ok {
		o._mux.Unlock()
		c.mux.RLock()
		c.mux.RUnlock()
		return c.ret, c.err
	}

	c = &call{}
	c.mux.Lock()
	o._callings[key] = c
	o._mux.Unlock()
	c.ret, c.err = f()
	c.mux.Unlock()

	o._mux.Lock()
	delete(o._callings, key)
	o._mux.Unlock()

	return c.ret, c.err
}

// NewBatch .
func NewBatch() *Batch {
	return &Batch{_callings: map[interface{}]*call{}}
}

// Do .
func Do(key interface{}, f func() (interface{}, error)) (interface{}, error) {
	return _defaultBatch.Do(key, f)
}
