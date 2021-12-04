// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package concurrent

import (
	"sync"
)

var (
	_defaultMux = NewMutex()
)

// Mutex .
type Mutex struct {
	_mux   sync.Mutex
	_muxes map[interface{}]*sync.Mutex
}

// Lock .
func (m *Mutex) Lock(key interface{}) {
	m._mux.Lock()
	mux, ok := m._muxes[key]
	if !ok {
		mux = &sync.Mutex{}
		m._muxes[key] = mux
	}
	m._mux.Unlock()
	mux.Lock()
}

// Unlock .
func (m *Mutex) Unlock(key interface{}) {
	m._mux.Lock()
	mux, ok := m._muxes[key]
	m._mux.Unlock()
	if ok {
		mux.Unlock()
	}
}

// NewMutex .
func NewMutex() *Mutex {
	return &Mutex{_muxes: map[interface{}]*sync.Mutex{}}
}

// // Lock .
// func Lock(key interface{}) {
// 	_defaultMux.Lock(key)
// }

// // Unlock .
// func Unlock(key interface{}) {
// 	_defaultMux.Unlock(key)
// }
