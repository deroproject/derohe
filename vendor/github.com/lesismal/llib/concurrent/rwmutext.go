// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package concurrent

import (
	"sync"
)

var (
	_defaultRWMux = NewRWMutex()
)

// RWMutex .
type RWMutex struct {
	_mux     sync.Mutex
	_rwmuxes map[interface{}]*sync.RWMutex
}

// Lock .
func (m *RWMutex) Lock(key interface{}) {
	m._mux.Lock()
	mux, ok := m._rwmuxes[key]
	if !ok {
		mux = &sync.RWMutex{}
		m._rwmuxes[key] = mux
	}
	m._mux.Unlock()
	mux.Lock()
}

// Unlock .
func (m *RWMutex) Unlock(key interface{}) {
	m._mux.Lock()
	mux, ok := m._rwmuxes[key]
	m._mux.Unlock()
	if ok {
		mux.Unlock()
	}
}

// RLock .
func (m *RWMutex) RLock(key interface{}) {
	m._mux.Lock()
	mux, ok := m._rwmuxes[key]
	if !ok {
		mux = &sync.RWMutex{}
		m._rwmuxes[key] = mux
	}
	m._mux.Unlock()
	mux.RLock()
}

// RUnlock .
func (m *RWMutex) RUnlock(key interface{}) {
	m._mux.Lock()
	mux, ok := m._rwmuxes[key]
	m._mux.Unlock()
	if ok {
		mux.RUnlock()
	}
}

// NewRWMutex .
func NewRWMutex() *RWMutex {
	return &RWMutex{_rwmuxes: map[interface{}]*sync.RWMutex{}}
}

// Lock .
func Lock(key interface{}) {
	_defaultRWMux.Lock(key)
}

// Unlock .
func Unlock(key interface{}) {
	_defaultRWMux.Unlock(key)
}

// RLock .
func RLock(key interface{}) {
	_defaultRWMux.RLock(key)
}

// RUnlock .
func RUnlock(key interface{}) {
	_defaultRWMux.RUnlock(key)
}
