// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package concurrent

import (
	"log"
	"testing"
	"time"
)

func TestRWMutex(t *testing.T) {
	rwmux := NewRWMutex()
	rwmuxRLockPrint := func(id int) {
		for i := 0; i < 3; i++ {
			rwmux.RLock(2)
			time.Sleep(time.Second / 100)
			log.Println("rwmux print:", id, i)
			rwmux.RUnlock(2)
		}
	}
	go rwmuxRLockPrint(2)
	rwmuxRLockPrint(1)

	rwmuxLockPrint := func(id int) {
		for i := 0; i < 3; i++ {
			rwmux.Lock(2)
			time.Sleep(time.Second / 100)
			log.Println("rwmux print:", id, i)
			rwmux.Unlock(2)
		}
	}
	go rwmuxLockPrint(2)
	rwmuxLockPrint(1)

	time.Sleep(time.Second / 10)
}
