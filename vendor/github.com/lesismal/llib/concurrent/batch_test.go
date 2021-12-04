// Copyright 2020 lesismal. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package concurrent

import (
	"log"
	"testing"
	"time"
)

func TestBatch(t *testing.T) {
	batchCall := func() (interface{}, error) {
		time.Sleep(time.Second)
		return time.Now().Format("2006/01/02 15:04:05.000"), nil
	}
	for i := 0; i < 10; i++ {
		go func(id int) {
			ret, err := Do(3, batchCall)
			log.Println("Batch().Do():", id, ret, err)
		}(2)
	}
	func(id int) {
		ret, err := Do(3, batchCall)
		log.Println("Batch().Do():", id, ret, err)
	}(1)

	func(id int) {
		ret, err := Do(3, batchCall)
		log.Println("Batch().Do():", id, ret, err)
	}(3)
	time.Sleep(time.Second)
}
