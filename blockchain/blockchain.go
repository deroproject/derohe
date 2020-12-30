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

// This file runs the core consensus protocol
// please think before randomly editing for after effects
// We must not call any packages that can call panic
// NO Panics or FATALs please

/*
import "os"


*/
import "fmt"
import "sync"
import "time"
import "runtime/debug"
import "bytes"
import "sort"
import "math/big"

//import "runtime"
//import "bufio"
import "golang.org/x/crypto/sha3"
import "github.com/romana/rlog"

import "sync/atomic"

import log "github.com/sirupsen/logrus"

import "github.com/golang/groupcache/lru"
import hashicorp_lru "github.com/hashicorp/golang-lru"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/errormsg"
import "github.com/prometheus/client_golang/prometheus"

//import "github.com/deroproject/derosuite/address"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/blockchain/mempool"
import "github.com/deroproject/derohe/blockchain/regpool"

/*
import "github.com/deroproject/derosuite/emission"

import "github.com/deroproject/derosuite/storage"
import "github.com/deroproject/derosuite/crypto/ringct"


import "github.com/deroproject/derosuite/checkpoints"
import "github.com/deroproject/derosuite/metrics"

import "github.com/deroproject/derosuite/blockchain/inputmaturity"
*/

import "github.com/deroproject/graviton"

// all components requiring access to blockchain must use , this struct to communicate
// this structure must be update while mutex
type Blockchain struct {
	Store       storage     // interface to storage layer
	Height      int64       // chain height is always 1 more than block
	height_seen int64       // height seen on peers
	Top_ID      crypto.Hash // id of the top block

	Tips map[crypto.Hash]crypto.Hash // current tips

	dag_unsettled              map[crypto.Hash]bool // current unsettled dag
	dag_past_unsettled_cache   *lru.Cache
	dag_future_unsettled_cache *lru.Cache
	lrucache_workscore         *lru.Cache
	lrucache_fullorder         *lru.Cache // keeps full order for  tips upto a certain height

	MINING_BLOCK bool // used to pause mining

	Difficulty        uint64           // current cumulative difficulty
	Median_Block_Size uint64           // current median block size
	Mempool           *mempool.Mempool // normal tx pool
	Regpool           *regpool.Regpool // registration pool
	Exit_Event        chan bool        // blockchain is shutting down and we must quit ASAP

	Top_Block_Median_Size uint64 // median block size of current top block
	Top_Block_Base_Reward uint64 // top block base reward

	checkpints_disabled bool // are checkpoints disabled
	simulator           bool // is simulator mode

	P2P_Block_Relayer func(*block.Complete_Block, uint64) // tell p2p to broadcast any block this daemon hash found

	RPC_NotifyNewBlock      *sync.Cond // used to notify rpc that a new block has been found
	RPC_NotifyHeightChanged *sync.Cond // used to notify rpc that  chain height has changed due to addition of block

	sync.RWMutex
}

var logger *log.Entry

//var Exit_Event = make(chan bool) // causes all threads to exit

// All blockchain activity is store in a single

/* do initialisation , setup storage, put genesis block and chain in store
   This is the first component to get up
   Global parameters are picked up  from the config package
*/

func Blockchain_Start(params map[string]interface{}) (*Blockchain, error) {

	var err error
	var chain Blockchain

	_ = err

	logger = globals.Logger.WithFields(log.Fields{"com": "CORE"})
	logger.Infof("Initialising blockchain")
	//init_static_checkpoints()           // init some hard coded checkpoints
	//checkpoints.LoadCheckPoints(logger) // load checkpoints from file if provided

	chain.Store.Initialize(params)
	chain.Tips = map[crypto.Hash]crypto.Hash{}

	//chain.Tips = map[crypto.Hash]crypto.Hash{} // initialize Tips map
	chain.lrucache_workscore = lru.New(8191)  // temporary cache for work caclculation
	chain.lrucache_fullorder = lru.New(20480) // temporary cache for fullorder caclculation

	if globals.Arguments["--disable-checkpoints"] != nil {
		chain.checkpints_disabled = globals.Arguments["--disable-checkpoints"].(bool)
	}

	if params["--simulator"] == true {
		chain.simulator = true // enable simulator mode, this will set hard coded difficulty to 1
	}

	chain.Exit_Event = make(chan bool) // init exit channel

	// init mempool before chain starts
	chain.Mempool, err = mempool.Init_Mempool(params)
	chain.Regpool, err = regpool.Init_Regpool(params)

	chain.RPC_NotifyNewBlock = sync.NewCond(&sync.Mutex{})      // used by dero daemon to notify all websockets that new block has arrived
	chain.RPC_NotifyHeightChanged = sync.NewCond(&sync.Mutex{}) // used by dero daemon to notify all websockets that chain height has changed

	if !chain.Store.IsBalancesIntialized() {
		logger.Debugf("Genesis block not in store, add it now")
		var complete_block block.Complete_Block
		bl := Generate_Genesis_Block()
		complete_block.Bl = &bl

		if err, ok := chain.Add_Complete_Block(&complete_block); !ok {
			logger.Fatalf("Failed to add genesis block, we can no longer continue. err %s", err)
		}
	}
	/*

		// genesis block not in chain, add it to chain, together with its miner tx
		// make sure genesis is in the store

		//if !chain.Block_Exists(globals.Config.Genesis_Block_Hash) {
		if !chain.Block_Exists(nil, bl.GetHash()) {
			//chain.Store_TOP_ID(globals.Config.Genesis_Block_Hash) // store top id , exception of genesis block




			logger.Infof("Added block successfully")

			//chain.store_Block_Settled(bl.GetHash(),true) // genesis block is always settled



			bl_current_hash := bl.GetHash()
			// store total  reward
			//dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, bl_current_hash[:], PLANET_MINERTX_REWARD, bl.Miner_TX.Vout[0].Amount)

			// store base reward
			//dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, bl_current_hash[:], PLANET_BASEREWARD, bl.Miner_TX.Vout[0].Amount)

			// store total generated coins
			// this is hardcoded at initial chain import, keeping original emission schedule
			if globals.IsMainnet(){
					//dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, bl_current_hash[:], PLANET_ALREADY_GENERATED_COINS, config.MAINNET_HARDFORK_1_TOTAL_SUPPLY)
				}else{
					//dbtx.StoreUint64(BLOCKCHAIN_UNIVERSE, GALAXY_BLOCK, bl_current_hash[:], PLANET_ALREADY_GENERATED_COINS, config.TESTNET_HARDFORK_1_TOTAL_SUPPLY)
				}


			//chain.Store_Block_Topological_order(dbtx, bl.GetHash(), 0) // genesis block is the lowest
			//chain.Store_TOPO_HEIGHT(dbtx, 0)                           //
			//chain.Store_TOP_HEIGHT(dbtx, 0)

			//chain.store_TIPS(dbtx, []crypto.Hash{bl.GetHash()})

			//dbtx.Commit()

		}
	*/

	//fmt.Printf("Genesis Block should be present at height 0\n")
	//blocks := chain.Get_Blocks_At_Height(0)
	//  fmt.Printf("blocks at height 0 %+v\n", blocks)

	//  fmt.Printf("Past of  genesis %+v\n", chain.Get_Block_Past(bl.GetHash()))
	//  fmt.Printf("Future of  genesis %+v\n", chain.Get_Block_Future(bl.GetHash()))

	//  fmt.Printf("Future of  zero block  %+v\n", chain.Get_Block_Future(ZERO_HASH))

	// hard forks must be initialized asap
	init_hard_forks(params)

	// load the chain from the disk
	chain.Initialise_Chain_From_DB()

	//   logger.Fatalf("Testing complete quitting")

	go clean_up_valid_cache() // clean up valid cache

	/*  txlist := chain.Mempool.Mempool_List_TX()
	    for i := range txlist {
	       // if fmt.Sprintf("%s", txlist[i]) == "0fe0e7270ba911956e91d9ea099e4d12aa1bce2473d4064e239731bc37acfd86"{
	        logger.Infof("Verifying tx %s %+v", txlist[i], chain.Verify_Transaction_NonCoinbase(chain.Mempool.Mempool_Get_TX(txlist[i])))

	        //}
	        //p2p.Broadcast_Tx(chain.Mempool.Mempool_Get_TX(txlist[i]))
	    }


	if chain.checkpints_disabled {
		logger.Infof("Internal Checkpoints are disabled")
	} else {
		logger.Debugf("Internal Checkpoints are enabled")
	}

	_ = err

	*/

	/*
		// register the metrics with the metrics registry
		metrics.Registry.MustRegister(blockchain_tx_counter)
		metrics.Registry.MustRegister(mempool_tx_counter)
		metrics.Registry.MustRegister(mempool_tx_count)
		metrics.Registry.MustRegister(block_size)
		metrics.Registry.MustRegister(transaction_size)
		metrics.Registry.MustRegister(block_tx_count)
		metrics.Registry.MustRegister(block_processing_time)
	*/
	atomic.AddUint32(&globals.Subsystem_Active, 1) // increment subsystem

	return &chain, nil
}

// this is the only entrypoint for new / old blocks even for genesis block
// this will add the entire block atomically to the chain
// this is the only function which can add blocks to the chain
// this is exported, so ii can be fed new blocks by p2p layer
// genesis block is no different
// TODO: we should stop mining while adding the new block
func (chain *Blockchain) Add_Complete_Block(cbl *block.Complete_Block) (err error, result bool) {

	var block_hash crypto.Hash
	chain.Lock()
	defer chain.Unlock()
	result = false
	height_changed := false

	chain.MINING_BLOCK = true

	processing_start := time.Now()

	//old_top := chain.Load_TOP_ID() // store top as it may change
	defer func() {

		// safety so if anything wrong happens, verification fails
		if r := recover(); r != nil {
			logger.Warnf("Recovered while adding new block, Stack trace below block_hash %s", block_hash)
			logger.Warnf("Stack trace  \n%s", debug.Stack())
			result = false
			err = errormsg.ErrPanic
		}

		chain.MINING_BLOCK = false

		if result == true { // block was successfully added, commit it atomically
			rlog.Infof("Block successfully acceppted by chain %s", block_hash)

			// gracefully try to instrument
			func() {
				defer func() {
					if r := recover(); r != nil {
						rlog.Warnf("Recovered while instrumenting")
						rlog.Warnf("Stack trace \n%s", debug.Stack())

					}
				}()
				blockchain_tx_counter.Add(float64(len(cbl.Bl.Tx_hashes)))
				block_tx_count.Observe(float64(len(cbl.Bl.Tx_hashes)))
				block_processing_time.Observe(float64(time.Now().Sub(processing_start).Round(time.Millisecond) / 1000000))

				// tracks counters for tx_size

				{
					complete_block_size := 0
					for i := 0; i < len(cbl.Txs); i++ {
						tx_size := len(cbl.Txs[i].Serialize())
						complete_block_size += tx_size
						transaction_size.Observe(float64(tx_size))
					}
					block_size.Observe(float64(complete_block_size))
				}
			}()

			// notify everyone who needs to know that a new block is in the chain
			chain.RPC_NotifyNewBlock.L.Lock()
			chain.RPC_NotifyNewBlock.Broadcast()
			chain.RPC_NotifyNewBlock.L.Unlock()

			if height_changed {
				chain.RPC_NotifyHeightChanged.L.Lock()
				chain.RPC_NotifyHeightChanged.Broadcast()
				chain.RPC_NotifyHeightChanged.L.Unlock()
			}

			//dbtx.Sync() // sync the DB to disk after every execution of this function

			//if old_top != chain.Load_TOP_ID() { // if top has changed, discard mining templates and start afresh
			// TODO discard mining templates or something else, if top chnages requires some action

			//}
		} else {
			//dbtx.Rollback() // if block could not be added, rollback all changes to previous block
			rlog.Infof("Block rejected by chain %s err %s", block_hash, err)
		}
	}()

	bl := cbl.Bl // small pointer to block

	// first of all lets do some quick checks
	// before doing extensive checks
	result = false

	block_hash = bl.GetHash()
	block_logger := logger.WithFields(log.Fields{"blid": block_hash})

	// check if block already exist skip it
	if chain.Is_Block_Topological_order(block_hash) {
		block_logger.Debugf("block already in chain skipping it ")
		return errormsg.ErrAlreadyExists, false
	}

	for k := range chain.Tips {
		if block_hash == k {
			block_logger.Debugf("block already in chain skipping it ")
			return errormsg.ErrAlreadyExists, false
		}
	}

	// only 3 tips allowed in block
	if len(bl.Tips) >= 4 {
		rlog.Warnf("More than 3 tips present in block %s rejecting", block_hash)
		return errormsg.ErrPastMissing, false
	}

	// check whether the tips exist in our chain, if not reject
	for i := range bl.Tips { //if !chain.Is_Block_Topological_order(bl.Tips[i]) {
		if !chain.Block_Exists(bl.Tips[i]) { // alt-tips might not have a topo order at this point, so make sure they exist on disk
			rlog.Warnf("Tip  %s  is NOT present in chain current block %s, skipping it till we get a parent", bl.Tips[i], block_hash)
			return errormsg.ErrPastMissing, false
		}
	}

	block_height := chain.Calculate_Height_At_Tips(bl.Tips)

	if block_height == 0 && int64(bl.Height) == block_height && len(bl.Tips) != 0 {
		block_logger.Warnf("Genesis block cannot have tips. len of tips(%d)", len(bl.Tips))
		return errormsg.ErrInvalidBlock, false
	}

	if len(bl.Tips) >= 1 && bl.Height == 0 {
		block_logger.Warnf("Genesis block can only be at height 0. len of tips(%d)", len(bl.Tips))
		return errormsg.ErrInvalidBlock, false
	}

	if block_height != 0 && block_height < chain.Get_Stable_Height() {
		rlog.Warnf("Block %s rejected since it is stale stable height %d  block height %d", bl.GetHash(), chain.Get_Stable_Height(), block_height)
		return errormsg.ErrInvalidBlock, false
	}

	// use checksum to quick jump
	/*
		if chain.checkpints_disabled == false && checkpoints.IsCheckSumKnown(chain.BlockCheckSum(cbl)) {
			rlog.Debugf("Skipping Deep Checks for block %s ", block_hash)
			goto skip_checks
		} else {
			rlog.Debugf("Deep Checks for block %s ", block_hash)
		}
	*/

	// make sure time is NOT into future, we do not have any margin here
	// some OS have trouble syncing with more than 1 sec granularity
	// if clock diff is more than  1 secs, reject the block
	if bl.Timestamp > (uint64(time.Now().UTC().Unix())) {
		block_logger.Warnf("Rejecting Block, timestamp is too much into future, make sure that system clock is correct")
		return errormsg.ErrFutureTimestamp, false
	}

	// verify that the clock is not being run in reverse
	// the block timestamp cannot be less than any of the parents
	for i := range bl.Tips {
		if chain.Load_Block_Timestamp(bl.Tips[i]) > bl.Timestamp {
			block_logger.Warnf("Block timestamp is  less than its parent, rejecting block %x ", bl.Serialize())
			return errormsg.ErrInvalidTimestamp, false
		}
	}

	//logger.Infof("current version %d  height %d", chain.Get_Current_Version_at_Height( 2500), chain.Calculate_Height_At_Tips(dbtx, bl.Tips))
	// check whether the major version ( hard fork) is valid
	if !chain.Check_Block_Version(bl) {
		block_logger.Warnf("Rejecting !! Block has invalid fork version actual %d expected %d", bl.Major_Version, chain.Get_Current_Version_at_Height(chain.Calculate_Height_At_Tips(bl.Tips)))
		return errormsg.ErrInvalidBlock, false
	}

	// verify whether the tips are unreachable from one another
	if !chain.VerifyNonReachability(bl) {
		block_logger.Warnf("Rejecting !! Block has invalid reachability")
		return errormsg.ErrInvalidBlock, false

	}

	// if the block is referencing any past tip too distant into main chain discard now
	// TODO FIXME this need to computed
	for i := range bl.Tips {
		rusty_tip_base_distance := chain.calculate_mainchain_distance(bl.Tips[i])

		// tips of deviation >= 8 will rejected
		if (int64(chain.Get_Height()) - rusty_tip_base_distance) >= config.STABLE_LIMIT {
			block_logger.Warnf("Rusty TIP  mined by ROGUE miner discarding block %s  best height %d deviation %d rusty_tip %d", bl.Tips[i], chain.Get_Height(), (int64(chain.Get_Height()) - rusty_tip_base_distance), rusty_tip_base_distance)
			return errormsg.ErrInvalidBlock, false
		}
	}

	// verify difficulty of tips provided
	if len(bl.Tips) > 1 {
		best_tip := chain.find_best_tip_cumulative_difficulty(bl.Tips)
		for i := range bl.Tips {
			if best_tip != bl.Tips[i] {
				if !chain.validate_tips(best_tip, bl.Tips[i]) { // reference is first
					block_logger.Warnf("Rusty tip mined by ROGUE miner, discarding block")
					return errormsg.ErrInvalidBlock, false
				}
			}
		}
	}

	// check whether the block crosses the size limit
	// block size is calculate by adding all the txs
	// block header/miner tx is excluded, only tx size if calculated
	{
		block_size := 0
		for i := 0; i < len(cbl.Txs); i++ {
			block_size += len(cbl.Txs[i].Serialize())
			if uint64(block_size) >= config.STARGATE_HE_MAX_BLOCK_SIZE {
				block_logger.Warnf("Block is bigger than max permitted, Rejecting it Actual %d MAX %d ", block_size, config.STARGATE_HE_MAX_BLOCK_SIZE)
				return errormsg.ErrInvalidSize, false
			}
		}
	}

	//logger.Infof("pow hash %s height %d", bl.GetPoWHash(), block_height)

	// Verify Blocks Proof-Of-Work
	// check if the PoW is satisfied
	if !chain.VerifyPoW(bl) { // if invalid Pow, reject the bloc
		block_logger.Warnf("Block has invalid PoW, rejecting it %x", bl.Serialize())
		return errormsg.ErrInvalidPoW, false
	}

	{ // miner TX checks are here
		if bl.Height == 0 && !bl.Miner_TX.IsPremine() { // genesis block contain premine tx a
			block_logger.Warnf("Miner tx failed verification for genesis  rejecting ")
			return errormsg.ErrInvalidBlock, false
		}

		if bl.Height != 0 && !bl.Miner_TX.IsCoinbase() { // all blocks except genesis block contain coinbase TX
			block_logger.Warnf("Miner tx failed  it is not coinbase ")
			return errormsg.ErrInvalidBlock, false
		}

		// always check whether the coin base tx is okay
		if bl.Height != 0 && !chain.Verify_Transaction_Coinbase(cbl, &bl.Miner_TX) { // if miner address is not registered give error
			block_logger.Warnf("Miner address is not registered")
			return errormsg.ErrInvalidBlock, false
		}

		// TODO we need to verify address  whether they are valid points on curve or not
	}

	// now we need to verify each and every tx in detail
	// we need to verify each and every tx contained in the block, sanity check everything
	// first of all check, whether all the tx contained in the block, match their hashes
	{
		if len(bl.Tx_hashes) != len(cbl.Txs) {
			block_logger.Warnf("Block says it has %d txs , however complete block contained %d txs", len(bl.Tx_hashes), len(cbl.Txs))
			return errormsg.ErrInvalidBlock, false
		}

		// first check whether the complete block contains any diplicate hashes
		tx_checklist := map[crypto.Hash]bool{}
		for i := 0; i < len(bl.Tx_hashes); i++ {
			tx_checklist[bl.Tx_hashes[i]] = true
		}

		if len(tx_checklist) != len(bl.Tx_hashes) { // block has duplicate tx, reject
			block_logger.Warnf("Block has %d  duplicate txs, reject it", len(bl.Tx_hashes)-len(tx_checklist))
			return errormsg.ErrInvalidBlock, false

		}
		// now lets loop through complete block, matching each tx
		// detecting any duplicates using txid hash
		for i := 0; i < len(cbl.Txs); i++ {
			tx_hash := cbl.Txs[i].GetHash()
			if _, ok := tx_checklist[tx_hash]; !ok {
				// tx is NOT found in map, RED alert reject the block
				block_logger.Warnf("Block says it has tx %s, but complete block does not have it", tx_hash)
				return errormsg.ErrInvalidBlock, false
			}
		}
	}

	// another check, whether the block contains any duplicate registration within the block
	// block wide duplicate input detector
	{
		nonce_map := map[string]bool{}
		for i := 0; i < len(cbl.Txs); i++ {

			if cbl.Txs[i].TransactionType == transaction.REGISTRATION {
				if _, ok := nonce_map[string(cbl.Txs[i].MinerAddress[:])]; ok {
					block_logger.Warnf("Double Registration within block %s", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}
				nonce_map[string(cbl.Txs[i].MinerAddress[:])] = true
			}
		}
	}

	// another check, whether the tx  is build with the latest snapshot of balance tree
	{
		for i := 0; i < len(cbl.Txs); i++ {
			if cbl.Txs[i].TransactionType == transaction.NORMAL {
				if cbl.Txs[i].Height+1 != cbl.Bl.Height {
					block_logger.Warnf("invalid tx mined %s", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}

			}
		}
	}

	// another check, whether the tx contains any duplicate nonces within the block
	// block wide duplicate input detector
	{
		nonce_map := map[crypto.Hash]bool{}
		for i := 0; i < len(cbl.Txs); i++ {

			if cbl.Txs[i].TransactionType == transaction.NORMAL {
				if _, ok := nonce_map[cbl.Txs[i].Proof.Nonce()]; ok {
					block_logger.Warnf("Double Spend attack within block %s", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}
				nonce_map[cbl.Txs[i].Proof.Nonce()] = true
			}
		}
	}

	// we also need to reject if the the immediately reachable history, has spent the nonce
	// both the checks works on the basis of nonces and not on the basis of txhash
	/*
		{
			reachable_nonces := chain.BuildReachabilityNonces(bl)
		for i := 0; i < len(cbl.Txs); i++ { // loop through all the TXs
			   if cbl.Txs[i].TransactionType == transaction.NORMAL {
				if _, ok := reachable_nonces[cbl.Txs[i].Proof.Nonce()]; ok {
					block_logger.Warnf("Double spend attack tx %s is already mined, rejecting ", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}
			}
		}
	}*/

	// we need to anyways verify the TXS since proofs are not covered by checksum
	{
		fail_count := int32(0)
		wg := sync.WaitGroup{}
		wg.Add(len(cbl.Txs)) // add total number of tx as work

		hf_version := chain.Get_Current_Version_at_Height(chain.Calculate_Height_At_Tips(bl.Tips))
		for i := 0; i < len(cbl.Txs); i++ {
			go func(j int) {
				if !chain.Verify_Transaction_NonCoinbase(hf_version, cbl.Txs[j]) { // transaction verification failed
					atomic.AddInt32(&fail_count, 1) // increase fail count by 1
					block_logger.Warnf("Block verification failed rejecting since TX  %s verification failed", cbl.Txs[j].GetHash())
				}
				wg.Done()
			}(i)
		}

		wg.Wait()           // wait for verifications to finish
		if fail_count > 0 { // check the result
			block_logger.Warnf("Block verification failed  rejecting since TX verification failed ")
			return errormsg.ErrInvalidTX, false
		}

	}

	// we are here means everything looks good, proceed and save to chain
	//skip_checks:

	// save all the txs
	// and then save the block
	{ // first lets save all the txs, together with their link to this block as height
		for i := 0; i < len(cbl.Txs); i++ {
			if err = chain.Store.Block_tx_store.WriteTX(bl.Tx_hashes[i], cbl.Txs[i].Serialize()); err != nil {
				panic(err)
			}
		}
	}

	chain.StoreBlock(bl)

	// if the block is on a lower height tip, the block will not increase chain height
	height := chain.Load_Height_for_BL_ID(block_hash)
	if height > chain.Get_Height() || height == 0 { // exception for genesis block
		atomic.StoreInt64(&chain.Height, height)
		//chain.Store_TOP_HEIGHT(dbtx, height)

		height_changed = true
		rlog.Infof("Chain extended new height %d blid %s", chain.Height, block_hash)

	} else {
		rlog.Infof("Chain extended but height is same %d blid %s", chain.Height, block_hash)

	}

	// calculate new set of tips
	// this is done by removing all known tips which are in the past
	// and add this block as tip

	//past := chain.Get_Block_Past( bl.GetHash())

	tips := chain.Get_TIPS()
	tips = tips[:0]
	//chain.Tips[bl.GetHash()] = bl.GetHash() // add this new block as tip

	for k := range chain.Tips {
		for i := range bl.Tips {
			if bl.Tips[i] == k {
				goto skip_tip
			}

		}
		tips = append(tips, k)
	skip_tip:
	}

	tips = append(tips, bl.GetHash()) // add current block as new tip

	// find the biggest tip  in terms of work
	{

		base, base_height := chain.find_common_base(tips)
		best := chain.find_best_tip(tips, base, base_height)

		// we  only generate full order for the biggest tip

		//gbl := Generate_Genesis_Block()
		// full_order := chain.Generate_Full_Order( bl.GetHash(), gbl.GetHash(), 0,0)
		//base_topo_index := chain.Load_Block_Topological_order(gbl.GetHash())

		full_order := chain.Generate_Full_Order(best, base, base_height, 0)
		base_topo_index := chain.Load_Block_Topological_order(base)

		// we will directly use graviton to mov in to history
		rlog.Debugf("Full order %+v base %s base topo pos %d", full_order, base, base_topo_index)

		if len(bl.Tips) == 0 {
			base_topo_index = 0
		}

		// any blocks which have not changed their topo will be skipped using graviton trick
		skip := true
		for i := int64(0); i < int64(len(full_order)); i++ {

			// check whether the new block is at the same position at the last position
			current_topo_block := i + base_topo_index
			previous_topo_block := current_topo_block - 1
			if skip {
				if current_topo_block < chain.Store.Topo_store.Count() {
					toporecord, err := chain.Store.Topo_store.Read(current_topo_block)
					if err != nil {
						panic(err)
					}
					if full_order[i] == toporecord.BLOCK_ID { // skip reprocessing if not required
						continue
					}
				}

				skip = false // if one block processed, process every higher block
			}

			rlog.Debugf("will execute order from %d %s", i, full_order[i])

			// TODO we must run smart contracts and TXs in this order
			// basically client protocol must run here
			// even if the HF has triggered we may still accept, old blocks for some time
			// so hf is detected block-wise and processed as such

			bl_current_hash := full_order[i]
			bl_current, err1 := chain.Load_BL_FROM_ID(bl_current_hash)
			if err1 != nil {
				block_logger.Debugf("Cannot load block %s for client protocol,probably DB corruption", bl_current_hash)
				return errormsg.ErrInvalidBlock, false
			}

			//fmt.Printf("\ni %d bl %+v\n",i, bl_current)

			height_current := chain.Calculate_Height_At_Tips(bl_current.Tips)
			hard_fork_version_current := chain.Get_Current_Version_at_Height(height_current)

			// this version does not require client protocol as of now
			//  run full client protocol and find valid transactions
			//	rlog.Debugf("running client protocol for %s minertx %s  topo %d", bl_current_hash, bl_current.Miner_TX.GetHash(), highest_topo)

			// generate miner TX rewards as per client protocol
			if hard_fork_version_current == 1 {

			}

			var balance_tree *graviton.Tree
			//
			if bl_current.Height == 0 { // if it's genesis block
				if ss, err := chain.Store.Balance_store.LoadSnapshot(0); err != nil {
					panic(err)
				} else if balance_tree, err = ss.GetTree(BALANCE_TREE); err != nil {
					panic(err)
				}
			} else { // we already have a block before us, use it

				record_version := uint64(0)
				if previous_topo_block >= 0 {
					toporecord, err := chain.Store.Topo_store.Read(previous_topo_block)

					if err != nil {
						panic(err)
					}
					record_version = toporecord.State_Version
				}

				ss, err := chain.Store.Balance_store.LoadSnapshot(record_version)
				if err != nil {
					panic(err)
				}

				balance_tree, err = ss.GetTree(BALANCE_TREE)
				if err != nil {
					panic(err)
				}
			}

			fees_collected := uint64(0)

			// side blocks only represent chain strenth , else they are are ignored
			// this means they donot get any reward , 0 reward
			// their transactions are ignored

			//chain.Store.Topo_store.Write(i+base_topo_index, full_order[i],0, int64(bl_current.Height)) // write entry so as sideblock could work
			if !chain.isblock_SideBlock_internal(full_order[i], current_topo_block, int64(bl_current.Height)) {
				for _, txhash := range bl_current.Tx_hashes { // execute all the transactions
					if tx_bytes, err := chain.Store.Block_tx_store.ReadTX(txhash); err != nil {
						panic(err)
					} else {
						var tx transaction.Transaction
						if err = tx.DeserializeHeader(tx_bytes); err != nil {
							panic(err)
						}
						// we have loaded a tx successfully, now lets execute it
						fees_collected += chain.process_transaction(tx, balance_tree)
					}
				}

				chain.process_miner_transaction(bl_current.Miner_TX, bl_current.Height == 0, balance_tree, fees_collected)
			} else {
				rlog.Debugf("this block is a side block   block height %d blid %s ", chain.Load_Block_Height(full_order[i]), full_order[i])

			}

			// we are here, means everything is okay, lets commit the update balance tree
			commit_version, err := graviton.Commit(balance_tree)
			if err != nil {
				panic(err)
			}

			chain.Store.Topo_store.Write(current_topo_block, full_order[i], commit_version, chain.Load_Block_Height(full_order[i]))
			rlog.Debugf("%d %s   topo_index %d  base topo %d", i, full_order[i], current_topo_block, base_topo_index)

			// this tx must be stored, linked with this block

		}

		// set main chain as new topo order
		// we must discard any rusty tips after they go stale
		best_height := int64(chain.Load_Height_for_BL_ID(best))

		new_tips := []crypto.Hash{}
		for i := range tips {
			rusty_tip_base_distance := chain.calculate_mainchain_distance(tips[i])
			// tips of deviation > 6 will be rejected
			if (best_height - rusty_tip_base_distance) < (config.STABLE_LIMIT - 1) {
				new_tips = append(new_tips, tips[i])

			} else { // this should be a rarest event, probably should never occur, until the network is under sever attack
				logger.Warnf("Rusty TIP declared stale %s  best height %d deviation %d rusty_tip %d", tips[i], best_height, (best_height - rusty_tip_base_distance), rusty_tip_base_distance)
				//chain.transaction_scavenger(dbtx, tips[i]) // scavenge tx if possible
				// TODO we must include any TX from the orphan blocks back to the mempool to avoid losing any TX
			}
		}

		// do more cleanup of tips for byzantine behaviour
		// this copy is necessary, otherwise data corruption occurs
		tips = append([]crypto.Hash{}, new_tips...)
		best_tip := chain.find_best_tip_cumulative_difficulty(tips)

		{
			new_tips := map[crypto.Hash]crypto.Hash{}
			new_tips[best_tip] = best_tip
			for i := range tips {
				if best_tip != tips[i] {
					if !chain.validate_tips(best_tip, tips[i]) { // reference is first
						logger.Warnf("Rusty tip %s declaring stale", tips[i])
						//chain.transaction_scavenger(dbtx, tips[i]) // scavenge tx if possible
					} else {
						//new_tips = append(new_tips, tips[i])
						new_tips[tips[i]] = tips[i]
					}
				}
			}

			rlog.Debugf("New tips(after adding %s) %+v", bl.GetHash(), new_tips)

			chain.Tips = new_tips
		}
	}

	//chain.store_TIPS(chain.)

	//chain.Top_ID = block_hash // set new top block id

	// every 200 block print a line
	if chain.Get_Height()%200 == 0 {
		block_logger.Infof("Chain Height %d", chain.Height)
	}

	result = true

	// TODO fix hard fork
	// maintain hard fork votes to keep them SANE
	//chain.Recount_Votes() // does not return anything

	// enable mempool book keeping

	func() {
		if r := recover(); r != nil {
			logger.Warnf("Mempool House Keeping triggered panic height = %d", block_height)
		}

		// discard the transactions from mempool if they are present there
		chain.Mempool.Monitor()

		for i := 0; i < len(cbl.Txs); i++ {
			txid := cbl.Txs[i].GetHash()

			switch cbl.Txs[i].TransactionType {

			case transaction.REGISTRATION:
				if chain.Regpool.Regpool_TX_Exist(txid) {
					rlog.Tracef(1, "Deleting TX from regpool txid=%s", txid)
					chain.Regpool.Regpool_Delete_TX(txid)
					continue
				}

			case transaction.NORMAL:
				if chain.Mempool.Mempool_TX_Exist(txid) {
					rlog.Tracef(1, "Deleting TX from mempool txid=%s", txid)
					chain.Mempool.Mempool_Delete_TX(txid)
					continue
				}

			}

		}

		// give mempool an oppurtunity to clean up tx, but only if they are not mined
		chain.Mempool.HouseKeeping(uint64(block_height))

		// ggive regpool a chance to register
		if ss, err := chain.Store.Balance_store.LoadSnapshot(0); err == nil {
			if balance_tree, err := ss.GetTree(BALANCE_TREE); err == nil {

				chain.Regpool.HouseKeeping(uint64(block_height), func(tx *transaction.Transaction) bool {
					if tx.TransactionType != transaction.REGISTRATION { // tx not registration so delete
						return true
					}

					if _, err := balance_tree.Get(tx.MinerAddress[:]); err != nil { // address already registered
						return true
					}
					return false // account not already registered, so give another chance
				})

			}
		}

	}()

	return // run any handlers necesary to atomically
}

// this function is called to read blockchain state from DB
// It is callable at any point in time

func (chain *Blockchain) Initialise_Chain_From_DB() {
	chain.Lock()
	defer chain.Unlock()

	// find the tips from the chain , first by reaching top height
	// then downgrading to top-10 height
	// then reworking the chain to get the tip
	best_height := chain.Load_TOP_HEIGHT()
	chain.Height = best_height

	chain.Tips = map[crypto.Hash]crypto.Hash{} // reset the map
	// reload top tip from disk
	top := chain.Get_Top_ID()

	chain.Tips[top] = top // we only can load a single tip from db

	// get dag unsettled, it's only possible when we have the tips
	// chain.dag_unsettled = chain.Get_DAG_Unsettled() // directly off the disk

	logger.Infof("Chain Tips  %+v Height %d", chain.Tips, chain.Height)

}

// before shutdown , make sure p2p is confirmed stopped
func (chain *Blockchain) Shutdown() {

	chain.Lock()            // take the lock as chain is no longer in unsafe mode
	close(chain.Exit_Event) // send signal to everyone we are shutting down

	chain.Mempool.Shutdown() // shutdown mempool first
	chain.Regpool.Shutdown() // shutdown regpool first

	logger.Infof("Stopping Blockchain")
	//chain.Store.Shutdown()
	atomic.AddUint32(&globals.Subsystem_Active, ^uint32(0)) // this decrement 1 fom subsystem
}

// get top unstable height
// this is obtained by  getting the highest topo block and getting its height
func (chain *Blockchain) Get_Height() int64 {
	topo_count := chain.Store.Topo_store.Count()
	if topo_count == 0 {
		return 0
	}

	//return atomic.LoadUint64(&chain.Height)
	return chain.Load_TOP_HEIGHT()
}

// get height where chain is now stable
func (chain *Blockchain) Get_Stable_Height() int64 {
	tips := chain.Get_TIPS()
	base, base_height := chain.find_common_base(tips)
	_ = base

	return int64(base_height)
}

// we should be holding lock at this time, atleast read only

func (chain *Blockchain) Get_TIPS() (tips []crypto.Hash) {
	for _, x := range chain.Tips {
		tips = append(tips, x)

	}
	return tips
}

func (chain *Blockchain) Get_Difficulty() uint64 {
	return chain.Get_Difficulty_At_Tips(chain.Get_TIPS()).Uint64()
}

/*
func (chain *Blockchain) Get_Cumulative_Difficulty() uint64 {

	return 0 //chain.Load_Block_Cumulative_Difficulty(chain.Top_ID)
}

func (chain *Blockchain) Get_Median_Block_Size() uint64 { // get current cached median size
	return chain.Median_Block_Size
}
*/
func (chain *Blockchain) Get_Network_HashRate() uint64 {
	return chain.Get_Difficulty() / chain.Get_Current_BlockTime()
}

// this is used to for quick syncs as entire blocks as SHA1,
// entires block can skipped for verification, if checksum matches what the devs have stored
func (chain *Blockchain) BlockCheckSum(cbl *block.Complete_Block) []byte {
	h := sha3.New256()
	h.Write(cbl.Bl.Serialize())
	for i := range cbl.Txs {
		h.Write(cbl.Txs[i].Serialize())
	}
	return h.Sum(nil)
}

// various counters/gauges which track a numer of metrics
// such as number of txs, number of inputs, number of outputs
// mempool total addition, current mempool size
// block processing time etcs

// Try it once more, this time with a help string.
var blockchain_tx_counter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "blockchain_tx_counter",
	Help: "Number of tx mined",
})

var mempool_tx_counter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "mempool_tx_counter",
	Help: "Total number of tx added in mempool",
})
var mempool_tx_count = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "mempool_tx_count",
	Help: "Number of tx in mempool at this point",
})

//  track block size about 2 MB
var block_size = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name:    "block_size_byte",
	Help:    "Block size in byte (complete)",
	Buckets: prometheus.LinearBuckets(0, 102400, 10), // start block size 0, each 1 KB step,  2048 such buckets .
})

//  track transaction size upto 500 KB
var transaction_size = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name:    "tx_size_byte",
	Help:    "TX size in byte",
	Buckets: prometheus.LinearBuckets(0, 10240, 16), // start 0  byte, each 1024 byte,  512 such buckets.
})

//  number of tx per block
var block_tx_count = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name:    "block_tx_count",
	Help:    "Number of TX in the block",
	Buckets: prometheus.LinearBuckets(0, 20, 25), // start 0  byte, each 1024 byte,  1024 such buckets.
})

//
var block_processing_time = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name:    "block_processing_time_ms",
	Help:    "Block processing time milliseconds",
	Buckets: prometheus.LinearBuckets(0, 100, 20), // start 0  ms, each 100 ms,  200 such buckets.
})

// this is the only entrypoint for new txs in the chain
// add a transaction to MEMPOOL,
// verifying everything  means everything possible
// this only change mempool, no DB changes
func (chain *Blockchain) Add_TX_To_Pool(tx *transaction.Transaction) (result bool) {
	var err error
	if tx.IsRegistration() { // registration tx will not go any forward
		// ggive regpool a chance to register
		if ss, err := chain.Store.Balance_store.LoadSnapshot(0); err == nil {
			if balance_tree, err := ss.GetTree(BALANCE_TREE); err == nil {
				if _, err := balance_tree.Get(tx.MinerAddress[:]); err == nil { // address already registered
					return false
				}
			}
		}

		return chain.Regpool.Regpool_Add_TX(tx, 0)
	}

	// track counter for the amount of mempool tx
	defer mempool_tx_count.Set(float64(len(chain.Mempool.Mempool_List_TX())))

	txhash := tx.GetHash()

	// Coin base TX can not come through this path
	if tx.IsCoinbase() {
		logger.WithFields(log.Fields{"txid": txhash}).Warnf("TX rejected  coinbase tx cannot appear in mempool")
		return false
	}

	chain_height := uint64(chain.Get_Height())
	if chain_height > tx.Height {
		rlog.Tracef(2, "TX %s rejected since chain has already progressed", txhash)
		return false
	}

	// quick check without calculating everything whether tx is in pool, if yes we do nothing
	if chain.Mempool.Mempool_TX_Exist(txhash) {
		rlog.Tracef(2, "TX %s rejected Already in MEMPOOL", txhash)
		return false
	}

	// check whether tx is already mined
	if _, err = chain.Store.Block_tx_store.ReadTX(txhash); err == nil {
		rlog.Tracef(2, "TX %s rejected Already mined in some block", txhash)
		return false
	}

	hf_version := chain.Get_Current_Version_at_Height(int64(chain_height))

	// if TX is too big, then it cannot be mined due to fixed block size, reject such TXs here
	// currently, limits are  as per consensus
	if uint64(len(tx.Serialize())) > config.STARGATE_HE_MAX_TX_SIZE {
		logger.WithFields(log.Fields{"txid": txhash}).Warnf("TX rejected  Size %d byte Max possible %d", len(tx.Serialize()), config.STARGATE_HE_MAX_TX_SIZE)
		return false
	}

	// check whether enough fees is provided in the transaction
	calculated_fee := chain.Calculate_TX_fee(hf_version, uint64(len(tx.Serialize())))
	provided_fee := tx.Statement.Fees // get fee from tx

	_ = calculated_fee
	_ = provided_fee

	//logger.WithFields(log.Fields{"txid": txhash}).Warnf("TX fees check disabled  provided fee %d calculated fee %d", provided_fee, calculated_fee)

	/*
		if calculated_fee > provided_fee { // 2 % margin see blockchain.cpp L 2913
			logger.WithFields(log.Fields{"txid": txhash}).Warnf("TX rejected due to low fees  provided fee %d calculated fee %d", provided_fee, calculated_fee)

			rlog.Warnf("TX  %s rejected due to low fees  provided fee %d calculated fee %d", txhash, provided_fee, calculated_fee)
			return false
		}
	*/

	if chain.Verify_Transaction_NonCoinbase(hf_version, tx) && chain.Verify_Transaction_NonCoinbase_DoubleSpend_Check(tx) {
		if chain.Mempool.Mempool_Add_TX(tx, 0) { // new tx come with 0 marker
			rlog.Tracef(2, "Successfully added tx %s to pool", txhash)

			mempool_tx_counter.Inc()
			return true
		} else {
			rlog.Tracef(2, "TX %s rejected by pool", txhash)
			return false
		}
	}

	rlog.Warnf("Incoming TX %s could not be verified", txhash)
	return false

}

// structure used to rank/sort  blocks on a number of factors
type BlockScore struct {
	BLID crypto.Hash
	// Weight uint64
	Height                int64    // block height
	Cumulative_Difficulty *big.Int // used to score blocks on cumulative difficulty
}

// Heighest node weight is ordered first,  the condition is reverted see eg. at https://golang.org/pkg/sort/#Slice
//  if weights are equal, nodes are sorted by their block ids which will never collide , hopefullly
// block ids are sorted by lowest byte first diff
func sort_descending_by_cumulative_difficulty(tips_scores []BlockScore) {

	sort.Slice(tips_scores, func(i, j int) bool {
		if tips_scores[i].Cumulative_Difficulty.Cmp(tips_scores[j].Cumulative_Difficulty) != 0 { // if diffculty mismatch use them

			if tips_scores[i].Cumulative_Difficulty.Cmp(tips_scores[j].Cumulative_Difficulty) > 0 { // if i diff >  j diff
				return true
			} else {
				return false
			}

		} else {
			return bytes.Compare(tips_scores[i].BLID[:], tips_scores[j].BLID[:]) == -1
		}
	})
}

func sort_ascending_by_height(tips_scores []BlockScore) {

	// base is the lowest height
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
		tips_scores[i].Cumulative_Difficulty = chain.Load_Block_Cumulative_Difficulty(tips[i])
	}

	sort_descending_by_cumulative_difficulty(tips_scores)

	for i := range tips_scores {
		sorted = append(sorted, tips_scores[i].BLID)
	}
	return
}

// side blocks are blocks which lost the race the to become part
// of main chain, but there transactions are honoured,
// they are given 67 % reward
// a block is a side block if it satisfies the following condition
// if  block height   is less than or equal to height of past 3*config.STABLE_LIMIT topographical blocks
// this is part of consensus rule
// this is the topoheight of this block itself
func (chain *Blockchain) Isblock_SideBlock(blid crypto.Hash) bool {
	block_topoheight := chain.Load_Block_Topological_order(blid)
	if block_topoheight == 0 {
		return false
	}
	// lower reward for byzantine behaviour
	// for as many block as added
	block_height := chain.Load_Height_for_BL_ID(blid)

	return chain.isblock_SideBlock_internal(blid, block_topoheight, block_height)
}

// todo optimize/ run more checks
func (chain *Blockchain) isblock_SideBlock_internal(blid crypto.Hash, block_topoheight int64, block_height int64) (result bool) {
	if block_topoheight == 0 {
		return false
	}
	counter := int64(0)
	for i := block_topoheight - 1; i >= 0 && counter < 16*config.STABLE_LIMIT; i-- {
		counter++
		toporecord, err := chain.Store.Topo_store.Read(i)
		if err != nil {
			panic("Could not load block from previous order")
		}
		if block_height <= toporecord.Height { // lost race (or byzantine behaviour)
			return true // give only 67 % reward
		}

	}

	return false
}

// this will return the tx combination as valid/invalid
// this is not used as core consensus but reports only to user that his tx though in the blockchain is invalid
// a tx is valid, if it exist in a block which is not a side block
func (chain *Blockchain) IS_TX_Valid(txhash crypto.Hash) (valid_blid crypto.Hash, invalid_blid []crypto.Hash, valid bool) {

	var tx_bytes []byte
	var err error

	if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(txhash); err != nil {
		return
	}

	var tx transaction.Transaction
	if err = tx.DeserializeHeader(tx_bytes); err != nil {
		return
	}

	blids, _ := chain.Store.Topo_store.binarySearchHeight(int64(tx.Height + 1))

	var exist_list []crypto.Hash

	for _, blid := range blids {
		bl, err := chain.Load_BL_FROM_ID(blid)
		if err != nil {
			return
		}

		for _, bltxhash := range bl.Tx_hashes {
			if bltxhash == txhash {
				exist_list = append(exist_list, blid)
				break
			}
		}
	}

	for _, blid := range exist_list {
		if chain.Isblock_SideBlock(blid) {
			invalid_blid = append(invalid_blid, blid)
		} else {
			valid_blid = blid
			valid = true
		}

	}

	return
}

/*


// runs the client protocol which includes the following operations
// if any TX are being duplicate or double-spend ignore them
// mark all the valid transactions as valid
// mark all invalid transactions  as invalid
// calculate total fees based on valid TX
// we need NOT check ranges/ring signatures here, as they have been done already by earlier steps
func (chain *Blockchain) client_protocol(dbtx storage.DBTX, bl *block.Block, blid crypto.Hash, height int64, topoheight int64) (total_fees uint64) {
	// run client protocol for all TXs
	for i := range bl.Tx_hashes {
		tx, err := chain.Load_TX_FROM_ID(dbtx, bl.Tx_hashes[i])
		if err != nil {
			panic(fmt.Errorf("Cannot load  tx for %x err %s ", bl.Tx_hashes[i], err))
		}
		// mark TX found in this block also  for explorer
		chain.store_TX_in_Block(dbtx, blid, bl.Tx_hashes[i])

		// check all key images as double spend, if double-spend detected mark invalid, else consider valid
		if chain.Verify_Transaction_NonCoinbase_DoubleSpend_Check(dbtx, tx) {

			chain.consume_keyimages(dbtx, tx, height) // mark key images as consumed
			total_fees += tx.RctSignature.Get_TX_Fee()

			chain.Store_TX_Height(dbtx, bl.Tx_hashes[i], topoheight) // link the tx with the topo height

			//mark tx found in this block is valid
			chain.mark_TX(dbtx, blid, bl.Tx_hashes[i], true)

		} else { // TX is double spend or reincluded by 2 blocks simultaneously
			rlog.Tracef(1,"Double spend TX is being ignored %s %s", blid, bl.Tx_hashes[i])
			chain.mark_TX(dbtx, blid, bl.Tx_hashes[i], false)
		}
	}

	return total_fees
}

// this undoes everything that is done by client protocol
// NOTE: this will have any effect, only if client protocol has been run on this block earlier
func (chain *Blockchain) client_protocol_reverse(dbtx storage.DBTX, bl *block.Block, blid crypto.Hash) {
	// run client protocol for all TXs
	for i := range bl.Tx_hashes {
		tx, err := chain.Load_TX_FROM_ID(dbtx, bl.Tx_hashes[i])
		if err != nil {
			panic(fmt.Errorf("Cannot load  tx for %x err %s ", bl.Tx_hashes[i], err))
		}
		// only the  valid TX must be revoked
		if chain.IS_TX_Valid(dbtx, blid, bl.Tx_hashes[i]) {
			chain.revoke_keyimages(dbtx, tx) // mark key images as not used

			chain.Store_TX_Height(dbtx, bl.Tx_hashes[i], -1) // unlink the tx with the topo height

			//mark tx found in this block is invalid
			chain.mark_TX(dbtx, blid, bl.Tx_hashes[i], false)

		} else { // TX is double spend or reincluded by 2 blocks simultaneously
			// invalid tx is related
		}
	}

	return
}

// scavanger for transactions from rusty/stale tips to reinsert them into pool
func (chain *Blockchain) transaction_scavenger(dbtx storage.DBTX, blid crypto.Hash) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warnf("Recovered while transaction scavenging, Stack trace below ")
			logger.Warnf("Stack trace  \n%s", debug.Stack())
		}
	}()

	logger.Debugf("scavenging transactions from blid %s", blid)
	reachable_blocks := chain.BuildReachableBlocks(dbtx, []crypto.Hash{blid})
	reachable_blocks[blid] = true // add self
	for k, _ := range reachable_blocks {
		if chain.Is_Block_Orphan(k) {
			bl, err := chain.Load_BL_FROM_ID(dbtx, k)
			if err == nil {
				for i := range bl.Tx_hashes {
					tx, err := chain.Load_TX_FROM_ID(dbtx, bl.Tx_hashes[i])
					if err != nil {
						rlog.Warnf("err while scavenging blid %s  txid %s err %s", k, bl.Tx_hashes[i], err)
					} else {
						// add tx to pool, it will do whatever is necessarry
						chain.Add_TX_To_Pool(tx)
					}
				}
			} else {
				rlog.Warnf("err while scavenging blid %s err %s", k, err)
			}
		}
	}
}

*/
// Finds whether a  block is orphan
// since we donot store any fields, we need to calculate/find the block as orphan
// using an algorithm
// if the block is NOT topo ordered , it is orphan/stale
func (chain *Blockchain) Is_Block_Orphan(hash crypto.Hash) bool {
	return !chain.Is_Block_Topological_order(hash)
}

// this is used to find if a tx is orphan, YES orphan TX
// these can occur during  when they lie only  in a side block
// so the TX becomes orphan ( chances are less may be less that .000001 % but they are there)
// if a tx is not valid in any of the blocks, it has been mined it is orphan
func (chain *Blockchain) Is_TX_Orphan(hash crypto.Hash) (result bool) {
	_, _, result = chain.IS_TX_Valid(hash)
	return !result
}

// verifies whether we are lagging
// return true if we need resync
// returns false if we are good and resync is not required
func (chain *Blockchain) IsLagging(peer_cdiff *big.Int) bool {

	our_diff := new(big.Int).SetInt64(0)

	high_block, err := chain.Load_Block_Topological_order_at_index(chain.Load_TOPO_HEIGHT())
	if err != nil {
		return false
	} else {
		our_diff = chain.Load_Block_Cumulative_Difficulty(high_block)
	}
	rlog.Tracef(2, "P_cdiff %s cdiff %d  our top block %s", peer_cdiff.String(), our_diff.String(), high_block)

	if our_diff.Cmp(peer_cdiff) < 0 {
		return true // peer's cumulative difficulty is more than ours , active resync
	}
	return false
}

// this function will rewind the chain from the topo height one block at a time
// this function also runs the client protocol in reverse and also deletes the block from the storage
func (chain *Blockchain) Rewind_Chain(rewind_count int) (result bool) {
	defer chain.Initialise_Chain_From_DB()

	chain.Lock()
	defer chain.Unlock()

	// we must till we reach a safe point
	// safe point is point where a single block exists at specific height
	// this may lead us to rewinding a it more
	//safe := false

	// TODO we must fix safeness using the stable calculation

	if rewind_count == 0 {
		return
	}

	top_block_topo_index := chain.Load_TOPO_HEIGHT()
	rewinded := int64(0)

	for { // rewind as many as possible
		if top_block_topo_index-rewinded < 1 || rewinded >= int64(rewind_count) {
			break
		}

		rewinded++
	}

	for { // rewinf till we reach a safe point
		r, err := chain.Store.Topo_store.Read(top_block_topo_index - rewinded)
		if err != nil {
			panic(err)
		}

		if chain.IsBlockSyncBlockHeight(r.BLOCK_ID) || r.Height == 1 {
			break
		}

		rewinded++
	}

	for i := int64(0); i != rewinded; i++ {
		chain.Store.Topo_store.Clean(top_block_topo_index - i)
	}

	return true
}

// build reachability graph upto 2*config deeps to answer reachability queries
func (chain *Blockchain) buildReachability_internal(reachmap map[crypto.Hash]bool, blid crypto.Hash, level int) {
	bl, err := chain.Load_BL_FROM_ID(blid)
	if err != nil {
		panic(err)
	}
	past := bl.Tips
	reachmap[blid] = true // add self to reach map

	if level >= int(2*config.STABLE_LIMIT) { // stop recursion must be more than  checks in add complete block
		return
	}
	for i := range past { // if no past == genesis return
		if _, ok := reachmap[past[i]]; !ok { // process a node, only if has not been processed earlier
			chain.buildReachability_internal(reachmap, past[i], level+1)
		}
	}

}

// build reachability graph upto 2*limit  deeps to answer reachability queries
func (chain *Blockchain) buildReachability(blid crypto.Hash) map[crypto.Hash]bool {
	reachmap := map[crypto.Hash]bool{}
	chain.buildReachability_internal(reachmap, blid, 0)
	return reachmap
}

// this is part of consensus rule, 2 tips cannot refer to their common parent
func (chain *Blockchain) VerifyNonReachability(bl *block.Block) bool {
	return chain.verifyNonReachabilitytips(bl.Tips)
}

// this is part of consensus rule, 2 tips cannot refer to their common parent
func (chain *Blockchain) verifyNonReachabilitytips(tips []crypto.Hash) bool {

	reachmaps := make([]map[crypto.Hash]bool, len(tips), len(tips))
	for i := range tips {
		reachmaps[i] = chain.buildReachability(tips[i])
	}

	// bruteforce all reachability combinations, max possible 3x3 = 9 combinations
	for i := range tips {
		for j := range tips {
			if i == j { // avoid self test
				continue
			}

			if _, ok := reachmaps[j][tips[i]]; ok { // if a tip can be referenced as another's past, this is not a tip , probably malicious, discard block
				return false
			}

		}
	}

	return true
}

// used in the difficulty calculation for consensus and while scavenging
func (chain *Blockchain) BuildReachableBlocks(tips []crypto.Hash) map[crypto.Hash]bool {
	reachblocks := map[crypto.Hash]bool{} // contains a list of all reachable blocks
	for i := range tips {
		reachmap := chain.buildReachability(tips[i])
		for k, _ := range reachmap {
			reachblocks[k] = true // build unique block list
		}
	}
	return reachblocks
}

// this is part of consensus rule, reachable blocks cannot have keyimages collision with new blocks
// this is to avoid dishonest miners including dead transactions
//
func (chain *Blockchain) BuildReachabilityNonces(bl *block.Block) map[crypto.Hash]bool {

	nonce_reach_map := map[crypto.Hash]bool{}
	reachblocks := map[crypto.Hash]bool{} // contains a list of all reachable blocks
	for i := range bl.Tips {
		reachmap := chain.buildReachability(bl.Tips[i])
		for k, _ := range reachmap {
			reachblocks[k] = true // build unique block list
		}
	}

	// load all blocks and process their TX as per client protocol
	for blid, _ := range reachblocks {
		bl, err := chain.Load_BL_FROM_ID(blid)
		if err != nil {
			panic(fmt.Errorf("Cannot load  block for %s err %s", blid, err))
		}

		for i := 0; i < len(bl.Tx_hashes); i++ { // load all tx one by one, skipping as per client_protocol

			tx_bytes, err := chain.Store.Block_tx_store.ReadTX(bl.Tx_hashes[i])
			if err != nil {
				panic(fmt.Errorf("Cannot load  tx for %s err %s", bl.Tx_hashes[i], err))
			}

			var tx transaction.Transaction
			if err = tx.DeserializeHeader(tx_bytes); err != nil {
				panic(err)
			}

			// tx has been loaded, now lets get the nonce
			nonce_reach_map[tx.Proof.Nonce()] = true // add element to map for next check
		}
	}
	return nonce_reach_map
}

// sync blocks have the following specific property
// 1) the block is singleton at this height
// basically the condition allow us to confirm weight of future blocks with reference to sync blocks
// these are the one who settle the chain and guarantee it
func (chain *Blockchain) IsBlockSyncBlockHeight(blid crypto.Hash) bool {
	return chain.IsBlockSyncBlockHeightSpecific(blid, chain.Get_Height())
}

func (chain *Blockchain) IsBlockSyncBlockHeightSpecific(blid crypto.Hash, chain_height int64) bool {

	// TODO make sure that block exist
	height := chain.Load_Height_for_BL_ID(blid)
	if height == 0 { // genesis is always a sync block
		return true
	}

	//  top blocks are always considered unstable
	if (height + config.STABLE_LIMIT) > chain_height {
		return false
	}

	// if block is not ordered, it can never be sync block
	if !chain.Is_Block_Topological_order(blid) {
		return false
	}

	blocks := chain.Get_Blocks_At_Height(height)

	if len(blocks) == 0 && height != 0 { // this  should NOT occur
		panic("No block exists at this height, not possible")
	}

	//   if len(blocks) == 1 { //  ideal blockchain case, it is a sync block
	//       return true
	//   }

	// check whether single block exists in the TOPO order index, if no we are NOT a sync block

	// we are here means we have one oor more block
	blocks_in_main_chain := 0
	for i := range blocks {
		if chain.Is_Block_Topological_order(blocks[i]) {
			blocks_in_main_chain++
			if blocks_in_main_chain >= 2 {
				return false
			}
		}
	}

	// we are here if we only have one block in topological order, others are  dumped/rejected blocks

	// collect all blocks of past LIMIT heights
	var preblocks []crypto.Hash
	for i := height - 1; i >= (height-config.STABLE_LIMIT) && i != 0; i-- {
		blocks := chain.Get_Blocks_At_Height(i)
		for j := range blocks { //TODO BUG BUG BUG we need to make sure only main chain blocks are considered
			preblocks = append(preblocks, blocks[j])
		}
	}

	// we need to find a common base to compare them, otherwise comparision is futile  due to duplication
	sync_block_cumulative_difficulty := chain.Load_Block_Cumulative_Difficulty(blid) //+ chain.Load_Block_Difficulty(blid)

	// if any of the blocks  has a cumulative difficulty  more than  sync block, this situation affects  consensus, so mitigate it
	for i := range preblocks {
		cumulative_difficulty := chain.Load_Block_Cumulative_Difficulty(preblocks[i]) // + chain.Load_Block_Difficulty(preblocks[i])

		//if cumulative_difficulty >= sync_block_cumulative_difficulty {
		if cumulative_difficulty.Cmp(sync_block_cumulative_difficulty) >= 0 {
			rlog.Warnf("Mitigating CONSENSUS issue on block %s height %d  child %s cdiff %d sync block cdiff %d", blid, height, preblocks[i], cumulative_difficulty, sync_block_cumulative_difficulty)
			return false
		}

	}

	return true
}

// key is string of blid and appendded chain height
var tipbase_cache, _ = hashicorp_lru.New(10240)

// base of a tip is last known sync point
// weight of bases in mentioned in term of height
// this must not employ any cache
func (chain *Blockchain) FindTipBase(blid crypto.Hash, chain_height int64) (bs BlockScore) {

	// see if cache contains it
	if bsi, ok := tipbase_cache.Get(fmt.Sprintf("%s%d", blid, chain_height)); ok {
		bs = bsi.(BlockScore)
		return bs
	}

	defer func() { // capture return value of bs to cache
		z := bs
		tipbase_cache.Add(fmt.Sprintf("%s%d", blid, chain_height), z)
	}()

	// if we are genesis return genesis block as base

	tips := chain.Get_Block_Past(blid)
	if len(tips) == 0 {
		gbl := Generate_Genesis_Block()
		bs = BlockScore{gbl.GetHash(), 0, nil}
		return
	}

	bases := make([]BlockScore, len(tips), len(tips))

	for i := range tips {
		if chain.IsBlockSyncBlockHeightSpecific(tips[i], chain_height) {
			//	rlog.Tracef(2, "SYNC block %s", tips[i])
			bs = BlockScore{tips[i], chain.Load_Height_for_BL_ID(tips[i]), nil}
			return
		}
		bases[i] = chain.FindTipBase(tips[i], chain_height)
	}

	sort_ascending_by_height(bases)

	//   logger.Infof("return BASE %s",bases[0])
	bs = bases[0]
	return bs
}

// this will find the sum of  work done ( skipping any repetive nodes )
// all the information is privided in unique_map
func (chain *Blockchain) FindTipWorkScore_internal(unique_map map[crypto.Hash]*big.Int, blid crypto.Hash, base crypto.Hash, base_height int64) {

	tips := chain.Get_Block_Past(blid)

	for i := range tips {
		if _, ok := unique_map[tips[i]]; !ok {

			ordered := chain.Is_Block_Topological_order(tips[i])

			if !ordered {
				chain.FindTipWorkScore_internal(unique_map, tips[i], base, base_height) // recursively process any nodes
				//logger.Infof("IBlock is not ordered %s", tips[i])
			} else if ordered && chain.Load_Block_Topological_order(tips[i]) >= chain.Load_Block_Topological_order(base) {
				chain.FindTipWorkScore_internal(unique_map, tips[i], base, base_height) // recursively process any nodes

				//logger.Infof("IBlock ordered %s %d %d", tips[i],chain.Load_Block_Topological_order(tips[i]), chain.Load_Block_Topological_order(base) )
			}
		}
	}

	unique_map[blid] = chain.Load_Block_Difficulty(blid)

}

type cachekey struct {
	blid        crypto.Hash
	base        crypto.Hash
	base_height int64
}

// find the score of the tip  in reference to  a base (NOTE: base is always a sync block otherwise results will be wrong )
func (chain *Blockchain) FindTipWorkScore(blid crypto.Hash, base crypto.Hash, base_height int64) (map[crypto.Hash]*big.Int, *big.Int) {

	//logger.Infof("BASE %s",base)
	if tmp_map_i, ok := chain.lrucache_workscore.Get(cachekey{blid, base, base_height}); ok {
		work_score := tmp_map_i.(map[crypto.Hash]*big.Int)

		map_copy := map[crypto.Hash]*big.Int{}
		score := new(big.Int).SetInt64(0)
		for k, v := range work_score {
			map_copy[k] = v
			score.Add(score, v)
		}
		return map_copy, score
	}

	bl, err := chain.Load_BL_FROM_ID(blid)
	if err != nil {
		panic(fmt.Sprintf("Block NOT found %s", blid))
	}
	unique_map := map[crypto.Hash]*big.Int{}

	for i := range bl.Tips {
		if _, ok := unique_map[bl.Tips[i]]; !ok {
			//if chain.Load_Height_for_BL_ID(bl.Tips[i]) >  base_height {
			//    chain.FindTipWorkScore_internal(unique_map,bl.Tips[i],base,base_height) // recursively process any nodes
			//}

			ordered := chain.Is_Block_Topological_order(bl.Tips[i])
			if !ordered {
				chain.FindTipWorkScore_internal(unique_map, bl.Tips[i], base, base_height) // recursively process any nodes
				//   logger.Infof("Block is not ordered %s", bl.Tips[i])
			} else if ordered && chain.Load_Block_Topological_order(bl.Tips[i]) >= chain.Load_Block_Topological_order(base) {
				chain.FindTipWorkScore_internal(unique_map, bl.Tips[i], base, base_height) // recursively process any nodes

				// logger.Infof("Block ordered %s %d %d", bl.Tips[i],chain.Load_Block_Topological_order(bl.Tips[i]), chain.Load_Block_Topological_order(base) )
			}
		}
	}

	if base != blid {
		unique_map[base] = chain.Load_Block_Cumulative_Difficulty(base)
	}

	unique_map[blid] = chain.Load_Block_Difficulty(blid)
	score := new(big.Int).SetInt64(0)
	for _, v := range unique_map {
		score.Add(score, v)
	}

	//set in cache, save a copy in cache
	{
		map_copy := map[crypto.Hash]*big.Int{}
		for k, v := range unique_map {
			map_copy[k] = v
		}
		chain.lrucache_workscore.Add(cachekey{blid, base, base_height}, map_copy)
	}

	return unique_map, score

}

// find the score of the tip  in reference to  a base (NOTE: base is always a sync block otherwise results will be wrong )
func (chain *Blockchain) FindTipWorkScore_duringsave(bl *block.Block, diff *big.Int, base crypto.Hash, base_height int64) (map[crypto.Hash]*big.Int, *big.Int) {

	blid := bl.GetHash()
	//logger.Infof("BASE %s",base)
	if tmp_map_i, ok := chain.lrucache_workscore.Get(cachekey{blid, base, base_height}); ok {
		work_score := tmp_map_i.(map[crypto.Hash]*big.Int)

		map_copy := map[crypto.Hash]*big.Int{}
		score := new(big.Int).SetInt64(0)
		for k, v := range work_score {
			map_copy[k] = v
			score.Add(score, v)
		}
		return map_copy, score
	}

	unique_map := map[crypto.Hash]*big.Int{}

	for i := range bl.Tips {
		if _, ok := unique_map[bl.Tips[i]]; !ok {
			//if chain.Load_Height_for_BL_ID(bl.Tips[i]) >  base_height {
			//    chain.FindTipWorkScore_internal(unique_map,bl.Tips[i],base,base_height) // recursively process any nodes
			//}

			ordered := chain.Is_Block_Topological_order(bl.Tips[i])
			if !ordered {
				chain.FindTipWorkScore_internal(unique_map, bl.Tips[i], base, base_height) // recursively process any nodes
				//   logger.Infof("Block is not ordered %s", bl.Tips[i])
			} else if ordered && chain.Load_Block_Topological_order(bl.Tips[i]) >= chain.Load_Block_Topological_order(base) {
				chain.FindTipWorkScore_internal(unique_map, bl.Tips[i], base, base_height) // recursively process any nodes

				// logger.Infof("Block ordered %s %d %d", bl.Tips[i],chain.Load_Block_Topological_order(bl.Tips[i]), chain.Load_Block_Topological_order(base) )
			}
		}
	}

	if base != blid {
		unique_map[base] = chain.Load_Block_Cumulative_Difficulty(base)
	}

	unique_map[blid] = new(big.Int).Set(diff) // use whatever diff was supplied
	score := new(big.Int).SetInt64(0)
	for _, v := range unique_map {
		score.Add(score, v)
	}

	//set in cache, save a copy in cache
	{
		map_copy := map[crypto.Hash]*big.Int{}
		for k, v := range unique_map {
			map_copy[k] = v
		}
		chain.lrucache_workscore.Add(cachekey{blid, base, base_height}, map_copy)
	}

	return unique_map, score

}

// this function finds a common base  which can be used to compare tips
// weight is replace by height
func (chain *Blockchain) find_common_base(tips []crypto.Hash) (base crypto.Hash, base_height int64) {

	scores := make([]BlockScore, len(tips), len(tips))

	// var base crypto.Hash
	var best_height int64
	for i := range tips {
		tip_height := chain.Load_Height_for_BL_ID(tips[i])
		if tip_height > best_height {
			best_height = tip_height
		}
	}

	for i := range tips {
		scores[i] = chain.FindTipBase(tips[i], best_height) // we should chose the lowest weight
		scores[i].Height = chain.Load_Height_for_BL_ID(scores[i].BLID)
	}
	// base is the lowest height
	sort_ascending_by_height(scores)

	base = scores[0].BLID
	base_height = scores[0].Height

	return

}

// this function finds a common base  which can be used to compare tips based on cumulative difficulty
func (chain *Blockchain) find_best_tip(tips []crypto.Hash, base crypto.Hash, base_height int64) (best crypto.Hash) {

	tips_scores := make([]BlockScore, len(tips), len(tips))

	for i := range tips {
		tips_scores[i].BLID = tips[i] // we should chose the lowest weight
		_, tips_scores[i].Cumulative_Difficulty = chain.FindTipWorkScore(tips[i], base, base_height)
	}

	sort_descending_by_cumulative_difficulty(tips_scores)

	best = tips_scores[0].BLID
	//   base_height = scores[0].Weight

	return best

}

func (chain *Blockchain) calculate_mainchain_distance_internal_recursive(unique_map map[crypto.Hash]int64, blid crypto.Hash) {
	tips := chain.Get_Block_Past(blid)
	for i := range tips {
		ordered := chain.Is_Block_Topological_order(tips[i])
		if ordered {
			unique_map[tips[i]] = chain.Load_Height_for_BL_ID(tips[i])
		} else {
			chain.calculate_mainchain_distance_internal_recursive(unique_map, tips[i]) // recursively process any nodes
		}
	}
	return
}

// NOTE: some of the past may not be in the main chain  right now and need to be travelled recursively
// distance is number of hops to find a node, which is itself
func (chain *Blockchain) calculate_mainchain_distance(blid crypto.Hash) int64 {

	unique_map := map[crypto.Hash]int64{}
	//tips := chain.Get_Block_Past(dbtx, blid)

	//fmt.Printf("tips  %+v \n", tips)

	// if the block is already in order, no need to look back

	ordered := chain.Is_Block_Topological_order(blid)
	if ordered {
		unique_map[blid] = chain.Load_Height_for_BL_ID(blid)
	} else {
		chain.calculate_mainchain_distance_internal_recursive(unique_map, blid)
	}

	//for i := range tips {
	//}

	//fmt.Printf("unique_map %+v \n", unique_map)

	lowest_height := int64(0x7FFFFFFFFFFFFFFF) // max possible
	// now we need to find the lowest height
	for k, v := range unique_map {
		_ = k
		if lowest_height >= v {
			lowest_height = v
		}
	}

	return int64(lowest_height)
}

// converts a DAG's partial order into a full order, this function is recursive
// generate full order should be only callled on the basis of base blocks which satisfy sync block properties as follows
// generate full order is called on maximum weight tip at every tip change
// blocks are ordered recursively, till we find a find a block  which is already in the chain
func (chain *Blockchain) Generate_Full_Order(blid crypto.Hash, base crypto.Hash, base_height int64, level int) (order_bucket []crypto.Hash) {

	// return from cache if possible
	if tmp_order, ok := chain.lrucache_fullorder.Get(cachekey{blid, base, base_height}); ok {
		order := tmp_order.([]crypto.Hash)
		order_bucket = make([]crypto.Hash, len(order), len(order))
		copy(order_bucket, order[0:])
		return
	}

	bl, err := chain.Load_BL_FROM_ID(blid)
	if err != nil {
		panic(fmt.Sprintf("Block NOT found %s", blid))
	}

	if len(bl.Tips) == 0 {
		gbl := Generate_Genesis_Block()
		order_bucket = append(order_bucket, gbl.GetHash())
		return
	}

	// if the block has been previously ordered,  stop the recursion and return it as base
	//if chain.Is_Block_Topological_order(blid){
	if blid == base {
		order_bucket = append(order_bucket, blid)
		// logger.Infof("Generate order base reached  base %s", base)
		return
	}

	// we need to order previous tips first
	var tips_scores []BlockScore
	//tips_scores := make([]BlockScore,len(bl.Tips),len(bl.Tips))

	node_maps := map[crypto.Hash]map[crypto.Hash]*big.Int{}
	_ = node_maps
	for i := range bl.Tips {

		ordered := chain.Is_Block_Topological_order(bl.Tips[i])

		if !ordered {
			var score BlockScore
			score.BLID = bl.Tips[i]
			//node_maps[bl.Tips[i]], score.Weight = chain.FindTipWorkScore(bl.Tips[i],base,base_height)
			score.Cumulative_Difficulty = chain.Load_Block_Cumulative_Difficulty(bl.Tips[i])

			tips_scores = append(tips_scores, score)

		} else if ordered && chain.Load_Block_Topological_order(bl.Tips[i]) >= chain.Load_Block_Topological_order(base) {

			//  logger.Infof("Generate order topo order wrt base %d %d", chain.Load_Block_Topological_order(dbtx,bl.Tips[i]), chain.Load_Block_Topological_order(dbtx,base))
			var score BlockScore
			score.BLID = bl.Tips[i]

			//score.Weight = chain.Load_Block_Cumulative_Difficulty(bl.Tips[i])
			score.Cumulative_Difficulty = chain.Load_Block_Cumulative_Difficulty(bl.Tips[i])

			tips_scores = append(tips_scores, score)
		}

	}

	sort_descending_by_cumulative_difficulty(tips_scores)

	// now we must add the nodes in the topographical order

	for i := range tips_scores {
		tmp_bucket := chain.Generate_Full_Order(tips_scores[i].BLID, base, base_height, level+1)
		for j := range tmp_bucket {
			//only process if  this block is unsettled
			//if !chain.IsBlockSettled(tmp_bucket[j]) {
			// if order is already decided, do not order it again
			if !sliceExists(order_bucket, tmp_bucket[j]) {
				order_bucket = append(order_bucket, tmp_bucket[j])
			}
			//}
		}
	}
	// add self to the end, since all past nodes have been ordered
	order_bucket = append(order_bucket, blid)

	//  logger.Infof("Generate Order %s %+v  %+v", blid , order_bucket, tips_scores)

	//set in cache, save a copy in cache
	{
		order_copy := make([]crypto.Hash, len(order_bucket), len(order_bucket))
		copy(order_copy, order_bucket[0:])

		chain.lrucache_fullorder.Add(cachekey{blid, base, base_height}, order_copy)
	}

	if level == 0 {
		//logger.Warnf("generating full order for block %s %d", blid, level)
		//
		//   for i := range order_bucket{
		//            logger.Infof("%2d  %s", i, order_bucket[i])
		//   }

		//logger.Warnf("generating full order finished")
	}
	return
}

// tells whether the hash already exists in slice
func sliceExists(slice []crypto.Hash, hash crypto.Hash) bool {
	for i := range slice {
		if slice[i] == hash {
			return true
		}
	}
	return false
}
