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

import "fmt"
import "sync"
import "math/big"
import "io/ioutil"
import "crypto/rand"
import "path/filepath"
import log "github.com/sirupsen/logrus"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/crypto"

import "github.com/deroproject/graviton"

import "github.com/golang/groupcache/lru"

// note we are keeping the tree name small for disk savings, since they will be stored n times (atleast or archival nodes)
const BALANCE_TREE = "B"

// though these can be done within a single DB, these are separated for completely clarity purposes
type storage struct {
	Balance_store  *graviton.Store // stores most critical data, only history can be purged, its merkle tree is stored in the block
	Block_tx_store storefs         // stores blocks which can be discarded at any time(only past but keep recent history for rollback)
	Topo_store     storetopofs     // stores topomapping which can only be discarded by punching holes in the start of the file
}

var store_logger *log.Entry

func (s *storage) Initialize(params map[string]interface{}) (err error) {
	store_logger = globals.Logger.WithFields(log.Fields{"com": "STORE"})
	current_path := filepath.Join(globals.GetDataDirectory())

	if params["--simulator"] == true {
		if s.Balance_store, err = graviton.NewMemStore(); err == nil {

			current_path, err := ioutil.TempDir("", "dero_simulation")

			if err != nil {
				return err
			}

			if err = s.Topo_store.Open(current_path); err == nil {
				s.Block_tx_store.basedir = current_path
			}
		}

	} else {
		if s.Balance_store, err = graviton.NewDiskStore(filepath.Join(current_path, "balances")); err == nil {
			if err = s.Topo_store.Open(current_path); err == nil {
				s.Block_tx_store.basedir = current_path
			}
		}

	}

	if err != nil {
		store_logger.Fatalf("Cannot open store err %s", err)
	}
	store_logger.Infof("Initialized store at path %s", current_path)

	return nil
}

func (s *storage) IsBalancesIntialized() bool {

	var err error
	var buf [64]byte
	var balancehash, random_hash [32]byte

	balance_ss, _ := s.Balance_store.LoadSnapshot(0) // load most recent snapshot
	balancetree, _ := balance_ss.GetTree(BALANCE_TREE)

	// avoid hardcoding any hash
	if balancehash, err = balancetree.Hash(); err == nil {
		if _, err = rand.Read(buf[:]); err == nil {
			random_tree, _ := balance_ss.GetTree(string(buf[:]))
			if random_hash, err = random_tree.Hash(); err == nil {
				if random_hash == balancehash {
					return false
				}
			}
		}
	}

	if err != nil {
		panic("database issues")
	}
	return true
}

func (chain *Blockchain) StoreBlock(bl *block.Block) {
	hash := bl.GetHash()
	serialized_bytes := bl.Serialize() // we are storing the miner transactions within
	// calculate cumulative difficulty at last block

	if len(bl.Tips) == 0 { // genesis block has no parent
		difficulty_of_current_block := new(big.Int).SetUint64(1) // this is never used, as genesis block is a sync block, only its cumulative difficulty is used
		cumulative_difficulty := new(big.Int).SetUint64(1)       // genesis block cumulative difficulty is 1

		err := chain.Store.Block_tx_store.WriteBlock(hash, serialized_bytes, difficulty_of_current_block, cumulative_difficulty)
		if err != nil {
			panic(fmt.Sprintf("error while writing block"))
		}
	} else {

		difficulty_of_current_block := chain.Get_Difficulty_At_Tips(bl.Tips)

		// NOTE: difficulty must be stored before cumulative difficulty calculation, since it is used while calculating Cdiff

		base, base_height := chain.find_common_base(bl.Tips)

		// this function requires block and difficulty to be pre-saveed
		//work_map, cumulative_difficulty := chain.FindTipWorkScore( hash, base, base_height)

		//this function is a copy of above function, however, it uses memory copies
		work_map, cumulative_difficulty := chain.FindTipWorkScore_duringsave(bl, difficulty_of_current_block, base, base_height)
		_ = work_map

		err := chain.Store.Block_tx_store.WriteBlock(hash, serialized_bytes, difficulty_of_current_block, cumulative_difficulty)
		if err != nil {
			panic(fmt.Sprintf("error while writing block"))
		}

	}

}

// loads a block from disk, deserializes it
func (chain *Blockchain) Load_BL_FROM_ID(hash [32]byte) (*block.Block, error) {
	var bl block.Block

	if block_data, err := chain.Store.Block_tx_store.ReadBlock(hash); err == nil {

		if err = bl.Deserialize(block_data); err != nil { // we should deserialize the block here
			logger.Warnf("fError deserialiing block, block id %x len(data) %d data %x err %s", hash[:], len(block_data), block_data,err)
			return nil, err
		}

		return &bl, nil
	} else {
		return nil, err
	}

	/*else if xerrors.Is(err,graviton.ErrNotFound){
	}*/

}

// confirm whether the block exist in the data
// this only confirms whether the block has been downloaded
// a separate check is required, whether the block is valid ( satifies PoW and other conditions)
// we will not add a block to store, until it satisfies PoW
func (chain *Blockchain) Block_Exists(h crypto.Hash) bool {
	_, err := chain.Load_BL_FROM_ID(h)
	if err == nil {
		return true
	}
	return false
}

// This will get the biggest height of tip for hardfork version and other calculations
// get biggest height of parent, add 1
func (chain *Blockchain) Calculate_Height_At_Tips(tips []crypto.Hash) int64 {
	height := int64(0)
	if len(tips) == 0 { // genesis block has no parent

	} else { // find the best height of past
		for i := range tips {
			bl, err := chain.Load_BL_FROM_ID(tips[i])
			if err != nil {
				panic(err)
			}
			past_height := int64(bl.Height)
			if height <= past_height {
				height = past_height
			}
		}
		height++
	}
	return height

}

func (chain *Blockchain) Load_Block_Timestamp(h crypto.Hash) uint64 {
	bl, err := chain.Load_BL_FROM_ID(h)
	if err != nil {
		panic(err)
	}

	return bl.Timestamp
}

func (chain *Blockchain) Load_Block_Height(h crypto.Hash) (height int64) {

	defer func() {
		if r := recover(); r != nil {
			height = -1
		}
	}()

	bl, err := chain.Load_BL_FROM_ID(h)
	if err != nil {
		panic(err)
	}
	height = int64(bl.Height)

	return
}
func (chain *Blockchain) Load_Height_for_BL_ID(h crypto.Hash) int64 {
	return chain.Load_Block_Height(h)
}

var past_cache = lru.New(10240)
var past_cache_lock sync.Mutex

// all the immediate past of a block
func (chain *Blockchain) Get_Block_Past(hash crypto.Hash) (blocks []crypto.Hash) {

	//fmt.Printf("loading tips for block %x\n", hash)
	past_cache_lock.Lock()
	defer past_cache_lock.Unlock()

	if keysi, ok := past_cache.Get(hash); ok {
		keys := keysi.([]crypto.Hash)
		blocks = make([]crypto.Hash, len(keys))
		for i := range keys {
			copy(blocks[i][:], keys[i][:])
		}
		return
	}

	bl, err := chain.Load_BL_FROM_ID(hash)
	if err != nil {
		panic(err)
	}

	blocks = make([]crypto.Hash, 0, len(bl.Tips))
	for i := range bl.Tips {
		blocks = append(blocks, bl.Tips[i])
	}

	cache_copy := make([]crypto.Hash, len(blocks), len(blocks))
	for i := range blocks {
		cache_copy[i] = blocks[i]
	}

	//set in cache
	past_cache.Add(hash, cache_copy)

	return
}

func (chain *Blockchain) Load_Block_Difficulty(h crypto.Hash) *big.Int {
	if diff, err := chain.Store.Block_tx_store.ReadBlockDifficulty(h); err != nil {
		panic(err)
	} else {
		return diff
	}
}

func (chain *Blockchain) Load_Block_Cumulative_Difficulty(h crypto.Hash) *big.Int {
	if cdiff, err := chain.Store.Block_tx_store.ReadBlockCDifficulty(h); err != nil {
		panic(err)
	} else {
		return cdiff
	}
}

func (chain *Blockchain) Get_Top_ID() crypto.Hash {
	var h crypto.Hash

	topo_count := chain.Store.Topo_store.Count()

	if topo_count == 0 {
		return h
	}

	cindex := topo_count - 1
	for {
		r, err := chain.Store.Topo_store.Read(cindex)
		if err != nil {
			panic(err)
		}
		if !r.IsClean() {
			return r.BLOCK_ID
		}

		if cindex == 0 {
			return h
		}

		cindex--
	}
}

// faster bootstrap
func (chain *Blockchain) Load_TOP_HEIGHT() int64 {
	return chain.Load_Block_Height(chain.Get_Top_ID())
}

func (chain *Blockchain) Load_TOPO_HEIGHT() int64 {

	topo_count := chain.Store.Topo_store.Count()

	if topo_count == 0 {
		return 0
	}

	cindex := topo_count - 1
	for {
		r, err := chain.Store.Topo_store.Read(cindex)
		if err != nil {
			panic(err)
		}
		if !r.IsClean() {
			return cindex
		}

		if cindex == 0 {
			return 0
		}

		cindex--
	}
}

func (chain *Blockchain) Load_Block_Topological_order_at_index(index_pos int64) (hash crypto.Hash, err error) {

	r, err := chain.Store.Topo_store.Read(index_pos)
	if err != nil {
		return hash, err
	}
	if !r.IsClean() {
		return r.BLOCK_ID, nil
	} else {
		panic("cnnot query clean block id")
	}

}
