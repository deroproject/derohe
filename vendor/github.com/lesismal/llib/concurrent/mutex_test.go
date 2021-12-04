// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package concurrent

import (
	"log"
	"testing"
	"time"
)

func TestMutex(t *testing.T) {
	mux := NewMutex()
	muxPrint := func(id int) {
		for i := 0; i < 3; i++ {
			mux.Lock(1)
			time.Sleep(time.Second / 100)
			log.Println("mux print:", id, i)
			mux.Unlock(1)
		}
	}
	go muxPrint(2)
	muxPrint(1)
	time.Sleep(time.Second / 10)
}
