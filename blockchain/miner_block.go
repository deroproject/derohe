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
//  if heights are equal, nodes are sorted by their block ids which will never collide , hopefullly
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

//NOTE: this function is quite big since we do a lot of things in preparation of next blocks
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
	if mbls := chain.MiniBlocks.GetAllTipsAtHeight(chain.Get_Height() + 1); len(mbls) > 0 {

		mbls = block.MiniBlocks_SortByDistanceDesc(mbls)
		for _, mbl := range mbls {
			tips = tips[:0]
			gens := block.GetGenesisFromMiniBlock(mbl)
			if len(gens) <= 0 { // if tip cannot be resolved to genesis skip it
				continue
			}

			var tip crypto.Hash
			copy(tip[:], gens[0].Check[8:8+12])
			if ehash, ok := chain.ExpandMiniBlockTip(tip); ok {
				tips = append(tips, ehash)
			} else {
				continue
			}

			if gens[0].PastCount == 2 {
				copy(tip[:], gens[0].Check[8+12:])
				if ehash, ok := chain.ExpandMiniBlockTip(tip); ok {
					tips = append(tips, ehash)
				} else {
					continue
				}
			}
			break
		}
	}

	if len(tips) == 0 {
		tips = chain.SortTips(chain.Get_TIPS())
	}

	for i := range tips {
		if len(bl.Tips) < 2 { //only 2 tips max
			var check_tips []crypto.Hash
			check_tips = append(check_tips, bl.Tips...)
			check_tips = append(check_tips, tips[i])

			if chain.CheckDagStructure(check_tips) { // avoid any tips which fail structure test
				bl.Tips = append(bl.Tips, tips[i])
			}
		}

	}

	height := chain.Calculate_Height_At_Tips(bl.Tips) // we are 1 higher than previous highest tip

	history := map[crypto.Hash]bool{}

	var history_array []crypto.Hash
	for i := range bl.Tips {
		history_array = append(history_array, chain.get_ordered_past(bl.Tips[i], 26)...)
	}
	for _, h := range history_array {
		history[h] = true
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

	for i := range tx_hash_list_sorted {
		if (sizeoftxs + tx_hash_list_sorted[i].Size) > (99*config.STARGATE_HE_MAX_BLOCK_SIZE)/100 { // limit block to max possible
			break
		}

		if tx := chain.Mempool.Mempool_Get_TX(tx_hash_list_sorted[i].Hash); tx != nil {
			if int64(tx.Height) < height {
				if history[tx.BLID] != true {
					logger.V(8).Info("not selecting tx since the reference with which it was made is not in history", "txid", tx_hash_list_sorted[i].Hash)
					continue
				}
				if height-int64(tx.Height) < TX_VALIDITY_HEIGHT {
					if nil == chain.Verify_Transaction_NonCoinbase_CheckNonce_Tips(hf_version, tx, bl.Tips) {
						if nil == pre_check.check(tx, false) {
							pre_check.check(tx, true)
							sizeoftxs += tx_hash_list_sorted[i].Size
							cbl.Txs = append(cbl.Txs, tx)
							tx_hash_list_included = append(tx_hash_list_included, tx_hash_list_sorted[i].Hash)
							logger.V(8).Info("tx selected for mining ", "txlist", tx_hash_list_sorted[i].Hash)
						} else {
							logger.V(8).Info("not selecting tx due to pre_check failure", "txid", tx_hash_list_sorted[i].Hash)
						}
					} else {
						logger.V(8).Info("not selecting tx due to nonce failure", "txid", tx_hash_list_sorted[i].Hash)
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
	if mbls := chain.MiniBlocks.GetAllTipsAtHeight(height); len(mbls) > 0 {

		mbls = block.MiniBlocks_SortByDistanceDesc(mbls)
		max_distance := uint32(0)
		tipcount := 0
		for _, mbl := range mbls {
			if tipcount == 2 { //we can only support max 2 tips
				break
			}

			gens := block.GetGenesisFromMiniBlock(mbl)
			if len(gens) <= 0 { // if tip cannot be resolved to genesis skip it
				continue
			}
			gens_filtered := block.MiniBlocks_FilterOnlyGenesis(gens, bl.Tips)
			if len(gens_filtered) <= 0 { // no valid genesis having same tips
				continue
			}

			if len(gens) != len(gens_filtered) { // more than 1 genesis, with some not pointing to same tips
				continue
			}

			if max_distance < mbl.Distance {
				max_distance = mbl.Distance
			}

			if mbl.Genesis && max_distance-mbl.Distance > miniblock_genesis_distance { // only 0 distance is supported for genesis
				continue
			}
			if !mbl.Genesis && max_distance-mbl.Distance > miniblock_normal_distance { // only 3 distance is supported
				continue
			}

			history := block.GetEntireMiniBlockHistory(mbl)
			if !mbl.Genesis && len(history) < 2 {
				logger.V(1).Error(nil, "history missing. this should never occur", "mbl", fmt.Sprintf("%+v", mbl))
				continue
			}

			bl.MiniBlocks = append(bl.MiniBlocks, history...)
			tipcount++
		}

		if len(bl.MiniBlocks) > 1 { // we need to unique and sort them by time
			bl.MiniBlocks = block.MiniBlocks_SortByTimeAsc(block.MiniBlocks_Unique(bl.MiniBlocks))
		}

	}

	cbl.Bl = &bl

	return
}

//
func ConvertBlockToMiniblock(bl block.Block, miniblock_miner_address rpc.Address) (mbl block.MiniBlock) {
	mbl.Version = 1

	if len(bl.Tips) == 0 {
		panic("Tips cannot be zero")
	}

	mbl.Timestamp = uint64(globals.Time().UTC().UnixMilli())

	if len(bl.MiniBlocks) == 0 {
		mbl.Genesis = true
		mbl.PastCount = byte(len(bl.Tips))
		for i := range bl.Tips {
			mbl.Past[i] = binary.BigEndian.Uint32(bl.Tips[i][:])
		}
	} else {
		tmp_collection := block.CreateMiniBlockCollection()
		for _, tmbl := range bl.MiniBlocks {
			if err, ok := tmp_collection.InsertMiniBlock(tmbl); !ok {
				logger.Error(err, "error converting block to miniblock")
				panic("not possible, logical flaw")
			}
		}

		tips := tmp_collection.GetAllTips()
		if len(tips) > 2 || len(tips) == 0 {
			logger.Error(nil, "block contains miniblocks for more tips than possible", "count", len(tips))
			panic("not possible, logical flaw")
		}
		for i, tip := range tips {
			mbl.PastCount++
			tiphash := tip.GetHash()
			mbl.Past[i] = binary.BigEndian.Uint32(tiphash[:])
			if tip.Timestamp >= uint64(globals.Time().UTC().UnixMilli()) {
				mbl.Timestamp = tip.Timestamp + 1
			}
		}
	}

	if mbl.Genesis {
		binary.BigEndian.PutUint64(mbl.Check[:], bl.Height)
		copy(mbl.Check[8:], bl.Tips[0][0:12])
		if len(bl.Tips) == 2 {
			copy(mbl.Check[8+12:], bl.Tips[1][0:12])
		}
	} else {
		txshash := bl.GetTXSHash()
		block_header_hash := bl.GetHashWithoutMiniBlocks()
		for i := range mbl.Check {
			mbl.Check[i] = txshash[i] ^ block_header_hash[i]
		}
	}

	miner_address_hashed_key := graviton.Sum(miniblock_miner_address.Compressed())
	copy(mbl.KeyHash[:], miner_address_hashed_key[:])

	globals.Global_Random.Read(mbl.Nonce[:]) // fill with randomness
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
	if !chain.IsAddressHashValid(false, miner_hash) {
		logger.V(3).Error(err, "unregistered miner %s", miner_hash)
		err = fmt.Errorf("unregistered miner or you need to wait 15 mins")
		return
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
	if globals.Arguments["--sync-node"].(bool) {
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

	//fmt.Printf("received miniblock %x block %x\n", miniblock_blob, bl.Serialize())

	// lets try to check pow to detect whether the miner is cheating
	if !chain.VerifyMiniblockPoW(&bl, mbl) {
		logger.V(1).Error(err, "Error ErrInvalidPoW ")
		err = errormsg.ErrInvalidPoW
		return
	}

	var miner_hash crypto.Hash
	copy(miner_hash[:], mbl.KeyHash[:])
	if !chain.IsAddressHashValid(true, miner_hash) {
		logger.V(3).Error(err, "unregistered miner %s", miner_hash)
		err = fmt.Errorf("unregistered miner or you need to wait 15 mins")
		return
	}

	// if we reach here, everything looks ok
	bl.MiniBlocks = append(bl.MiniBlocks, mbl)

	if err = chain.Verify_MiniBlocks(bl); err != nil {

		fmt.Printf("verifying miniblocks %s\n", err)
		return
	}

	mblid = mbl.GetHash()
	if err1, ok := chain.InsertMiniBlock(mbl); ok {
		//fmt.Printf("miniblock %s inserted successfully, total %d\n",mblid,len(chain.MiniBlocks.Collection) )
		result = true

	} else {
		logger.V(1).Error(err1, "miniblock insertion failed", "mbl", fmt.Sprintf("%+v", mbl))
		err = err1
		return
	}

	cache_block_mutex.Lock()
	cache_block.Timestamp = 0 // expire cache block
	cache_block_mutex.Unlock()

	// notify peers, we have a miniblock and return to miner
	if !chain.simulator { // if not in simulator mode, relay miniblock to the chain
		var mbls []block.MiniBlock

		if !mbl.Genesis {
			for i := uint8(0); i < mbl.PastCount; i++ {
				mbls = append(mbls, chain.MiniBlocks.Get(mbl.Past[i]))
			}

		}
		mbls = append(mbls, mbl)
		go chain.P2P_MiniBlock_Relayer(mbls, 0)

	}

	// if a duplicate block is being sent, reject the block
	if _, ok := duplicate_height_check[bl.Height]; ok {
		logger.V(1).Error(nil, "Block %s rejected by chain due to duplicate hwork.", "blid", bl.GetHash())
		err = fmt.Errorf("Error duplicate work")
		return
	}

	// fast check dynamic consensus rules
	// if it passes then this miniblock completes the puzzle (if other consensus rules allow)
	if scraperr := chain.Check_Dynamism(bl.MiniBlocks); scraperr != nil {
		logger.V(3).Error(scraperr, "dynamism check failed ")
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
		logger.Info("Block rejected by chain.", "blid", bl.GetHash())
		return
	}

	blid = bl.GetHash()

	var result_block bool
	err, result_block = chain.Add_Complete_Block(cbl)

	if result_block {

		duplicate_height_check[bl.Height] = true

		logger.V(1).Info("Block successfully accepted, Notifying Network", "blid", bl.GetHash(), "height", bl.Height)

		if !chain.simulator { // if not in simulator mode, relay block to the chain
			chain.P2P_Block_Relayer(cbl, 0) // lets relay the block to network
		}
	} else {
		logger.V(1).Error(err, "Block Rejected", "blid", bl.GetHash())
		return
	}
	return
}

// this expands the 12 byte tip to full 32 byte tip
// it is not used in consensus but used by p2p for safety checks
func (chain *Blockchain) ExpandMiniBlockTip(hash crypto.Hash) (result crypto.Hash, found bool) {

	tips := chain.Get_TIPS()
	for i := range tips {
		if bytes.Equal(hash[:12], tips[i][:12]) {
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
			if bytes.Equal(hash[:12], blhash[:12]) {
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
		if !chain.cache_disabled {
			chain.cache_IsAddressHashValid.Add(fmt.Sprintf("%s", hash), true) // set in cache
		}
	}

	return true
}
