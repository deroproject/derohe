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
import "math/big"
import "path/filepath"

import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/cryptography/crypto"

import "github.com/deroproject/graviton"

// though these can be done within a single DB, these are separated for completely clarity purposes
type storage struct {
	Balance_store  *graviton.Store // stores most critical data, only history can be purged, its merkle tree is stored in the block
	Block_tx_store storefs         // stores blocks which can be discarded at any time(only past but keep recent history for rollback)
	Topo_store     storetopofs     // stores topomapping which can only be discarded by punching holes in the start of the file
}

func (s *storage) Initialize(params map[string]interface{}) (err error) {

	current_path := filepath.Join(globals.GetDataDirectory())

	if s.Balance_store, err = graviton.NewDiskStore(filepath.Join(current_path, "balances")); err == nil {
		if err = s.Topo_store.Open(current_path); err == nil {
			s.Block_tx_store.basedir = current_path
			s.Block_tx_store.migrate_old_tx()
		}
	}

	if err != nil {
		logger.Error(err, "Cannot open store")
		return err
	}
	logger.Info("Initialized", "path", current_path)

	return nil
}

func (s *storage) IsBalancesIntialized() bool {
	var err error
	var balancehash, random_hash [32]byte

	balance_ss, _ := s.Balance_store.LoadSnapshot(0) // load most recent snapshot
	balancetree, _ := balance_ss.GetTree(config.BALANCE_TREE)

	// avoid hardcoding any hash
	if balancehash, err = balancetree.Hash(); err == nil {
		random_tree, _ := balance_ss.GetTree(config.SC_META)
		if random_hash, err = random_tree.Hash(); err == nil {
			if random_hash == balancehash {
				return false
			}
		}
	}

	if err != nil {
		panic("database issues")
	}
	return true
}

func (chain *Blockchain) StoreBlock(bl *block.Block, snapshot_version uint64) {
	hash := bl.GetHash()
	serialized_bytes := bl.Serialize() // we are storing the miner transactions within

	difficulty_of_current_block := new(big.Int)

	if len(bl.Tips) == 0 { // genesis block has no parent
		difficulty_of_current_block.SetUint64(1) // this is never used, as genesis block is a sync block, only its cumulative difficulty is used
	} else {
		difficulty_of_current_block = chain.Get_Difficulty_At_Tips(bl.Tips)
	}

	chain.Store.Block_tx_store.DeleteBlock(hash) // what should we do on error

	err := chain.Store.Block_tx_store.WriteBlock(hash, serialized_bytes, difficulty_of_current_block, snapshot_version, bl.Height)
	if err != nil {
		panic(fmt.Sprintf("error while writing block"))
	}
}

// loads a block from disk, deserializes it
func (chain *Blockchain) Load_BL_FROM_ID(hash [32]byte) (*block.Block, error) {
	var bl block.Block

	if block_data, err := chain.Store.Block_tx_store.ReadBlock(hash); err == nil {

		if err = bl.Deserialize(block_data); err != nil { // we should deserialize the block here
			//logger.Warnf("fError deserialiing block, block id %x len(data) %d data %x err %s", hash[:], len(block_data), block_data, err)
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
	if _, err := chain.Load_BL_FROM_ID(h); err == nil {
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
			past_height := chain.Load_Block_Height(tips[i])
			if past_height < 0 {
				panic(fmt.Errorf("could not find height for blid %s", tips[i]))
			}
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

	if heighti, err := chain.ReadBlockHeight(h); err != nil {
		return -1
	} else {
		return int64(heighti)
	}
}
func (chain *Blockchain) Load_Height_for_BL_ID(h crypto.Hash) int64 {
	return chain.Load_Block_Height(h)
}

// all the immediate past of a block
func (chain *Blockchain) Get_Block_Past(hash crypto.Hash) (blocks []crypto.Hash) {

	//fmt.Printf("loading tips for block %x\n", hash)
	if keysi, ok := chain.cache_BlockPast.Get(hash); ok {
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

	if chain.cache_enabled { //set in cache
		chain.cache_BlockPast.Add(hash, cache_copy)
	}

	return
}

func (chain *Blockchain) Load_Block_Difficulty(h crypto.Hash) *big.Int {
	if diff, err := chain.Store.Block_tx_store.ReadBlockDifficulty(h); err != nil {
		panic(err)
	} else {
		return diff
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

// load store hash from 2 tree
func (chain *Blockchain) Load_Merkle_Hash(version uint64) (hash crypto.Hash, err error) {

	if hashi, ok := chain.cache_VersionMerkle.Get(version); ok {
		hash = hashi.(crypto.Hash)
		return
	}

	ss, err := chain.Store.Balance_store.LoadSnapshot(version)
	if err != nil {
		return
	}

	balance_tree, err := ss.GetTree(config.BALANCE_TREE)
	if err != nil {
		return
	}
	sc_meta_tree, err := ss.GetTree(config.SC_META)
	if err != nil {
		return
	}
	balance_merkle_hash, err := balance_tree.Hash()
	if err != nil {
		return
	}
	meta_merkle_hash, err := sc_meta_tree.Hash()
	if err != nil {
		return
	}
	for i := range balance_merkle_hash {
		hash[i] = balance_merkle_hash[i] ^ meta_merkle_hash[i]
	}

	if chain.cache_enabled { //set in cache
		chain.cache_VersionMerkle.Add(version, hash)
	}
	return hash, nil
}

// loads a complete block from disk
func (chain *Blockchain) Load_Complete_Block(blid crypto.Hash) (cbl *block.Complete_Block, err error) {
	cbl = &block.Complete_Block{}
	cbl.Bl, err = chain.Load_BL_FROM_ID(blid)
	if err != nil {
		return
	}

	for _, txid := range cbl.Bl.Tx_hashes {
		var tx_bytes []byte
		if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(txid); err != nil {
			return
		} else {
			var tx transaction.Transaction
			if err = tx.Deserialize(tx_bytes); err != nil {
				return
			}
			cbl.Txs = append(cbl.Txs, &tx)
		}

	}
	return
}
