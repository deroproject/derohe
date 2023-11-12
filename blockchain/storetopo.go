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

package blockchain

import "os"
import "fmt"
import "math"
import "path/filepath"
import "encoding/binary"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/crypto"

type TopoRecord struct {
	BLOCK_ID      [32]byte
	State_Version uint64
	Height        int64
}

const TOPORECORD_SIZE int64 = 48

// this file implements a filesystem store which is used to store topo to block mapping directly in the file system and the state version directly tied
type storetopofs struct {
	topomapping        *os.File
	last_state_version uint64
}

func (s TopoRecord) String() string {
	return fmt.Sprintf("blid %x state version %d height %d", s.BLOCK_ID[:], s.State_Version, s.Height)
}

func (s *storetopofs) Open(basedir string) (err error) {
	s.topomapping, err = os.OpenFile(filepath.Join(basedir, "topo.map"), os.O_RDWR|os.O_CREATE, 0700)
	return err
}

func (s *storetopofs) Count() int64 {
	fstat, err := s.topomapping.Stat()
	if err != nil {
		panic(fmt.Sprintf("cannot stat topofile. err %s", err))
	}
	count := int64(fstat.Size() / int64(TOPORECORD_SIZE))
	for ; count >= 1; count-- {
		if record, err := s.Read(count - 1); err == nil && !record.IsClean() {
			break
		} else if err != nil {
			panic(fmt.Sprintf("cannot read topofile. err %s", err))
		}
	}
	return count
}

// it basically represents Load_Block_Topological_order_at_index
// reads an entry at specific location
func (s *storetopofs) Read(index int64) (TopoRecord, error) {
	var buf [TOPORECORD_SIZE]byte
	var record TopoRecord

	if n, err := s.topomapping.ReadAt(buf[:], index*TOPORECORD_SIZE); int64(n) != TOPORECORD_SIZE {
		logger.V(4).Info("cannot read topo record", "index", index, "err", err)
		return record, fmt.Errorf("cannot read topo record at index %d", index)
	}

	copy(record.BLOCK_ID[:], buf[:])
	record.State_Version = binary.LittleEndian.Uint64(buf[len(record.BLOCK_ID):])
	record.Height = int64(binary.LittleEndian.Uint64(buf[len(record.BLOCK_ID)+8:]))

	return record, nil
}

func (s *storetopofs) Write(index int64, blid [32]byte, state_version uint64, height int64) (err error) {
	var buf [TOPORECORD_SIZE]byte
	var record TopoRecord
	var zero_hash [32]byte

	copy(buf[:], blid[:])
	binary.LittleEndian.PutUint64(buf[len(record.BLOCK_ID):], state_version)

	//height := chain.Load_Height_for_BL_ID(blid)
	binary.LittleEndian.PutUint64(buf[len(record.BLOCK_ID)+8:], uint64(height))

	_, err = s.topomapping.WriteAt(buf[:], index*TOPORECORD_SIZE)
	if s.last_state_version != state_version || state_version == 0 {
		if blid != zero_hash { // during fast sync avoid syncing overhead
			s.topomapping.Sync() // looks like this is the cause of corruption
		}
	}
	s.last_state_version = state_version

	return err
}
func (s *storetopofs) Sync() {
	s.topomapping.Sync()
}

func (s *storetopofs) Clean(index int64) (err error) {
	var state_version uint64
	var blid [32]byte
	return s.Write(index, blid, state_version, 0)
}

// whether record is clean
func (r *TopoRecord) IsClean() bool {
	if r.State_Version != 0 {
		return false
	}
	for _, x := range r.BLOCK_ID {
		if x != 0 {
			return false
		}
	}
	return true
}

var pruned_till int64 = -1

// locates prune topoheight till where the history has been pruned
// this is not used anywhere in the consensus and can be modified any way possible
// this is for the wallet
func (s *storetopofs) LocatePruneTopo() int64 {
	if pruned_till >= 0 { // return cached result
		return pruned_till
	}
	count := s.Count()
	if count < 10 {
		return 0
	}
	zero_block, err := s.Read(0)
	if err != nil || zero_block.IsClean() {
		return 0
	}

	fifth_block, err := s.Read(5)
	if err != nil || fifth_block.IsClean() {
		return 0
	}
	// we are assumming atleast 5 blocks are pruned
	if zero_block.State_Version != fifth_block.State_Version {
		return 0
	}

	// now we must find the point  where version number =  zero_block.State_Version + 1

	low := int64(0) // in case of purging DB, this should start from N
	high := int64(count)

	prune_topo := int64(math.MaxInt64)
	for low <= high {
		median := (low + high) / 2
		median_block, _ := s.Read(median)
		if median_block.State_Version >= (zero_block.State_Version + 1) {
			if prune_topo > median {
				prune_topo = median
			}
			high = median - 1
		} else {
			low = median + 1
		}
	}

	prune_topo--

	if prune_topo > count {
		panic("invalid prune detected")
	}

	pruned_till = prune_topo
	return prune_topo
}

// exported from chain
func (chain *Blockchain) LocatePruneTopo() int64 {
	return chain.Store.Topo_store.LocatePruneTopo()
}

func (s *storetopofs) binarySearchHeight(targetheight int64) (blids []crypto.Hash, topos []int64) {

	startIndex := int64(0)

	total_records := int64(s.Count())
	endIndex := total_records
	midIndex := total_records / 2

	if endIndex < 0 { // no record
		return
	}

	for startIndex <= endIndex {
		record, _ := s.Read(midIndex)

		if record.Height >= targetheight-((config.STABLE_LIMIT*4)/2) && record.Height <= targetheight+((config.STABLE_LIMIT*4)/2) {
			break
		}

		if record.Height >= targetheight {
			endIndex = midIndex - 1
			midIndex = (startIndex + endIndex) / 2
			continue
		}

		startIndex = midIndex + 1
		midIndex = (startIndex + endIndex) / 2
	}

	for i, count := midIndex, 0; i >= 0 && count < 100; i, count = i-1, count+1 {
		record, _ := s.Read(i)
		if record.Height == targetheight {
			blids = append(blids, record.BLOCK_ID)
			topos = append(topos, i)
		}
	}

	for i, count := midIndex, 0; i < total_records && count < 100; i, count = i+1, count+1 {
		record, _ := s.Read(i)
		if record.Height == targetheight {
			blids = append(blids, record.BLOCK_ID)
			topos = append(topos, i)
		}
	}

	blids, topos = SliceUniqTopoRecord(blids, topos) // unique the record

	return
}

// SliceUniq removes duplicate values in given slice
func SliceUniqTopoRecord(s []crypto.Hash, h []int64) ([]crypto.Hash, []int64) {
	for i := 0; i < len(s); i++ {
		for i2 := i + 1; i2 < len(s); i2++ {
			if s[i] == s[i2] {
				// delete
				s = append(s[:i2], s[i2+1:]...)
				h = append(h[:i2], h[i2+1:]...)
				i2--
			}
		}
	}
	return s, h
}

func (chain *Blockchain) Get_Blocks_At_Height(height int64) []crypto.Hash {
	blids, _ := chain.Store.Topo_store.binarySearchHeight(height)
	return blids
}

// since topological order might mutate, instead of doing cleanup, we double check the pointers
// we first locate the block and its height, then we locate that height, then we traverse  50 blocks up and 50 blocks down

func (chain *Blockchain) Is_Block_Topological_order(blid crypto.Hash) bool {
	bl_height := chain.Load_Height_for_BL_ID(blid)
	blids, _ := chain.Store.Topo_store.binarySearchHeight(bl_height)

	for i := range blids {
		if blids[i] == blid {
			return true
		}
	}

	return false
}

func (chain *Blockchain) Load_Block_Topological_order(blid crypto.Hash) int64 {
	bl_height := chain.Load_Height_for_BL_ID(blid)
	blids, topos := chain.Store.Topo_store.binarySearchHeight(bl_height)

	for i := range blids {
		if blids[i] == blid {
			return topos[i]
		}
	}
	return -1
}

// this function is not used in core
func (chain *Blockchain) Find_Blocks_Height_Range(startheight, stopheight int64) (blids []crypto.Hash) {
	_, topos_start := chain.Store.Topo_store.binarySearchHeight(startheight)

	if stopheight > chain.Get_Height() {
		stopheight = chain.Get_Height()
	}
	_, topos_end := chain.Store.Topo_store.binarySearchHeight(stopheight)

	lowest := topos_start[0]
	for _, t := range topos_start {
		if t < lowest {
			lowest = t
		}
	}

	highest := topos_end[0]
	for _, t := range topos_end {
		if t > highest {
			highest = t
		}
	}

	blid_map := map[crypto.Hash]bool{}
	for i := lowest; i <= highest; i++ {
		if toporecord, err := chain.Store.Topo_store.Read(i); err != nil {
			panic(err)
		} else {
			blid_map[toporecord.BLOCK_ID] = true
		}
	}
	for k := range blid_map {
		blids = append(blids, k)
	}
	return
}
