// Copyright 2017-2021 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package block

import "fmt"
import "sort"
import "sync"

type MiniBlocksCollection struct {
	Collection map[MiniBlockKey][]MiniBlock
	sync.RWMutex
}

// create a collection
func CreateMiniBlockCollection() *MiniBlocksCollection {
	return &MiniBlocksCollection{Collection: map[MiniBlockKey][]MiniBlock{}}
}

// purge all heights less than this height
func (c *MiniBlocksCollection) PurgeHeight(height int64) (purge_count int) {
	if height < 0 {
		return
	}
	c.Lock()
	defer c.Unlock()

	for k, _ := range c.Collection {
		if k.Height <= uint64(height) {
			purge_count++
			delete(c.Collection, k)
		}
	}
	return purge_count
}

func (c *MiniBlocksCollection) Count() int {
	c.RLock()
	defer c.RUnlock()
	count := 0
	for _, v := range c.Collection {
		count += len(v)
	}

	return count
}

// check if already inserted
func (c *MiniBlocksCollection) IsAlreadyInserted(mbl MiniBlock) bool {
	return c.IsCollision(mbl)
}

// check if collision will occur
func (c *MiniBlocksCollection) IsCollision(mbl MiniBlock) bool {
	c.RLock()
	defer c.RUnlock()

	return c.isCollisionnolock(mbl)
}

// this assumes that we are already locked
func (c *MiniBlocksCollection) isCollisionnolock(mbl MiniBlock) bool {
	mbls := c.Collection[mbl.GetKey()]
	for i := range mbls {
		if mbl == mbls[i] {
			return true
		}
	}
	return false
}

// insert a miniblock
func (c *MiniBlocksCollection) InsertMiniBlock(mbl MiniBlock) (err error, result bool) {
	if mbl.Final {
		return fmt.Errorf("Final cannot be inserted"), false
	}

	c.Lock()
	defer c.Unlock()

	if c.isCollisionnolock(mbl) {
		return fmt.Errorf("collision %x", mbl.Serialize()), false
	}

	c.Collection[mbl.GetKey()] = append(c.Collection[mbl.GetKey()], mbl)
	return nil, true
}

// get all the genesis blocks
func (c *MiniBlocksCollection) GetAllMiniBlocks(key MiniBlockKey) (mbls []MiniBlock) {
	c.RLock()
	defer c.RUnlock()

	for _, mbl := range c.Collection[key] {
		mbls = append(mbls, mbl)
	}
	return
}

// get all the tips from the map, this is atleast O(n)
func (c *MiniBlocksCollection) GetAllKeys(height int64) (keys []MiniBlockKey) {
	c.RLock()
	defer c.RUnlock()

	for k := range c.Collection {
		if k.Height == uint64(height) {
			keys = append(keys, k)
		}
	}

	sort.SliceStable(keys, func(i, j int) bool { // sort descending on the basis of work done
		return len(c.Collection[keys[i]]) > len(c.Collection[keys[j]])
	})

	return
}
