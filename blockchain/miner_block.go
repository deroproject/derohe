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
import "bytes"
import "sort"
import "sync"
import "runtime/debug"
import "encoding/binary"

import "golang.org/x/xerrors"
import "golang.org/x/time/rate"
import "golang.org/x/crypto/sha3"

// this file creates the blobs which can be used to mine new blocks

import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/rpc"

import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/transaction"

import "github.com/deroproject/graviton"

const TX_VALIDITY_HEIGHT = 11

// structure used to rank/sort  blocks on a number of factors
type BlockScore struct {
	BLID crypto.Hash
	//MiniCount int
	Height int64 // block height
}

// Heighest height is ordered first,  the condition is reverted see eg. at https://golang.org/pkg/sort/#Slice
//
//	if heights are equal, nodes are sorted by their block ids which will never collide , hopefullly
//
// block ids are sorted by lowest byte first diff
func sort_descending_by_height_blid(tips_scores []BlockScore) {
	sort.Slice(tips_scores, func(i, j int) bool {
		if tips_scores[i].Height != tips_scores[j].Height { // if height mismatch use them
			if tips_scores[i].Height > tips_scores[j].Height {
				return true
			} else {
				return false
			}
		} else { // cumulative difficulty is same, we must check minerblocks
			return bytes.Compare(tips_scores[i].BLID[:], tips_scores[j].BLID[:]) == -1
		}
	})
}

func sort_ascending_by_height(tips_scores []BlockScore) {
	sort.Slice(tips_scores, func(i, j int) bool { return tips_scores[i].Height < tips_scores[j].Height })
}

// this will sort the tips based on cumulative difficulty and/or block ids
// the tips will sorted in descending order
func (chain *Blockchain) SortTips(tips []crypto.Hash) (sorted []crypto.Hash) {
	if len(tips) == 0 {
		panic("tips cannot be 0")
	}
	if len(tips) == 1 {
		sorted = []crypto.Hash{tips[0]}
		return
	}

	tips_scores := make([]BlockScore, len(tips), len(tips))
	for i := range tips {
		tips_scores[i].BLID = tips[i]
		tips_scores[i].Height = chain.Load_Block_Height(tips[i])
	}
	sort_descending_by_height_blid(tips_scores)

	for i := range tips_scores {
		sorted = append(sorted, tips_scores[i].BLID)
	}
	return
}

// used by tip
func convert_uint32_to_crypto_hash(i uint32) crypto.Hash {
	var h crypto.Hash
	binary.BigEndian.PutUint32(h[:], i)
	return h
}

// NOTE: this function is quite big since we do a lot of things in preparation of next blocks
func (chain *Blockchain) Create_new_miner_block(miner_address rpc.Address) (cbl *block.Complete_Block, bl block.Block, err error) {
	//chain.Lock()
	//defer chain.Unlock()

	cbl = &block.Complete_Block{}

	topoheight := chain.Load_TOPO_HEIGHT()
	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	if err != nil {
		return
	}

	ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	if err != nil {
		return
	}

	balance_tree, err := ss.GetTree(config.BALANCE_TREE)
	if err != nil {
		return
	}

	var tips []crypto.Hash
	// lets fill in the tips from miniblocks, list is already sorted
	if keys := chain.MiniBlocks.GetAllKeys(chain.Get_Height() + 1); len(keys) > 0 {

		for _, key := range keys {
			mbls := chain.MiniBlocks.GetAllMiniBlocks(key)
			if len(mbls) < 1 {
				continue
			}
			mbl := mbls[0]
			tips = tips[:0]
			tip := convert_uint32_to_crypto_hash(mbl.Past[0])
			if ehash, ok := chain.ExpandMiniBlockTip(tip); ok {
				tips = append(tips, ehash)
			} else {
				continue
			}

			if mbl.PastCount == 2 {
				tip = convert_uint32_to_crypto_hash(mbl.Past[1])
				if ehash, ok := chain.ExpandMiniBlockTip(tip); ok {
					tips = append(tips, ehash)
				} else {
					continue
				}
			}
			if mbl.PastCount == 2 && mbl.Past[0] == mbl.Past[1] {
				continue
			}
			break
		}

	}

	if len(tips) == 0 {
		tips = chain.SortTips(chain.Get_TIPS())
	}

	for i := range tips {
		if len(bl.Tips) < 1 { //only 1 tip max
			var check_tips []crypto.Hash
			check_tips = append(check_tips, bl.Tips...)
			check_tips = append(check_tips, tips[i])

			if chain.CheckDagStructure(check_tips) { // avoid any tips which fail structure test
				bl.Tips = append(bl.Tips, tips[i])
			}
		}

	}

	height := chain.Calculate_Height_At_Tips(bl.Tips) // we are 1 higher than previous highest tip
	history := map[crypto.Hash]crypto.Hash{}

	var history_array []crypto.Hash
	for i := range bl.Tips {
		h := height - 20
		if h < 0 {
			h = 0
		}
		history_array = append(history_array, chain.get_ordered_past(bl.Tips[i], h)...)
	}
	for _, h := range history_array {

		version, err := chain.ReadBlockSnapshotVersion(h)
		if err != nil {
			panic(err)
		}
		hash, err := chain.Load_Merkle_Hash(version)
		if err != nil {
			panic(err)
		}

		history[h] = hash
	}

	var tx_hash_list_included []crypto.Hash // these tx will be included ( due to  block size limit )

	sizeoftxs := uint64(0) // size of all non coinbase tx included within this block

	// add upto 100 registration tx each registration tx is 99 bytes, so 100 tx will take 9900 bytes or 10KB
	{
		tx_hash_list_sorted := chain.Regpool.Regpool_List_TX() // hash of all tx expected to be included within this block , sorted by fees
		for i := range tx_hash_list_sorted {
			if tx := chain.Regpool.Regpool_Get_TX(tx_hash_list_sorted[i]); tx != nil {
				if _, err = balance_tree.Get(tx.MinerAddress[:]); err != nil {
					if xerrors.Is(err, graviton.ErrNotFound) { // address needs registration
						cbl.Txs = append(cbl.Txs, tx)
						tx_hash_list_included = append(tx_hash_list_included, tx_hash_list_sorted[i])
					}
				}
			}
		}
	}

	hf_version := chain.Get_Current_Version_at_Height(height)
	//rlog.Infof("Total tx in pool %d", len(tx_hash_list_sorted))

	// select tx based on fees
	// first of lets find the tx fees collected by consuming txs from mempool
	tx_hash_list_sorted := chain.Mempool.Mempool_List_TX_SortedInfo() // hash of all tx expected to be included within this block , sorted by fees

	logger.V(8).Info("mempool returned tx list", "tx_list", tx_hash_list_sorted)
	var pre_check cbl_verify // used to verify sanity of new block

	history_tx := map[crypto.Hash]bool{} // used to build history of recent blocks

	for _, h := range history_array {
		var history_bl *block.Block
		if history_bl, err = chain.Load_BL_FROM_ID(h); err != nil {
			return
		}
		for i := range history_bl.Tx_hashes {
			history_tx[history_bl.Tx_hashes[i]] = true
		}
	}

	for i := range tx_hash_list_sorted {
		if (sizeoftxs + tx_hash_list_sorted[i].Size) > (config.STARGATE_HE_MAX_BLOCK_SIZE - 102400) { // limit block to max possible
			break
		}

		if _, ok := history_tx[tx_hash_list_sorted[i].Hash]; ok {
			logger.V(8).Info("not selecting tx since it is already mined", "txid", tx_hash_list_sorted[i].Hash)
			continue
		}

		if tx := chain.Mempool.Mempool_Get_TX(tx_hash_list_sorted[i].Hash); tx != nil {
			if int64(tx.Height) < height {
				if _, ok := history[tx.BLID]; !ok {
					logger.V(8).Info("not selecting tx since the reference with which it was made is not in history", "txid", tx_hash_list_sorted[i].Hash)
					continue
				}

				if tx.IsProofRequired() && len(bl.Tips) == 2 {
					if tx.BLID == bl.Tips[0] || tx.BLID == bl.Tips[1] { // delay txs by a  block if they would collide
						logger.V(8).Info("not selecting tx due to probable collision", "txid", tx_hash_list_sorted[i].Hash)
						continue
					}
				}

				if history[tx.BLID] != tx.Payloads[0].Statement.Roothash {
					//return fmt.Errorf("Tx statement roothash mismatch expected %x actual %x", tx.Payloads[0].Statement.Roothash, hash[:])
					continue
				}

				if height-int64(tx.Height) < TX_VALIDITY_HEIGHT {
					if nil == chain.Verify_Transaction_NonCoinbase_CheckNonce_Tips(hf_version, tx, bl.Tips) {
						if nil == pre_check.check(tx, false) {
							pre_check.check(tx, true)
							sizeoftxs += tx_hash_list_sorted[i].Size
							cbl.Txs = append(cbl.Txs, tx)
							tx_hash_list_included = append(tx_hash_list_included, tx_hash_list_sorted[i].Hash)
							logger.V(1).Info("tx selected for mining ", "txlist", tx_hash_list_sorted[i].Hash)
						} else {
							logger.V(8).Info("not selecting tx due to pre_check failure", "txid", tx_hash_list_sorted[i].Hash)
						}
					} else {
						logger.V(1).Info("not selecting tx due to nonce failure", "txid", tx_hash_list_sorted[i].Hash)
					}
				} else {
					logger.V(8).Info("not selecting tx due to height difference", "txid", tx_hash_list_sorted[i].Hash)
				}
			} else {
				logger.V(8).Info("not selecting tx due to height", "txid", tx_hash_list_sorted[i].Hash)
			}
		} else {
			logger.V(8).Info("not selecting tx  since tx is nil", "txid", tx_hash_list_sorted[i].Hash)
		}
	}

	// now we have all major parts of block, assemble the block
	bl.Major_Version = uint64(chain.Get_Current_Version_at_Height(height))
	bl.Minor_Version = uint64(chain.Get_Ideal_Version_at_Height(height)) // This is used for hard fork voting,
	bl.Height = uint64(height)
	bl.Timestamp = uint64(globals.Time().UTC().UnixMilli())
	bl.Miner_TX.Version = 1
	bl.Miner_TX.TransactionType = transaction.COINBASE // what about unregistered users
	copy(bl.Miner_TX.MinerAddress[:], miner_address.Compressed())

	for i := range bl.Tips { // adjust time stamp, only if someone mined a block in extreme precision
		if chain.Load_Block_Timestamp(bl.Tips[i]) >= uint64(globals.Time().UTC().UnixMilli()) {
			bl.Timestamp = chain.Load_Block_Timestamp(bl.Tips[i]) + 1
		}
	}

	// check whether the miner address is registered
	if _, err = balance_tree.Get(bl.Miner_TX.MinerAddress[:]); err != nil {
		if xerrors.Is(err, graviton.ErrNotFound) { // address needs registration
			err = fmt.Errorf("miner address is not registered")
		}
		return
	}

	for i := range tx_hash_list_included {
		bl.Tx_hashes = append(bl.Tx_hashes, tx_hash_list_included[i])
	}

	// lets fill in the miniblocks, list is already sorted

	var key block.MiniBlockKey
	key.Height = bl.Height
	key.Past0 = binary.BigEndian.Uint32(bl.Tips[0][:])
	if len(bl.Tips) == 2 {
		key.Past1 = binary.BigEndian.Uint32(bl.Tips[1][:])
	}

	if mbls := chain.MiniBlocks.GetAllMiniBlocks(key); len(mbls) > 0 {
		if uint64(len(mbls)) > config.BLOCK_TIME-config.MINIBLOCK_HIGHDIFF {
			mbls = mbls[:config.BLOCK_TIME-config.MINIBLOCK_HIGHDIFF]
		}
		bl.MiniBlocks = mbls
	}

	cbl.Bl = &bl

	return
}

func ConvertBlockToMiniblock(bl block.Block, miniblock_miner_address rpc.Address) (mbl block.MiniBlock) {
	mbl.Version = 1

	if len(bl.Tips) == 0 {
		panic("Tips cannot be zero")
	}
	mbl.Height = bl.Height

	timestamp := uint64(globals.Time().UTC().UnixMilli())
	mbl.Timestamp = uint16(timestamp) // this will help us better understand network conditions

	mbl.PastCount = byte(len(bl.Tips))
	for i := range bl.Tips {
		mbl.Past[i] = binary.BigEndian.Uint32(bl.Tips[i][:])
	}

	if uint64(len(bl.MiniBlocks)) < config.BLOCK_TIME-config.MINIBLOCK_HIGHDIFF {
		miner_address_hashed_key := graviton.Sum(miniblock_miner_address.Compressed())
		copy(mbl.KeyHash[:], miner_address_hashed_key[:])
	} else {
		mbl.Final = true
		mbl.HighDiff = true
		block_header_hash := sha3.Sum256(bl.Serialize()) // note here this block is not present
		for i := range mbl.KeyHash {
			mbl.KeyHash[i] = block_header_hash[i]
		}
	}
	// leave the flags for users as per their request

	for i := range mbl.Nonce {
		mbl.Nonce[i] = globals.Global_Random.Uint32() // fill with randomness
	}

	return
}

// returns a new block template ready for mining
// block template has the following format
// miner block header in hex  +
// miner tx in hex +
// 2 bytes ( inhex 4 bytes for number of tx )
// tx hashes that follow
var cache_block block.Block
var cache_block_mutex sync.Mutex

func (chain *Blockchain) Create_new_block_template_mining(miniblock_miner_address rpc.Address) (bl block.Block, mbl block.MiniBlock, miniblock_blob string, reserved_pos int, err error) {
	cache_block_mutex.Lock()
	defer cache_block_mutex.Unlock()

	if (cache_block.Timestamp+100) < (uint64(globals.Time().UTC().UnixMilli())) || (cache_block.Timestamp > 0 && int64(cache_block.Height) != chain.Get_Height()+1) {
		if chain.simulator {
			_, bl, err = chain.Create_new_miner_block(miniblock_miner_address) // simulator lets you test everything
		} else {
			_, bl, err = chain.Create_new_miner_block(chain.integrator_address)
		}

		if err != nil {
			logger.V(1).Error(err, "block template error ")
			return
		}
		cache_block = bl // setup block cache for 100 msec
		chain.mining_blocks_cache.Add(fmt.Sprintf("%d", cache_block.Timestamp), string(bl.Serialize()))
	} else {
		bl = cache_block
	}

	mbl = ConvertBlockToMiniblock(bl, miniblock_miner_address)

	var miner_hash crypto.Hash
	copy(miner_hash[:], mbl.KeyHash[:])
	if !mbl.Final {

		if !chain.IsAddressHashValid(false, miner_hash) {
			logger.V(3).Error(err, "unregistered miner %s", miner_hash)
			err = fmt.Errorf("unregistered miner or you need to wait 15 mins")
			return
		}
	}

	miniblock_blob = fmt.Sprintf("%x", mbl.Serialize())

	return
}

// rate limiter is deployed, in case RPC is exposed over internet
// someone should not be just giving fake inputs and delay chain syncing
var accept_limiter = rate.NewLimiter(1.0, 4) // 1 block per sec, burst of 4 blocks is okay
var accept_lock = sync.Mutex{}
var duplicate_height_check = map[uint64]bool{}

// accept work given by us
// we should verify that the transaction list supplied back by the miner exists in the mempool
// otherwise the miner is trying to attack the network

func (chain *Blockchain) Accept_new_block(tstamp uint64, miniblock_blob []byte) (mblid crypto.Hash, blid crypto.Hash, result bool, err error) {
	if globals.Arguments["--sync-node"] != nil && globals.Arguments["--sync-node"].(bool) {
		logger.Error(fmt.Errorf("Mining is deactivated since daemon is running with --sync-mode, please check program options."), "")
		return mblid, blid, false, fmt.Errorf("Please deactivate --sync-node option before mining")
	}

	accept_lock.Lock()
	defer accept_lock.Unlock()

	cbl := &block.Complete_Block{}
	bl := block.Block{}
	var mbl block.MiniBlock

	//logger.Infof("Incoming block for accepting %x", block_template)
	// safety so if anything wrong happens, verification fails
	defer func() {
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recovered while accepting new block", "r", r, "stack", debug.Stack())
			err = fmt.Errorf("Error while parsing block")
		}
	}()

	if err = mbl.Deserialize(miniblock_blob); err != nil {
		logger.V(1).Error(err, "Error Deserializing blob")
		return
	}

	// now lets locate the actual block from our cache
	if block_data, found := chain.mining_blocks_cache.Get(fmt.Sprintf("%d", tstamp)); found {
		if err = bl.Deserialize([]byte(block_data.(string))); err != nil {
			logger.V(1).Error(err, "Error parsing submitted work block template ", "template", block_data)
			return
		}
	} else {
		logger.V(1).Error(nil, "Job not found in cache", "jobid", fmt.Sprintf("%d", tstamp), "tstamp", uint64(globals.Time().UTC().UnixMilli()))
		err = fmt.Errorf("job not found in cache")
		return
	}

	// lets try to check pow to detect whether the miner is cheating

	if !chain.simulator && !chain.VerifyMiniblockPoW(&bl, mbl) {
		//logger.V(1).Error(err, "Error ErrInvalidPoW")
		err = errormsg.ErrInvalidPoW
		return
	}

	if !mbl.Final {
		var miner_hash crypto.Hash
		copy(miner_hash[:], mbl.KeyHash[:])
		if !chain.IsAddressHashValid(true, miner_hash) {
			logger.V(3).Error(err, "unregistered miner %s", miner_hash)
			err = fmt.Errorf("unregistered miner or you need to wait 15 mins")
			return
		}

		if err1, ok := chain.InsertMiniBlock(mbl); ok {
			//fmt.Printf("miniblock %s inserted successfully, total %d\n",mblid,len(chain.MiniBlocks.Collection) )
			result = true

			// notify peers, we have a miniblock and return to miner
			if !chain.simulator { // if not in simulator mode, relay miniblock to the chain
				go chain.P2P_MiniBlock_Relayer(mbl, 0)
			}

		} else {
			logger.V(1).Error(err1, "miniblock insertion failed", "mbl", fmt.Sprintf("%+v", mbl))
			err = err1

		}
		return
	}

	// if we reach here, everything looks ok, we can complete the block we have, lets add the final piece
	bl.MiniBlocks = append(bl.MiniBlocks, mbl)

	// if a duplicate block is being sent, reject the block
	if _, ok := duplicate_height_check[bl.Height]; ok {
		logger.V(3).Error(nil, "Block %s rejected by chain due to duplicate hwork.", "blid", bl.GetHash())
		err = fmt.Errorf("Error duplicate work")
		return
	}

	// since we have passed dynamic rules, build a full block and try adding to chain
	// lets build up the complete block

	// collect tx list + their fees

	for i := range bl.Tx_hashes {
		var tx *transaction.Transaction
		var tx_bytes []byte
		if tx = chain.Mempool.Mempool_Get_TX(bl.Tx_hashes[i]); tx != nil {
			cbl.Txs = append(cbl.Txs, tx)
			continue
		} else if tx = chain.Regpool.Regpool_Get_TX(bl.Tx_hashes[i]); tx != nil {
			cbl.Txs = append(cbl.Txs, tx)
			continue
		} else if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(bl.Tx_hashes[i]); err == nil {
			tx = &transaction.Transaction{}
			if err = tx.Deserialize(tx_bytes); err != nil {
				logger.V(1).Error(err, "Tx could not be loaded from disk", "txid", bl.Tx_hashes[i].String())
				return
			}
			cbl.Txs = append(cbl.Txs, tx)
		} else {
			logger.V(1).Error(err, "Tx not found in pool or DB, rejecting submitted block", "txid", bl.Tx_hashes[i].String())
			return
		}
	}

	cbl.Bl = &bl // the block is now complete, lets try to add it to chain

	if !chain.simulator && !accept_limiter.Allow() { // if rate limiter allows, then add block to chain
		logger.V(1).Info("Block rejected by chain", "blid", bl.GetHash())
		return
	}

	blid = bl.GetHash()

	var result_block bool
	err, result_block = chain.Add_Complete_Block(cbl)

	if result_block {
		duplicate_height_check[bl.Height] = true

		cache_block_mutex.Lock()
		cache_block.Timestamp = 0 // expire cache block
		cache_block_mutex.Unlock()

		logger.V(1).Info("Block successfully accepted, Notifying Network", "blid", bl.GetHash(), "height", bl.Height)

		result = true // block's pow is valid

		if !chain.simulator { // if not in simulator mode, relay block to the chain
			go chain.P2P_Block_Relayer(cbl, 0) // lets relay the block to network
		}
	} else {
		logger.V(3).Error(err, "Block Rejected", "blid", bl.GetHash())
		return
	}
	return
}

// this expands the 12 byte tip to full 32 byte tip
// it is not used in consensus but used by p2p for safety checks
func (chain *Blockchain) ExpandMiniBlockTip(hash crypto.Hash) (result crypto.Hash, found bool) {

	tips := chain.Get_TIPS()
	for i := range tips {
		if bytes.Equal(hash[:4], tips[i][:4]) {
			copy(result[:], tips[i][:])
			return result, true
		}
	}

	// the block may just have been mined, so we evaluate roughly 25 past blocks to cross check
	max_topo := chain.Load_TOPO_HEIGHT()
	tries := 0
	for i := max_topo; i >= 0 && tries < 25; i-- {

		blhash, err := chain.Load_Block_Topological_order_at_index(i)
		if err == nil {
			if bytes.Equal(hash[:4], blhash[:4]) {
				copy(result[:], blhash[:])
				return result, true
			}
		}
		tries++
	}

	return result, false
}

// it is USED by consensus and p2p whether the miners has is valid
func (chain *Blockchain) IsAddressHashValid(skip_cache bool, hashes ...crypto.Hash) (found bool) {

	if skip_cache {
		for _, hash := range hashes { // check whether everything could be satisfied via cache
			if _, found := chain.cache_IsAddressHashValid.Get(fmt.Sprintf("%s", hash)); !found {
				goto hard_way // do things the hard way
			}
		}
		return true
	}

hard_way:
	// the block may just have been mined, so we evaluate roughly 25 past blocks to cross check
	max_topo := chain.Load_TOPO_HEIGHT()

	if max_topo > 25 { // we can lag a bit here, basically atleast around 10 mins lag
		max_topo -= 25
	}

	toporecord, err := chain.Store.Topo_store.Read(max_topo)
	if err != nil {
		return
	}

	ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	if err != nil {
		return
	}

	var balance_tree *graviton.Tree
	if balance_tree, err = ss.GetTree(config.BALANCE_TREE); err != nil {
		return
	}

	for _, hash := range hashes {
		bits, _, _, err := balance_tree.GetKeyValueFromHash(hash[0:16])
		if err != nil || bits >= 120 {
			return
		}
		if chain.cache_enabled {
			chain.cache_IsAddressHashValid.Add(fmt.Sprintf("%s", hash), true) // set in cache
		}
	}

	return true
}
