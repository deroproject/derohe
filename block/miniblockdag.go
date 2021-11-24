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
import "strings"

//import "runtime/debug"
import "encoding/binary"

//import "golang.org/x/crypto/sha3"

import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derohe/astrobwt"

type MiniBlocksCollection struct {
	Collection map[uint32]MiniBlock
	sync.RWMutex
}

// create a collection
func CreateMiniBlockCollection() *MiniBlocksCollection {
	return &MiniBlocksCollection{Collection: map[uint32]MiniBlock{}}
}

// purge all heights less than this height
func (c *MiniBlocksCollection) PurgeHeight(height int64) (purge_count int) {
	c.Lock()
	defer c.Unlock()

	for k, mbl := range c.Collection {
		if mbl.Height <= height {
			purge_count++
			delete(c.Collection, k)
		}
	}
	return purge_count
}

func (c *MiniBlocksCollection) Count() int {
	c.Lock()
	defer c.Unlock()
	return len(c.Collection)
}

// check if already inserted
func (c *MiniBlocksCollection) IsAlreadyInserted(mbl MiniBlock) bool {
	return c.IsCollision(mbl)
}

// check if collision will occur
func (c *MiniBlocksCollection) IsCollision(mbl MiniBlock) bool {
	uid := mbl.GetMiniID()
	c.RLock()
	defer c.RUnlock()

	if _, ok := c.Collection[uid]; ok {
		//fmt.Printf("collision uid %08X %s   %08X %s stack %s\n",uid,mbl.GetHash(), col.GetMiniID(), col.GetHash(),debug.Stack())
		return true
	}
	return false
}

// check whether the miniblock is connected
func (c *MiniBlocksCollection) IsConnected(mbl MiniBlock) bool {
	if mbl.Genesis {
		return true
	}
	c.RLock()
	defer c.RUnlock()

	for i := uint8(0); i < mbl.PastCount; i++ {
		if _, ok := c.Collection[mbl.Past[i]]; !ok {
			return false
		}
	}
	return true
}

// get distance from base
func (c *MiniBlocksCollection) CalculateDistance(mbl MiniBlock) uint32 {
	if mbl.Genesis {
		return 0
	}
	c.RLock()
	defer c.RUnlock()

	max_distance := uint32(0)
	for i := uint8(0); i < mbl.PastCount; i++ {
		if prev, ok := c.Collection[mbl.Past[i]]; !ok {
			panic("past should be present")
		} else if prev.Distance > max_distance {
			max_distance = prev.Distance
		}
	}
	return max_distance
}

func (c *MiniBlocksCollection) Get(id uint32) (mbl MiniBlock) {
	c.RLock()
	defer c.RUnlock()
	var ok bool

	if mbl, ok = c.Collection[id]; !ok {
		panic("id requested should be present")
	}
	return mbl
}

// insert a miniblock
func (c *MiniBlocksCollection) InsertMiniBlock(mbl MiniBlock) (err error, result bool) {
	if c.IsCollision(mbl) {
		return fmt.Errorf("collision %x", mbl.Serialize()), false
	}

	if !c.IsConnected(mbl) {
		return fmt.Errorf("not connected"), false
	}

	if mbl.Genesis && mbl.Odd {
		return fmt.Errorf("genesis cannot be odd height"), false
	}

	prev_distance := c.CalculateDistance(mbl)
	hash := mbl.GetHash()
	uid := binary.BigEndian.Uint32(hash[:])

	c.Lock()
	defer c.Unlock()

	if _, ok := c.Collection[uid]; ok {
		return fmt.Errorf("collision1"), false
	}

	if mbl.Genesis {
		mbl.Height = int64(binary.BigEndian.Uint64(mbl.Check[:]))
	} else {
		for i := uint8(0); i < mbl.PastCount; i++ {
			if prev, ok := c.Collection[mbl.Past[i]]; !ok {
				return fmt.Errorf("no past found"), false
			} else {
				if mbl.Timestamp < prev.Timestamp {
					return fmt.Errorf("timestamp less than parent"), false // childs timestamp cannot be less than parent, atleast one is fudging
				}
				mbl.PastMiniBlocks = append(mbl.PastMiniBlocks, prev)
				mbl.Height = prev.Height
			}
		}
		if mbl.Odd != (prev_distance%2 == 1) {
			return fmt.Errorf("invalid odd status prev %d  odd %+v", prev_distance, mbl.Odd), false
		}
		mbl.Distance = prev_distance + 1
	}
	c.Collection[uid] = mbl
	return nil, true
}

// get all the genesis blocks
func (c *MiniBlocksCollection) GetAllGenesisMiniBlocks() (mbls []MiniBlock) {
	c.Lock()
	defer c.Unlock()

	for _, mbl := range c.Collection {
		if mbl.Genesis {
			mbls = append(mbls, mbl)
		}
	}
	return
}

// get all the tips from the map, this is n²
func (c *MiniBlocksCollection) GetAllTips() (mbls []MiniBlock) {
	c.Lock()
	defer c.Unlock()

	clone := map[uint32]MiniBlock{}

	clone_list := make([]MiniBlock, 0, 64)
	for k, v := range c.Collection {
		clone[k] = v
		clone_list = append(clone_list, v)
	}

	for _, mbl := range clone_list {
		if mbl.Genesis {
			continue // genesis tips do no have valid past
		}
		for i := uint8(0); i < mbl.PastCount; i++ {
			delete(clone, mbl.Past[i])
		}
	}

	for _, v := range clone {
		mbls = append(mbls, v)
	}
	mbls = MiniBlocks_SortByDistanceDesc(mbls)

	return
}

// get all the tips from the map, this is atleast O(n)
func (c *MiniBlocksCollection) GetAllTipsAtHeight(height int64) (mbls []MiniBlock) {
	c.Lock()
	defer c.Unlock()

	clone := map[uint32]MiniBlock{}
	var clone_list []MiniBlock
	for k, v := range c.Collection {
		if v.Height == height {
			clone[k] = v
			clone_list = append(clone_list, v)
		}
	}

	for _, mbl := range clone_list {
		if mbl.Genesis {
			continue // genesis tips do no have valid past
		}
		for i := uint8(0); i < mbl.PastCount; i++ {
			delete(clone, mbl.Past[i])
		}
	}

	for _, v := range clone {
		mbls = append(mbls, v)
	}

	mbls = MiniBlocks_SortByDistanceDesc(mbls)
	return
}

// this works in all case
func (c *MiniBlocksCollection) GetGenesisFromMiniBlock(mbl MiniBlock) (genesis []MiniBlock) {
	if mbl.Genesis {
		genesis = append(genesis, mbl)
		return
	}

	if len(mbl.PastMiniBlocks) >= 1 { // we do not need locks as all history is connected
		return GetGenesisFromMiniBlock(mbl)
	}

	c.Lock()
	defer c.Unlock()

	var tmp_genesis []MiniBlock
	for i := uint8(0); i < mbl.PastCount; i++ {
		if pmbl, ok := c.Collection[mbl.Past[i]]; ok {
			tmp_genesis = append(tmp_genesis, GetGenesisFromMiniBlock(pmbl)...)
		} else {
			return
		}
	}
	return MiniBlocks_Unique(tmp_genesis)
}

// this works in all cases, but it may return truncated history,all returns must be checked for connectivity
func (c *MiniBlocksCollection) GetEntireMiniBlockHistory(mbl MiniBlock) (history []MiniBlock) {

	history = make([]MiniBlock, 0, 128)
	if mbl.Genesis {
		history = append(history, mbl)
		return
	}

	if len(mbl.PastMiniBlocks) >= 1 { // we do not need locks as all history is connected
		return GetEntireMiniBlockHistory(mbl)
	}

	c.Lock()
	defer c.Unlock()

	for i := uint8(0); i < mbl.PastCount; i++ {
		if pmbl, ok := c.Collection[mbl.Past[i]]; ok {
			history = append(history, GetEntireMiniBlockHistory(pmbl)...)
		} else {
			return
		}
	}
	history = append(history, mbl) // add self
	unique := MiniBlocks_Unique(history)

	return MiniBlocks_SortByTimeAsc(unique)
}

// gets the genesis from the tips
// this function only works, if the miniblock has been expanded
func GetGenesisFromMiniBlock(mbl MiniBlock) (genesis []MiniBlock) {

	if mbl.Genesis {
		genesis = append(genesis, mbl)
		return
	}

	var queue []MiniBlock
	queue = append(queue, mbl.PastMiniBlocks...)

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:] // Dequeue

		if item.Genesis {
			genesis = append(genesis, item)
		} else {
			queue = append(queue, item.PastMiniBlocks...)
		}
	}

	return
}

// get entire history,its in sorted form
func GetEntireMiniBlockHistory(mbls ...MiniBlock) (history []MiniBlock) {
	queue := make([]MiniBlock, 0, 128)
	queue = append(queue, mbls...)
	history = make([]MiniBlock, 0, 128)
	unique := make([]MiniBlock, 0, 128)

	unique_map := map[crypto.Hash]MiniBlock{}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:] // Dequeue

		if _, ok := unique_map[item.GetHash()]; !ok {
			unique_map[item.GetHash()] = item
			history = append(history, item) //mini blocks might be duplicated
			if !item.Genesis {
				queue = append(queue, item.PastMiniBlocks...)
			}
		}
	}

	for _, v := range unique_map {
		unique = append(unique, v)
	}

	history = MiniBlocks_Unique(history)

	if len(unique) != len(history) {
		panic("result mismatch")
	}

	history = MiniBlocks_SortByTimeAsc(history) // sort on the basis of timestamps

	return
}

// this sorts by distance, in descending order
// if distance is equal, then it sorts by its id which is collision free
func MiniBlocks_SortByDistanceDesc(mbls []MiniBlock) (sorted []MiniBlock) {
	sorted = make([]MiniBlock, 0, len(mbls))
	sorted = append(sorted, mbls...)
	sort.SliceStable(sorted, func(i, j int) bool { // sort descending on the basis of Distance
		if sorted[i].Distance == sorted[j].Distance {
			return sorted[i].GetMiniID() > sorted[j].GetMiniID() // higher mini id first
		}
		return sorted[i].Distance > sorted[j].Distance
	})
	return sorted
}

// this sorts by timestamp,ascending order
// if timestamp is equal, then it sorts by its id which is collision free
func MiniBlocks_SortByTimeAsc(mbls []MiniBlock) (sorted []MiniBlock) {
	sorted = make([]MiniBlock, 0, len(mbls))
	sorted = append(sorted, mbls...)
	sort.SliceStable(sorted, func(i, j int) bool { // sort on the basis of timestamps
		if sorted[i].Timestamp == sorted[j].Timestamp {
			return sorted[i].GetMiniID() < sorted[j].GetMiniID()
		}
		return sorted[i].Timestamp < sorted[j].Timestamp
	})
	return sorted
}

func MiniBlocks_Unique(mbls []MiniBlock) (unique []MiniBlock) {
	unique = make([]MiniBlock, 0, len(mbls))
	unique_map := map[crypto.Hash]MiniBlock{}
	for _, mbl := range mbls {
		unique_map[mbl.GetHash()] = mbl
	}
	for _, v := range unique_map {
		unique = append(unique, v)
	}
	return
}

// will filter the mbls having the specific tips
// this will also remove any blocks which do not refer to base
func MiniBlocks_FilterOnlyGenesis(mbls []MiniBlock, tips []crypto.Hash) (result []MiniBlock) {
	var baselist []MiniBlock
	for _, mbl := range mbls {
		if mbl.Genesis {
			baselist = append(baselist, mbl)
		}
	}

	switch len(tips) {
	case 0:
		panic("atleast 1 tip must be present")
	case 1:
		pid1 := binary.BigEndian.Uint32(tips[0][:])
		return MiniBlocks_Filter(baselist, []uint32{pid1})
	case 2:
		pid1 := binary.BigEndian.Uint32(tips[0][:])
		pid2 := binary.BigEndian.Uint32(tips[1][:])
		return MiniBlocks_Filter(baselist, []uint32{pid1, pid2})
	default:
		panic("only max 2 tips are supported")
	}
}

/*
// this will filter if the blocks have any pids
// this will remove any nonbase blocks
func MiniBlocks_FilterPidsSkipGenesis(mbls []MiniBlock, pids []uint32) (result []MiniBlock) {
	var nongenesislist []MiniBlock
	for _, mbl := range mbls {
		if !mbl.Genesis {
			nongenesislist = append(nongenesislist, mbl)
		}
	}
	return MiniBlocks_Filter(nongenesislist, pids)
}
*/

// this will filter if the blocks have any pids
func MiniBlocks_Filter(mbls []MiniBlock, pids []uint32) (result []MiniBlock) {
	switch len(pids) {
	case 0:
		panic("atleast 1 pid must be present")
	case 1:
		pid1 := pids[0]
		for _, mbl := range mbls {
			if mbl.PastCount == uint8(len(pids)) && mbl.HasPid(pid1) {
				result = append(result, mbl)
			}
		}
	case 2:
		pid1 := pids[0]
		pid2 := pids[1]
		for _, mbl := range mbls {
			if mbl.PastCount == uint8(len(pids)) && mbl.HasPid(pid1) && mbl.HasPid(pid2) {
				result = append(result, mbl)
			}
		}
	default:
		panic("only max 2 tips are supported")

	}

	return
}

// draw out a dot graph
func (c *MiniBlocksCollection) Graph() string {
	w := new(strings.Builder)
	w.WriteString("digraph miniblock_graphs { \n")

	for _, mbl := range c.Collection { // draw all nodes
		color := "green"
		if mbl.Genesis {
			color = "white"
		}

		w.WriteString(fmt.Sprintf("node [ fontsize=12 style=filled ]\n{\n"))
		w.WriteString(fmt.Sprintf("L%08x  [ fillcolor=%s label = \"%08x %d height %d  Odd %+v\"  ];\n", mbl.GetMiniID(), color, mbl.GetMiniID(), 0, mbl.Distance, mbl.Odd))
		w.WriteString(fmt.Sprintf("}\n"))

		if !mbl.Genesis { // render connections
			w.WriteString(fmt.Sprintf("L%08x -> L%08x ;\n", mbl.Past[0], mbl.GetMiniID()))
			if mbl.PastCount == 2 {
				w.WriteString(fmt.Sprintf("L%08x -> L%08x ;\n", mbl.Past[1], mbl.GetMiniID()))
			}
		}
	}

	w.WriteString("}\n")
	return w.String()
}
