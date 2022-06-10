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

import "os"
import "fmt"
import "sync"
import "time"
import "bytes"
import "runtime/debug"
import "strings"

import "runtime"
import "context"
import "golang.org/x/crypto/sha3"
import "golang.org/x/sync/semaphore"
import "github.com/go-logr/logr"

import "sync/atomic"

import "github.com/hashicorp/golang-lru"

import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/metrics"

import "github.com/deroproject/derohe/dvm"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/blockchain/mempool"
import "github.com/deroproject/derohe/blockchain/regpool"

import "github.com/deroproject/graviton"

// all components requiring access to blockchain must use , this struct to communicate
// this structure must be update while mutex
type Blockchain struct {
	Store      storage                     // interface to storage layer
	Top_ID     crypto.Hash                 // id of the top block
	Pruned     int64                       // until where the chain has been pruned
	MiniBlocks *block.MiniBlocksCollection // used for consensus

	Tips map[crypto.Hash]crypto.Hash // current tips

	mining_blocks_cache          *lru.Cache // used to cache blocks which have been supplied to mining
	cache_IsMiniblockPowValid    *lru.Cache // used to cache mini blocks pow test result
	cache_IsNonceValidTips       *lru.Cache // used to cache nonce tests on specific tips
	cache_IsAddressHashValid     *lru.Cache // used to cache some outputs
	cache_Get_Difficulty_At_Tips *lru.Cache // used to cache some outputs
	cache_BlockPast              *lru.Cache // used to cache a blocks past
	cache_BlockHeight            *lru.Cache // used to cache a blocks past
	cache_VersionMerkle          *lru.Cache // used to cache a versions merkle root

	integrator_address rpc.Address // integrator rewards will be given to this address

	cache_enabled bool // enables all cache, based on ENV  DISABLE_CACHE

	Difficulty        uint64           // current cumulative difficulty
	Median_Block_Size uint64           // current median block size
	Mempool           *mempool.Mempool // normal tx pool
	Regpool           *regpool.Regpool // registration pool
	Exit_Event        chan bool        // blockchain is shutting down and we must quit ASAP

	Top_Block_Median_Size uint64 // median block size of current top block
	Top_Block_Base_Reward uint64 // top block base reward

	simulator bool // is simulator mode

	P2P_Block_Relayer     func(*block.Complete_Block, uint64) // tell p2p to broadcast any block this daemon hash found
	P2P_MiniBlock_Relayer func(mbl block.MiniBlock, peerid uint64)

	RPC_NotifyNewBlock      *sync.Cond // used to notify rpc that a new block has been found
	RPC_NotifyHeightChanged *sync.Cond // used to notify rpc that  chain height has changed due to addition of block
	RPC_NotifyNewMiniBlock  *sync.Cond // used to notify rpc that a new mini block has been found

	Sync bool // whether the sync is active, used while bootstrapping

	sync.RWMutex
}

var logger logr.Logger = logr.Discard() // default discard all logs

// All blockchain activity is store in a single

/* do initialisation , setup storage, put genesis block and chain in store
   This is the first component to get up
   Global parameters are picked up  from the config package
*/

func Blockchain_Start(params map[string]interface{}) (*Blockchain, error) {

	var err error
	var chain Blockchain

	logger = globals.Logger.WithName("CORE")
	logger.V(1).Info("Initialising")

	if err = chain.Store.Initialize(params); err != nil {
		return nil, err
	}
	chain.Tips = map[crypto.Hash]crypto.Hash{}
	chain.MiniBlocks = block.CreateMiniBlockCollection()

	var addr *rpc.Address
	if params["--integrator-address"] == nil {
		if addr, err = rpc.NewAddress(strings.TrimSpace(globals.Config.Dev_Address)); err != nil {
			return nil, err
		}
	} else {
		if addr, err = rpc.NewAddress(strings.TrimSpace(params["--integrator-address"].(string))); err != nil {
			return nil, err
		}
	}
	chain.integrator_address = *addr

	logger.Info("will use", "integrator_address", chain.integrator_address.String())

	if chain.cache_IsMiniblockPowValid, err = lru.New(8192); err != nil { // temporary cache for miniblock difficulty
		return nil, err
	}
	if chain.cache_Get_Difficulty_At_Tips, err = lru.New(8192); err != nil { // temporary cache for difficulty
		return nil, err
	}
	if chain.cache_IsNonceValidTips, err = lru.New(100 * 1024); err != nil { // temporary cache for nonce checks
		return nil, err
	}

	if chain.cache_IsAddressHashValid, err = lru.New(100 * 1024); err != nil { // temporary cache for valid address
		return nil, err
	}
	if chain.mining_blocks_cache, err = lru.New(256); err != nil { // temporary cache for miniing blocks
		return nil, err
	}

	if chain.cache_BlockPast, err = lru.New(1024); err != nil { // temporary cache for a blocks past
		return nil, err
	}

	if chain.cache_BlockHeight, err = lru.New(10 * 1024); err != nil { // temporary cache for a blocks height
		return nil, err
	}

	if chain.cache_VersionMerkle, err = lru.New(1024); err != nil { // temporary cache for a snapshot version
		return nil, err
	}

	chain.cache_enabled = os.Getenv("DISABLE_CACHE") == "" // disable cache if the environ var is set
	if !chain.cache_enabled {
		logger.Info("All caching except mining jobs will be disabled")
	}

	if params["--simulator"] == true {
		chain.simulator = true // enable simulator mode, this will set hard coded difficulty to 1
	}

	chain.Exit_Event = make(chan bool) // init exit channel

	// init mempool before chain starts
	if chain.Mempool, err = mempool.Init_Mempool(params); err != nil {
		return nil, err
	}
	if chain.Regpool, err = regpool.Init_Regpool(params); err != nil {
		return nil, err
	}

	chain.RPC_NotifyNewBlock = sync.NewCond(&sync.Mutex{})      // used by dero daemon to notify all websockets that new block has arrived
	chain.RPC_NotifyHeightChanged = sync.NewCond(&sync.Mutex{}) // used by dero daemon to notify all websockets that chain height has changed
	chain.RPC_NotifyNewMiniBlock = sync.NewCond(&sync.Mutex{})  // used by dero daemon to notify all websockets that new miniblock has arrived

	if chain.Store.Topo_store.Count() == 0 && !chain.Store.IsBalancesIntialized() {
		logger.Info("Genesis block not in store, add it now")
		var complete_block block.Complete_Block
		bl := Generate_Genesis_Block()
		complete_block.Bl = &bl

		if err, ok := chain.Add_Complete_Block(&complete_block); !ok {
			logger.Error(err, "Failed to add genesis block, we can no longer continue.")
			return nil, err
		}
	}

	init_hard_forks(params) // hard forks must be initialized asap

	chain.Initialise_Chain_From_DB() // load the chain from the disk

try_again:
	version, err := chain.ReadBlockSnapshotVersion(chain.Get_Top_ID())
	if err != nil {
		chain.Rewind_Chain(1) // rewind 1 block
		goto try_again
	}
	// this case happens when chain syncs from rsync and if rsync takes more time than block_time
	// basically this can also be fixed, if topo.map file is renamed to name starting with 'a'
	// it should be the first file to be synced, but instead of renaming file as fix
	// we are fixing it by checking how much we have progressed and skip those block
	if _, err = chain.Load_Merkle_Hash(version); err != nil {
		chain.Rewind_Chain(1) // rewind 1 block
		goto try_again
	}

	if chain.Pruned >= 1 {
		logger.Info("Chain Pruned till", "topoheight", chain.Pruned)
	}

	// detect case if chain was corrupted earlier,so as it can be deleted and resynced
	if chain.Pruned < globals.Config.HF1_HEIGHT && globals.IsMainnet() && chain.Get_Height() >= globals.Config.HF1_HEIGHT+1 {
		toporecord, err := chain.Store.Topo_store.Read(globals.Config.HF1_HEIGHT + 1)
		if err != nil {
			panic(err)
		}
		var ss *graviton.Snapshot
		ss, err = chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
		if err != nil {
			panic(err)
		}

		var namehash_scid crypto.Hash
		namehash_scid[31] = 1
		sc_data_tree, err := ss.GetTree(string(namehash_scid[:]))
		if err != nil {
			panic(err)
		}

		if code_bytes, err := sc_data_tree.Get(dvm.SC_Code_Key(namehash_scid)); err == nil {
			var v dvm.Variable
			if err = v.UnmarshalBinary(code_bytes); err != nil {
				panic("Unmarshal error")
			}
			if !strings.Contains(v.ValueString, "UpdateCode") {
				logger.Error(nil, "Chain corruption detected")
				logger.Error(nil, "You need to delete existing mainnet folder and resync chain again")
				os.Exit(-1)
				return nil, err
			}

		} else {
			panic(err)
		}
	}

	metrics.Version = config.Version.String()
	go metrics.Dump_metrics_data_directly(logger, globals.Arguments["--node-tag"]) // enable metrics if someone needs them

	chain.Sync = true
	if chain.Get_Height() <= 1 {
		if globals.Arguments["--fastsync"] != nil && globals.Arguments["--fastsync"].(bool) {
			chain.Sync = !globals.Arguments["--fastsync"].(bool)
		}
	}

	atomic.AddUint32(&globals.Subsystem_Active, 1) // increment subsystem

	globals.Cron.AddFunc("@every 360s", clean_up_valid_cache) // cleanup valid tx cache
	globals.Cron.AddFunc("@every 60s", func() {               // mempool house keeping

		stable_height := int64(0)
		if r := recover(); r != nil {
			logger.Error(nil, "Mempool House Keeping triggered panic", "r", r, "height", stable_height)
		}

		stable_height = chain.Get_Stable_Height()

		// give mempool an oppurtunity to clean up tx, but only if they are not mined
		chain.Mempool.HouseKeeping(uint64(stable_height))

		top_block_topo_index := chain.Load_TOPO_HEIGHT()

		if top_block_topo_index < 2 {
			return
		}

		top_block_topo_index -= 2

		blid, err := chain.Load_Block_Topological_order_at_index(top_block_topo_index)
		if err != nil {
			panic(err)
		}

		record_version, err := chain.ReadBlockSnapshotVersion(blid)
		if err != nil {
			panic(err)
		}

		// give regpool a chance to register
		if ss, err := chain.Store.Balance_store.LoadSnapshot(record_version); err == nil {
			if balance_tree, err := ss.GetTree(config.BALANCE_TREE); err == nil {
				chain.Regpool.HouseKeeping(uint64(stable_height), func(tx *transaction.Transaction) bool {
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
	})

	return &chain, nil
}

// return integrator address
func (chain *Blockchain) IntegratorAddress() rpc.Address {
	return chain.integrator_address
}
func (chain *Blockchain) SetIntegratorAddress(addr rpc.Address) {
	chain.integrator_address = addr
}

// this function is called to read blockchain state from DB
// It is callable at any point in time

func (chain *Blockchain) Initialise_Chain_From_DB() {
	chain.Lock()
	defer chain.Unlock()

	chain.Pruned = chain.LocatePruneTopo()

	// find the tips from the chain , first by reaching top height
	// then downgrading to top-10 height
	// then reworking the chain to get the tip
	best_height := chain.Load_TOP_HEIGHT()

	chain.Tips = map[crypto.Hash]crypto.Hash{} // reset the map
	// reload top tip from disk
	top := chain.Get_Top_ID()

	chain.Tips[top] = top // we only can load a single tip from db

	logger.V(1).Info("Reloaded Chain from disk", "Tips", chain.Tips, "Height", best_height)
}

// before shutdown , make sure p2p is confirmed stopped
func (chain *Blockchain) Shutdown() {

	chain.Lock()            // take the lock as chain is no longer in unsafe mode
	close(chain.Exit_Event) // send signal to everyone we are shutting down

	chain.Mempool.Shutdown() // shutdown mempool first
	chain.Regpool.Shutdown() // shutdown regpool first

	logger.Info("Stopping Blockchain")
	//chain.Store.Shutdown()
	atomic.AddUint32(&globals.Subsystem_Active, ^uint32(0)) // this decrement 1 fom subsystem
	logger.Info("Stopped Blockchain")
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
	defer globals.Recover(1)

	bl := cbl.Bl // small pointer to block

	block_hash = bl.GetHash()

	block_logger := logger.WithName(fmt.Sprintf("blid_%s", block_hash)).V(1)
	for k := range chain.Tips { // very fast path
		if block_hash == k {
			return errormsg.ErrAlreadyExists, false // block already in chain skipping it
		}
	}

	// check if block already exist skip it
	if chain.Is_Block_Topological_order(block_hash) {
		return errormsg.ErrAlreadyExists, false // block already in chain skipping it
	}

	result = false
	height_changed := false

	processing_start := time.Now()

	//old_top := chain.Load_TOP_ID() // store top as it may change
	defer func() {

		// safety so if anything wrong happens, verification fails
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recovered while adding new block", "blid", block_hash, "r", r, "stack", fmt.Sprintf("%s", string(debug.Stack())))
			result = false
			err = errormsg.ErrPanic
		}

		if result == true { // block was successfully added, commit it atomically
			logger.V(2).Info("Block successfully accepted by chain", "blid", block_hash.String(), "err", err)

			// gracefully try to instrument
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.V(1).Error(nil, "Recovered while instrumenting", "r", r, "stack", debug.Stack())
					}
				}()
				metrics.Set.GetOrCreateCounter("blockchain_tx_total").Add(len(cbl.Bl.Tx_hashes))
				metrics.Set.GetOrCreateHistogram("block_txcount_histogram_short").Update(float64(len(cbl.Bl.Tx_hashes)))
				metrics.Set.GetOrCreateHistogram("block_processing_duration_histogram_seconds").UpdateDuration(processing_start)

				// tracks counters for tx internals, do we need to serialize everytime, just for stats
				{
					complete_block_size := 0
					for i := 0; i < len(cbl.Txs); i++ {
						tx_size := len(cbl.Txs[i].Serialize())
						complete_block_size += tx_size
						metrics.Set.GetOrCreateHistogram("transaction_size_histogram_bytes").Update(float64(tx_size))
						metrics.Set.GetOrCreateCounter(fmt.Sprintf(`transaction_total{type="%s"}`, cbl.Txs[i].TransactionType.String())).Inc()
						if len(cbl.Txs[i].Payloads) >= 1 {
							metrics.Set.GetOrCreateHistogram("transaction_ring_histogram_short").Update(float64(cbl.Txs[i].Payloads[0].Statement.RingSize))
							metrics.Set.GetOrCreateHistogram("transaction_payloads_histogram_short").Update(float64(len(cbl.Txs[i].Payloads)))
						}
					}
					metrics.Set.GetOrCreateHistogram("block_size_histogram_bytes").Update(float64(complete_block_size))
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

		} else {
			logger.V(1).Error(err, "Block rejected by chain", "BLID", block_hash, "bl", fmt.Sprintf("%x", bl.Serialize()), "stack", debug.Stack())
			logger.V(1).Error(err, "Block rejected by chain", "BLID", block_hash)
		}
	}()

	// first of all lets do some quick checks
	// before doing extensive checks
	result = false

	// check if block already exist skip it
	if chain.Is_Block_Topological_order(block_hash) {
		return errormsg.ErrAlreadyExists, false // block already in chain skipping it
	}

	for k := range chain.Tips {
		if block_hash == k {
			return errormsg.ErrAlreadyExists, false // block already in chain skipping it
		}
	}

	if bl.Height > uint64(chain.Get_Height()+2) {
		return fmt.Errorf("advance Block"), false // block in future skipping it
	}

	if bl.Height > uint64(config.STABLE_LIMIT) {
		if bl.Height < uint64(chain.Get_Height()-config.STABLE_LIMIT) {
			return fmt.Errorf("previous Block"), false // block in past skipping it
		}
	}

	// only 1 tips allowed in block
	if len(bl.Tips) > 1 {
		block_logger.V(1).Error(fmt.Errorf("More than 1 tips present in block rejecting"), "")
		return errormsg.ErrPastMissing, false
	}

	// check whether the tips exist in our chain, if not reject
	for i := range bl.Tips {
		if !chain.Block_Exists(bl.Tips[i]) { // alt-tips might not have a topo order at this point, so make sure they exist on disk
			block_logger.V(1).Error(fmt.Errorf("Tip is NOT present in chain, skipping it till we get a parent"), "", "missing_tip", bl.Tips[i].String())
			return errormsg.ErrPastMissing, false
		}
	}

	block_height := chain.Calculate_Height_At_Tips(bl.Tips)
	for i := range bl.Tips { // previous block can be refer to only recent blocks, making some attacks almost impossible
		if block_height != chain.Load_Block_Height(bl.Tips[i])+1 {
			block_logger.V(1).Error(fmt.Errorf("Block  rejected since it is in too past"), "", "block_height", block_height, "tip_height", chain.Load_Block_Height(bl.Tips[i]))
			return errormsg.ErrInvalidBlock, false
		}
	}

	if block_height == 0 && int64(bl.Height) == block_height && len(bl.Tips) != 0 {
		block_logger.Error(fmt.Errorf("Genesis block cannot have tips."), "", "tip_count", len(bl.Tips))
		return errormsg.ErrInvalidBlock, false
	}

	if len(bl.Tips) >= 1 && bl.Height == 0 {
		block_logger.Error(fmt.Errorf("Genesis block can only be at height 0"), "", "tip_count", len(bl.Tips))
		return errormsg.ErrInvalidBlock, false
	}

	//if block_height != 0 && block_height < chain.Get_Stable_Height() {
	//	block_logger.Error(fmt.Errorf("Block rejected since it is stale."), "", "stable height", chain.Get_Stable_Height(), "block height", block_height)
	//	return errormsg.ErrInvalidBlock, false
	//}

	// make sure time is NOT into future,
	// if clock diff is more than  50 millisecs, reject the block
	if bl.Timestamp > (uint64(globals.Time().UTC().UnixMilli() + 50)) { // give 50 millisec passing
		block_logger.Error(fmt.Errorf("Rejecting Block, timestamp is too much into future, make sure that system clock is correct"), "")
		return errormsg.ErrFutureTimestamp, false
	}

	// verify that the clock is not being run in reverse
	// the block timestamp cannot be less than any of the parents
	for i := range bl.Tips {
		if chain.Load_Block_Timestamp(bl.Tips[i]) > bl.Timestamp {
			//fmt.Printf("timestamp prev %d  cur timestamp %d\n", chain.Load_Block_Timestamp(bl.Tips[i]), bl.Timestamp)

			block_logger.Error(fmt.Errorf("Block timestamp is  less than its parent."), "rejecting block")
			return errormsg.ErrInvalidTimestamp, false
		}
	}

	// check whether the major version ( hard fork) is valid
	if !chain.Check_Block_Version(bl) {
		block_logger.Error(fmt.Errorf("Rejecting !! Block has invalid fork version"), "actual", bl.Major_Version, "expected", chain.Get_Current_Version_at_Height(chain.Calculate_Height_At_Tips(bl.Tips)))
		return errormsg.ErrInvalidBlock, false
	}

	// verify whether the tips are reachable from one another
	if bl.Height >= 2 && !chain.CheckDagStructure(bl.Tips) {
		block_logger.Error(fmt.Errorf("Rejecting !! Block has invalid reachability"), "Invalid rechability", "tips", bl.Tips)
		return errormsg.ErrInvalidBlock, false

	}

	// if the block is referencing any past tip too distant into its history
	for i := range bl.Tips {
		if int64(bl.Height)-1 != chain.Load_Block_Height(bl.Tips[i]) {
			block_logger.Error(fmt.Errorf("Rusty TIP  mined by ROGUE miner discarding block"), "", "best height", bl.Height, "deviation", int64(bl.Height)-chain.Load_Block_Height(bl.Tips[i]))
			return errormsg.ErrInvalidBlock, false
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
				block_logger.Error(fmt.Errorf("Block is bigger than max permitted"), "Rejecting", "Actual", block_size, "MAX", config.STARGATE_HE_MAX_BLOCK_SIZE)
				return errormsg.ErrInvalidSize, false
			}
		}
	}

	// verify everything related to miniblocks in one go
	if !chain.simulator {
		if err = Verify_MiniBlocks(*cbl.Bl); err != nil { // verifies the miniblocks all refer to current block
			return err, false
		}

		if bl.Height != 0 { // a genesis block doesn't have miniblock
			// verify hash of miniblock for corruption
			if err = chain.Verify_MiniBlocks_HashCheck(cbl); err != nil {
				return err, false
			}
		}

		for _, mbl := range bl.MiniBlocks {
			var miner_hash crypto.Hash
			copy(miner_hash[:], mbl.KeyHash[:])
			if mbl.Final == false && !chain.IsAddressHashValid(true, miner_hash) {
				err = fmt.Errorf("miner address not registered")
				return err, false
			}
		}

		// verify Pow of miniblocks
		for i, mbl := range bl.MiniBlocks {
			if !chain.VerifyMiniblockPoW(bl, mbl) {
				block_logger.Error(fmt.Errorf("MiniBlock has invalid PoW"), "rejecting", "i", i)
				return errormsg.ErrInvalidPoW, false
			}
		}
	}

	{ // miner TX checks are here
		if bl.Height == 0 && !bl.Miner_TX.IsPremine() { // genesis block contain premine tx a
			block_logger.Error(fmt.Errorf("Miner tx failed verification for genesis"), "rejecting")
			return errormsg.ErrInvalidBlock, false
		}

		if bl.Height != 0 && !bl.Miner_TX.IsCoinbase() { // all blocks except genesis block contain coinbase TX
			block_logger.Error(fmt.Errorf("Miner tx failed  it is not coinbase"), "rejecting")
			return errormsg.ErrInvalidBlock, false
		}

		// always check whether the coin base tx is okay
		if bl.Height != 0 {
			if err = chain.Verify_Transaction_Coinbase(cbl, &bl.Miner_TX); err != nil { // if miner address is not registered give error
				//block_logger.Warnf("Error verifying coinbase tx, err :'%s'", err)
				return err, false
			}
		}
	}

	// now we need to verify each and every tx in detail
	// we need to verify each and every tx contained in the block, sanity check everything
	// first of all check, whether all the tx contained in the block, match their hashes
	{
		if len(bl.Tx_hashes) != len(cbl.Txs) {
			block_logger.Error(fmt.Errorf("Missing TX"), "Incomplete block", "expected_tx", len(bl.Tx_hashes), "actual_tx", len(cbl.Txs))
			return errormsg.ErrInvalidBlock, false
		}

		// first check whether the complete block contains any diplicate hashes
		tx_checklist := map[crypto.Hash]bool{}
		for i := 0; i < len(bl.Tx_hashes); i++ {
			tx_checklist[bl.Tx_hashes[i]] = true
		}

		if len(tx_checklist) != len(bl.Tx_hashes) { // block has duplicate tx, reject
			block_logger.Error(fmt.Errorf("duplicate TX"), "Incomplete block", "duplicate count", len(bl.Tx_hashes)-len(tx_checklist))
			return errormsg.ErrInvalidBlock, false
		}

		for i, tx := range cbl.Txs {
			if tx.Height >= bl.Height {
				block_logger.Error(fmt.Errorf("Invalid TX Height"), "TX height cannot be more than block", "txid", cbl.Txs[i].GetHash().String())
				return errormsg.ErrInvalidBlock, false
			}
		}

		// now lets loop through complete block, matching each tx
		// detecting any duplicates using txid hash
		for i := 0; i < len(cbl.Txs); i++ {
			tx_hash := cbl.Txs[i].GetHash()
			if _, ok := tx_checklist[tx_hash]; !ok {
				// tx is NOT found in map, RED alert reject the block
				block_logger.Error(fmt.Errorf("Missing TX"), "TX missing", "txid", tx_hash.String())
				return errormsg.ErrInvalidBlock, false
			}
		}
	}

	// another check, whether the block contains any duplicate registration within the block
	// block wide duplicate input detector
	{
		reg_map := map[string]bool{}
		for i := 0; i < len(cbl.Txs); i++ {

			if cbl.Txs[i].TransactionType == transaction.REGISTRATION {
				if _, ok := reg_map[string(cbl.Txs[i].MinerAddress[:])]; ok {
					block_logger.Error(fmt.Errorf("Double Registration TX"), "duplicate registration", "txid", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}

				tx_hash := cbl.Txs[i].GetHash()
				if chain.simulator == false && tx_hash[0] != 0 && tx_hash[1] != 0 {
					return fmt.Errorf("Registration TX has not solved PoW"), false
				}
				reg_map[string(cbl.Txs[i].MinerAddress[:])] = true
			}
		}
	}

	// another check, whether the block contains any colliding txs
	if len(bl.Tips) == 2 {
		for i := range cbl.Txs {
			if cbl.Txs[i].IsProofRequired() {
				if cbl.Txs[i].BLID == bl.Tips[0] || cbl.Txs[i].BLID == bl.Tips[1] {
					block_logger.Error(fmt.Errorf("Colliding TXs"), "may contain colliding transactions", "txid", cbl.Txs[i].GetHash())
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
			if cbl.Txs[i].TransactionType == transaction.NORMAL || cbl.Txs[i].TransactionType == transaction.BURN_TX || cbl.Txs[i].TransactionType == transaction.SC_TX {
				for j := range cbl.Txs[i].Payloads {
					if _, ok := nonce_map[cbl.Txs[i].Payloads[j].Proof.Nonce()]; ok {
						block_logger.Error(fmt.Errorf("Double Spend TX within block"), "duplicate nonce", "txid", cbl.Txs[i].GetHash())
						return errormsg.ErrTXDoubleSpend, false
					}
					nonce_map[cbl.Txs[i].Payloads[j].Proof.Nonce()] = true
				}
			}
		}
	}

	// all blocks except genesis will have  history
	// so make sure txs are connected
	if bl.Height >= 1 && len(cbl.Txs) > 0 {
		history := map[crypto.Hash]bool{}

		var history_array []crypto.Hash
		for i := range bl.Tips {
			h := int64(bl.Height) - 25
			if h < 0 {
				h = 0
			}
			history_array = append(history_array, chain.get_ordered_past(bl.Tips[i], h)...)
		}
		for _, h := range history_array {
			history[h] = true
		}

		block_height = chain.Calculate_Height_At_Tips(bl.Tips)
		for i, tx := range cbl.Txs {
			if cbl.Txs[i].TransactionType == transaction.NORMAL || cbl.Txs[i].TransactionType == transaction.BURN_TX || cbl.Txs[i].TransactionType == transaction.SC_TX {
				if history[cbl.Txs[i].BLID] != true {
					block_logger.Error(fmt.Errorf("Double Spend TX within block"), "unreferable history", "txid", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}
				if tx.Height != uint64(chain.Load_Height_for_BL_ID(cbl.Txs[i].BLID)) {
					block_logger.Error(fmt.Errorf("Double Spend TX within block"), "blid/height mismatch", "txid", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}

				if block_height-int64(tx.Height) < TX_VALIDITY_HEIGHT {

				} else {
					block_logger.Error(fmt.Errorf("Double Spend TX within block"), "long distance tx not supported", "txid", cbl.Txs[i].GetHash())
					return errormsg.ErrTXDoubleSpend, false
				}

				if tx.TransactionType == transaction.SC_TX {
					if tx.SCDATA.Has(rpc.SCACTION, rpc.DataUint64) {
						if rpc.SC_INSTALL == rpc.SC_ACTION(tx.SCDATA.Value(rpc.SCACTION, rpc.DataUint64).(uint64)) {
							txid := tx.GetHash()
							if txid[0] < 0x80 || txid[31] < 0x80 { // last byte should be more than 0x80
								block_logger.Error(fmt.Errorf("Invalid SCID"), "SCID installing tx must end with >0x80 byte", "txid", cbl.Txs[i].GetHash())
								return errormsg.ErrTXDoubleSpend, false
							}
						}
					}
				}
			}
		}
	}

	// we need to verify each tx with tips
	{
		fail_count := int32(0)
		wg := sync.WaitGroup{}
		wg.Add(len(cbl.Txs)) // add total number of tx as work

		hf_version := chain.Get_Current_Version_at_Height(chain.Calculate_Height_At_Tips(bl.Tips))

		sem := semaphore.NewWeighted(int64(runtime.NumCPU()))

		for i := 0; i < len(cbl.Txs); i++ {
			sem.Acquire(context.Background(), 1)
			go func(j int) {
				defer sem.Release(1)
				defer wg.Done()
				if atomic.LoadInt32(&fail_count) >= 1 { // fail fast
					return
				}
				if err := chain.Verify_Transaction_NonCoinbase_CheckNonce_Tips(hf_version, cbl.Txs[j], bl.Tips); err != nil { // transaction verification failed
					atomic.AddInt32(&fail_count, 1) // increase fail count by 1
					block_logger.Error(err, "tx nonce verification failed", "txid", cbl.Txs[j].GetHash())
				}

			}(i)
		}

		wg.Wait()           // wait for verifications to finish
		if fail_count > 0 { // check the result
			block_logger.Error(fmt.Errorf("TX nonce verification failed"), "rejecting block")
			return errormsg.ErrInvalidTX, false
		}
	}

	// we need to anyways verify the TXS since proofs are not covered by checksum
	{
		fail_count := int32(0)
		wg := sync.WaitGroup{}
		wg.Add(len(cbl.Txs)) // add total number of tx as work

		sem := semaphore.NewWeighted(int64(runtime.NumCPU()))

		for i := 0; i < len(cbl.Txs); i++ {
			sem.Acquire(context.Background(), 1)

			go func(j int) {
				defer sem.Release(1)
				defer wg.Done()
				if atomic.LoadInt32(&fail_count) >= 1 { // fail fast
					return
				}
				if err := chain.Verify_Transaction_NonCoinbase(cbl.Txs[j]); err != nil { // transaction verification failed
					atomic.AddInt32(&fail_count, 1) // increase fail count by 1
					block_logger.Error(err, "tx verification failed", "txid", cbl.Txs[j].GetHash())
				}
			}(i)
		}

		wg.Wait()           // wait for verifications to finish
		if fail_count > 0 { // check the result
			block_logger.Error(fmt.Errorf("TX verification failed"), "rejecting block")
			return errormsg.ErrInvalidTX, false
		}
	}

	// we need to do more checks but only after tx has been expanded
	{
		var check_data cbl_verify // used to verify sanity of new block
		for i := 0; i < len(cbl.Txs); i++ {
			if !(cbl.Txs[i].IsCoinbase() || cbl.Txs[i].IsRegistration()) { // all other tx must go through this check
				if err = check_data.check(cbl.Txs[i], false); err == nil {
					check_data.check(cbl.Txs[i], true) // keep in record for future tx
				} else {
					block_logger.Error(err, "Invalid TX within block", "txid", cbl.Txs[i].GetHash())
					return errormsg.ErrInvalidTX, false
				}
			}
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

	chain.StoreBlock(bl, 0)

	// if the block is on a lower height tip, the block will not increase chain height
	height := chain.Load_Height_for_BL_ID(block_hash)
	if height > chain.Get_Height() || height == 0 { // exception for genesis block
		//atomic.StoreInt64(&chain.Height, height)
		height_changed = true
		block_logger.Info("Chain extended", "new height", height)
	} else {
		block_logger.Info("Chain extended but height is same", "new height", height)
	}

	{
		// TODO we must run smart contracts and TXs in this order
		// basically client protocol must run here
		// even if the HF has triggered we may still accept, old blocks for some time
		// so hf is detected block-wise and processed as such

		bl_current_hash := cbl.Bl.GetHash()
		bl_current := cbl.Bl
		current_topo_block := bl_current.Height
		logger.V(3).Info("will insert block ", "blid", bl_current_hash.String())

		//fmt.Printf("\ni %d bl %+v\n",i, bl_current)

		height_current := chain.Calculate_Height_At_Tips(bl_current.Tips)
		hard_fork_version_current := chain.Get_Current_Version_at_Height(height_current)

		// generate miner TX rewards as per client protocol
		if hard_fork_version_current == 1 {

		}

		var balance_tree, sc_meta *graviton.Tree
		var ss *graviton.Snapshot
		if bl_current.Height == 0 { // if it's genesis block
			if ss, err = chain.Store.Balance_store.LoadSnapshot(0); err != nil {
				panic(err)
			}
		} else { // we already have a block before us, use it

			record_version, err := chain.ReadBlockSnapshotVersion(bl.Tips[0])
			if err != nil {
				panic(err)
			}

			logger.V(1).Info("reading block snapshot", "blid", bl.Tips[0], "record_version", record_version)

			ss, err = chain.Store.Balance_store.LoadSnapshot(record_version)
			if err != nil {
				panic(err)
			}
		}
		if balance_tree, err = ss.GetTree(config.BALANCE_TREE); err != nil {
			panic(err)
		}
		if sc_meta, err = ss.GetTree(config.SC_META); err != nil {
			panic(err)
		}

		fees_collected := uint64(0)

		// side blocks only represent chain strenth , else they are are ignored
		// this means they donot get any reward , 0 reward
		// their transactions are ignored

		//chain.Store.Topo_store.Write(i+base_topo_index, full_order[i],0, int64(bl_current.Height)) // write entry so as sideblock could work
		var data_trees []*graviton.Tree

		{

			sc_change_cache := map[crypto.Hash]*graviton.Tree{} // cache entire changes for entire block

			// install hardcoded contracts
			if err = chain.install_hardcoded_contracts(sc_change_cache, ss, balance_tree, sc_meta, bl_current.Height); err != nil {
				panic(err)
			}

			for _, txhash := range bl_current.Tx_hashes { // execute all the transactions
				if tx_bytes, err := chain.Store.Block_tx_store.ReadTX(txhash); err != nil {
					panic(err)
				} else {
					var tx transaction.Transaction
					if err = tx.Deserialize(tx_bytes); err != nil {
						panic(err)
					}
					for t := range tx.Payloads {
						if !tx.Payloads[t].SCID.IsZero() {
							tree, _ := ss.GetTree(string(tx.Payloads[t].SCID[:]))
							sc_change_cache[tx.Payloads[t].SCID] = tree
						}
					}
					// we have loaded a tx successfully, now lets execute it
					tx_fees := chain.process_transaction(sc_change_cache, tx, balance_tree, bl_current.Height)

					//fmt.Printf("transaction %s type %s data %+v\n", txhash, tx.TransactionType, tx.SCDATA)
					if tx.TransactionType == transaction.SC_TX {
						tx_fees, err = chain.process_transaction_sc(sc_change_cache, ss, bl_current.Height, uint64(current_topo_block), bl_current.Timestamp/1000, bl_current_hash, tx, balance_tree, sc_meta)

						//fmt.Printf("Processsing sc err %s\n", err)
						if err == nil { // TODO process gasg here

						}
					}
					fees_collected += tx_fees
				}
			}

			// at this point, we must commit all the SCs, so entire tree hash is interlinked
			for scid, v := range sc_change_cache {
				meta_bytes, err := sc_meta.Get(dvm.SC_Meta_Key(scid))
				if err != nil {
					panic(err)
				}

				var meta dvm.SC_META_DATA // the meta contains metadata about SC

				if bl_current.Height < uint64(globals.Config.HF2_HEIGHT) {
					if err := meta.UnmarshalBinary(meta_bytes); err != nil {
						panic(err)
					}
					if meta.DataHash, err = v.Hash(); err != nil { // encode data tree hash
						panic(err)
					}
					sc_meta.Put(dvm.SC_Meta_Key(scid), meta.MarshalBinary())
				} else {
					if err := meta.UnmarshalBinaryGood(meta_bytes); err != nil {
						panic(err)
					}
					if meta.DataHash, err = v.Hash(); err != nil { // encode data tree hash
						panic(err)
					}
					sc_meta.Put(dvm.SC_Meta_Key(scid), meta.MarshalBinaryGood())
				}
				data_trees = append(data_trees, v)

				/*fmt.Printf("will commit tree name %x \n", v.GetName())
								c := v.Cursor()
					for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
					fmt.Printf("key=%x, value=%x\n", k, v)
				}*/

			}

			chain.process_miner_transaction(bl_current, bl_current.Height == 0, balance_tree, fees_collected, bl_current.Height)
		}

		// we are here, means everything is okay, lets commit the update balance tree
		data_trees = append(data_trees, balance_tree, sc_meta)
		//fmt.Printf("committing data trees %+v\n", data_trees)
		commit_version, err := graviton.Commit(data_trees...)
		if err != nil {
			panic(err)
		}

		chain.StoreBlock(bl_current, commit_version)

		if height_changed {
			// we need to write history until the entire chain is fixed

			fix_bl := bl_current
			fix_pos := bl_current.Height
			fix_commit_version := commit_version
			for ; ; fix_pos-- {

				chain.Store.Topo_store.Write(int64(fix_bl.Height), fix_bl.GetHash(), fix_commit_version, int64(fix_bl.Height))
				logger.V(1).Info("fixed loop", "topo", fix_pos)

				if fix_pos == 0 { // break if we reached genesis
					break
				}
				r, err := chain.Store.Topo_store.Read(int64(fix_pos - 1))
				if err != nil {
					panic(err)
				}

				if fix_bl.Tips[0] == r.BLOCK_ID { // break if we reached a common point
					break
				}

				// prepare for another round
				fix_commit_version, err = chain.ReadBlockSnapshotVersion(fix_bl.Tips[0])
				if err != nil {
					panic(err)
				}

				fix_bl, err = chain.Load_BL_FROM_ID(fix_bl.Tips[0])
				if err != nil {
					panic(err)
				}

			}

		}

		if logger.V(1).Enabled() {
			merkle_root, err := chain.Load_Merkle_Hash(commit_version)
			if err != nil {
				panic(err)
			}
			logger.V(1).Info("storing topo", "topo", int64(bl_current.Height), "blid", bl_current_hash, "topoheight", current_topo_block, "commit_version", commit_version, "committed_merkle", merkle_root)
		}

	}

	{

		// calculate new set of tips
		// this is done by removing all known tips which are in the past
		// and add this block as tip

		old_tips := chain.Get_TIPS()
		var tips []crypto.Hash
		new_tips := map[crypto.Hash]crypto.Hash{}

		for i := range old_tips {
			for j := range bl.Tips {
				if bl.Tips[j] == old_tips[i] {
					goto skip_tip
				}
			}
			tips = append(tips, old_tips[i])
		skip_tip:
		}

		tips = append(tips, bl.GetHash()) // add current block as new tip

		chain_height := chain.Get_Height()

		for i := range tips {
			tip_height := int64(chain.Load_Height_for_BL_ID(tips[i]))
			if (chain_height - tip_height) == 0 {

				new_tips[tips[i]] = tips[i]
			} else { // this should be a rare event, unless network has very high latency
				logger.V(2).Info("Rusty TIP declared stale", "tip", tips[i], "best height", chain_height, "tip_height", tip_height)
				//chain.transaction_scavenger(dbtx, tips[i]) // scavenge tx if possible
				// TODO we must include any TX from the orphan blocks back to the mempool to avoid losing any TX
			}
		}

		//block_logger.Info("New tips(after adding block) ", "tips", new_tips)

		chain.Tips = new_tips
	}

	// every 2000 block print a line
	if chain.Get_Height()%2000 == 0 {
		block_logger.Info(fmt.Sprintf("Chain Height %d", chain.Get_Height()))
	}

	purge_count := chain.MiniBlocks.PurgeHeight(chain.Get_Stable_Height()) // purge all miniblocks upto this height
	logger.V(2).Info("Purged miniblock", "count", purge_count)

	result = true

	// TODO fix hard fork
	// maintain hard fork votes to keep them SANE
	//chain.Recount_Votes() // does not return anything

	return // run any handlers necesary to atomically
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
	return chain.Get_Height() - config.STABLE_LIMIT
}

// we should be holding lock at this time, atleast read only

func (chain *Blockchain) Get_TIPS() (tips []crypto.Hash) {
	for _, x := range chain.Tips {
		tips = append(tips, x)
	}
	return tips
}

// check whether the block is a tip
func (chain *Blockchain) Is_Block_Tip(blid crypto.Hash) (result bool) {
	for k := range chain.Tips {
		if blid == k {
			return true
		}
	}
	return false
}

func (chain *Blockchain) Get_Difficulty() uint64 {
	return chain.Get_Difficulty_At_Tips(chain.Get_TIPS()).Uint64()
}

func (chain *Blockchain) Get_Network_HashRate() uint64 {
	return chain.Get_Difficulty()
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

// this is the only entrypoint for new txs in the chain
// add a transaction to MEMPOOL,
// verifying everything  means everything possible
// this only change mempool, no DB changes
func (chain *Blockchain) Add_TX_To_Pool(tx *transaction.Transaction) error {
	var err error

	if tx.IsPremine() {
		return fmt.Errorf("premine tx not mineable")
	}
	if tx.IsRegistration() { // registration tx will not go any forward

		tx_hash := tx.GetHash()
		if chain.simulator == false && !(tx_hash[0] == 0 && tx_hash[1] == 0 && tx_hash[2] == 0) {
			return fmt.Errorf("TX doesn't solve Pow")
		}

		// ggive regpool a chance to register
		if ss, err := chain.Store.Balance_store.LoadSnapshot(0); err == nil {
			if balance_tree, err := ss.GetTree(config.BALANCE_TREE); err == nil {
				if _, err := balance_tree.Get(tx.MinerAddress[:]); err == nil { // address already registered
					return fmt.Errorf("address already registered")
				} else { // add  to regpool
					if chain.Regpool.Regpool_Add_TX(tx, 0) {
						return nil
					} else {
						return fmt.Errorf("registration for address is already pending")
					}
				}
			} else {
				return err
			}
		} else {
			return err
		}
	}

	switch tx.TransactionType {
	case transaction.BURN_TX, transaction.NORMAL, transaction.SC_TX:
	default:
		return fmt.Errorf("such transaction type cannot appear in mempool")
	}

	txhash := tx.GetHash()

	// Coin base TX can not come through this path
	if tx.IsCoinbase() {
		logger.Error(fmt.Errorf("coinbase tx cannot appear in mempool"), "tx_rejected", "txid", txhash)
		return fmt.Errorf("TX rejected  coinbase tx cannot appear in mempool")
	}

	chain_height := uint64(chain.Get_Height())
	/*if chain_height > tx.Height {
		rlog.Tracef(2, "TX %s rejected since chain has already progressed", txhash)
		return fmt.Errorf("TX %s rejected since chain has already progressed", txhash)
	}*/

	// quick check without calculating everything whether tx is in pool, if yes we do nothing
	if chain.Mempool.Mempool_TX_Exist(txhash) {
		//rlog.Tracef(2, "TX %s rejected Already in MEMPOOL", txhash)
		return fmt.Errorf("TX %s rejected Already in MEMPOOL", txhash)
	}

	// check whether tx is already mined
	if _, err = chain.Store.Block_tx_store.ReadTX(txhash); err == nil {
		//rlog.Tracef(2, "TX %s rejected Already mined in some block", txhash)
		return fmt.Errorf("TX %s rejected Already mined in some block", txhash)
	}

	toporecord, err := chain.Store.Topo_store.Read(int64(tx.Height))
	if err != nil {
		return fmt.Errorf("TX %s rejected height(%d) reference not found", txhash, tx.Height)
	}
	if toporecord.BLOCK_ID != tx.BLID {
		return fmt.Errorf("TX %s rejected block (%s) reference not found", txhash, tx.BLID)
	}

	hf_version := chain.Get_Current_Version_at_Height(int64(chain_height))

	// if TX is too big, then it cannot be mined due to fixed block size, reject such TXs here
	// currently, limits are  as per consensus
	if uint64(len(tx.Serialize())) > config.STARGATE_HE_MAX_TX_SIZE {
		logger.Error(fmt.Errorf("Huge TX"), "TX rejected", "Actual Size", len(tx.Serialize()), "max possible ", config.STARGATE_HE_MAX_TX_SIZE)
		return fmt.Errorf("TX rejected  Size %d byte Max possible %d", len(tx.Serialize()), config.STARGATE_HE_MAX_TX_SIZE)
	}

	// check whether enough fees is provided in the transaction
	calculated_fee := chain.Calculate_TX_fee(hf_version, uint64(len(tx.Serialize())))
	provided_fee := tx.Fees() // get fee from tx

	//logger.WithFields(log.Fields{"txid": txhash}).Warnf("TX fees check disabled  provided fee %d calculated fee %d", provided_fee, calculated_fee)
	if !chain.simulator && calculated_fee > provided_fee {
		err = fmt.Errorf("TX  %s rejected due to low fees  provided fee %d calculated fee %d", txhash, provided_fee, calculated_fee)
		return err
	}

	if tx.TransactionType == transaction.SC_TX {
		if tx.SCDATA.Has(rpc.SCACTION, rpc.DataUint64) {
			if rpc.SC_INSTALL == rpc.SC_ACTION(tx.SCDATA.Value(rpc.SCACTION, rpc.DataUint64).(uint64)) {
				txid := tx.GetHash()
				if txid[0] < 0x80 || txid[31] < 0x80 { // last byte should be more than 0x80
					return fmt.Errorf("Invalid SCID ID,it must not start with 0x80")
				}
			}
		}
	}

	if err := chain.Verify_Transaction_NonCoinbase_CheckNonce_Tips(hf_version, tx, chain.Get_TIPS()); err != nil { // transaction verification failed
		logger.V(2).Error(err, "Incoming TX nonce verification failed", "txid", txhash, "stacktrace", globals.StackTrace(false))
		return fmt.Errorf("Incoming TX %s nonce verification failed, err %s", txhash, err)
	}

	if err := chain.Verify_Transaction_NonCoinbase(tx); err != nil {
		logger.V(2).Error(err, "Incoming TX could not be verified", "txid", txhash)
		return fmt.Errorf("Incoming TX %s could not be verified, err %s", txhash, err)
	}

	if chain.Mempool.Mempool_Add_TX(tx, 0) { // new tx come with 0 marker
		//rlog.Tracef(2, "Successfully added tx %s to pool", txhash)
		return nil
	} else {
		//rlog.Tracef(2, "TX %s rejected by pool by mempool", txhash)
		return fmt.Errorf("TX %s rejected by pool by mempool", txhash)
	}

}

// side blocks are blocks which lost the race the to become part
// of main chain,
// a block is a side block if it satisfies the following condition
// if no other block exists on this height before this
// this is part of consensus rule
// this is the topoheight of this block itself
func (chain *Blockchain) Isblock_SideBlock(blid crypto.Hash) bool {
	block_topoheight := chain.Load_Block_Topological_order(blid)
	if block_topoheight >= 0 {
		return false
	}

	return true
}

// todo optimize/ run more checks
func (chain *Blockchain) isblock_SideBlock_internal(blid crypto.Hash, block_topoheight int64, block_height int64) (result bool) {
	if block_topoheight == 0 { // genesis cannot be side block
		return false
	}

	toporecord, err := chain.Store.Topo_store.Read(block_topoheight - 1)
	if err != nil {
		panic("Could not load block from previous order")
	}
	if block_height == toporecord.Height { // lost race (or byzantine behaviour)
		return true
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
	if err = tx.Deserialize(tx_bytes); err != nil {
		return
	}

	blids_list := chain.Find_Blocks_Height_Range(int64(tx.Height+1), int64(tx.Height+1)+2*TX_VALIDITY_HEIGHT)

	var exist_list []crypto.Hash

	for _, blid := range blids_list {
		bl, err := chain.Load_BL_FROM_ID(blid)
		if err != nil {
			return
		}

		for _, bltxhash := range bl.Tx_hashes {
			if bltxhash == txhash {
				exist_list = append(exist_list, blid)
			}
		}
	}

	for _, blid := range exist_list {
		valid_blid = blid
		valid = true

	}

	return
}

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

// this function will rewind the chain from the topo height one block at a time
// this function also runs the client protocol in reverse and also deletes the block from the storage
func (chain *Blockchain) Rewind_Chain(rewind_count int) (result bool) {
	defer chain.Initialise_Chain_From_DB()

	chain.Lock()
	defer chain.Unlock()

	if rewind_count == 0 {
		return
	}

	top_block_topo_index := chain.Load_TOPO_HEIGHT()
	rewinded := int64(0)

	for {
		r, err := chain.Store.Topo_store.Read(top_block_topo_index - rewinded)
		if err != nil {
			panic(err)
		}

		if top_block_topo_index-rewinded < 1 || rewinded >= int64(rewind_count) {
			break
		}

		if r.Height == 1 {
			break
		}
		rewinded++
	}

	for i := int64(0); i < rewinded; i++ {
		chain.Store.Topo_store.Clean(top_block_topo_index - i)
	}

	chain.MiniBlocks.PurgeHeight(0xffffffffffffff) // purge all miniblocks upto this height

	return true
}

// this is part of consensus rule, 2 tips cannot refer to different parents
func (chain *Blockchain) CheckDagStructure(tips []crypto.Hash) bool {
	if chain.Load_Height_for_BL_ID(tips[0]) <= 2 { //  before this we cannot complete checks
		return true
	}

	for i := range tips { // first make sure all the tips are at same height
		if chain.Load_Height_for_BL_ID(tips[0]) != chain.Load_Height_for_BL_ID(tips[i]) {
			return false
		}
	}

	if len(tips) == 2 && tips[0] == tips[1] {
		return false
	}

	switch len(tips) {
	case 1:
		past := chain.Get_Block_Past(tips[0])
		switch len(past) {
		case 1: // nothing to do here

		case 2:
			if chain.Load_Height_for_BL_ID(past[0]) != chain.Load_Height_for_BL_ID(past[1]) {
				return false
			}

			past0 := chain.Get_Block_Past(past[0])
			if len(past0) != 1 { //only 1 tip in past
				return false
			}
			past1 := chain.Get_Block_Past(past[1])
			if len(past1) != 1 { //only 1 tip in past
				fmt.Printf("checking tips %+v past1 failed %d for %s\n", tips, len(past0), tips[0])
				return false
			}

			if past0[0] != past1[0] { // avoid any tips which fail reachability test
				return false
			}

		}
	case 2: // lets make sure both tips originate from same parent
		pasttip0 := chain.Get_Block_Past(tips[0])
		if len(pasttip0) != 1 { //only 1 tip in past
			return false
		}
		pasttip1 := chain.Get_Block_Past(tips[1])
		if len(pasttip0) != len(pasttip1) {
			return false
		}
		if pasttip0[0] != pasttip1[0] { // avoid any tips which fail reachability test
			return false
		}

	default:
		return false

	}

	return true
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
	if len(blocks) != 1 { //  ideal blockchain case, it is a sync block
		return false
	}

	return true
}

// we will collect atleast 50 blocks  or till genesis
func (chain *Blockchain) get_ordered_past(tip crypto.Hash, tillheight int64) (order []crypto.Hash) {
	order = append(order, tip)
	current := tip
	for chain.Load_Height_for_BL_ID(current) > tillheight {
		past := chain.Get_Block_Past(current)

		switch len(past) {
		case 0: // we reached genesis return
			return

		case 1:
			order = append(order, past[0])
			current = past[0]
		case 2:
			if bytes.Compare(past[0][:], past[1][:]) < 0 {
				order = append(order, past[0], past[1])
			} else {
				order = append(order, past[1], past[0])
			}
			current = past[0]
		default:
			panic("data corruption")
		}
	}

	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return
}

// this will flip chain top, depending on which block has more work
// more worked block is normally identified in < 2 secs
func (chain *Blockchain) flip_top() {
	return
	chain.Lock()
	defer chain.Unlock()

	height := chain.Get_Height()
	var tips []crypto.Hash
	if len(chain.Get_TIPS()) <= 1 {
		return
	}
	// lets fill in the tips from miniblocks, list is already sorted
	if keys := chain.MiniBlocks.GetAllKeys(height + 1); len(keys) >= 1 {
		top_id := chain.Get_Top_ID()
		for _, key := range keys {
			mbls := chain.MiniBlocks.GetAllMiniBlocks(key)
			if len(mbls) < 1 {
				continue
			}
			mbl := mbls[0]
			tips = tips[:0]
			tip := convert_uint32_to_crypto_hash(mbl.Past[0])
			if ehash, ok := chain.ExpandMiniBlockTip(tip); ok {
				if ehash != top_id { // we need to flip top
					fix_commit_version, err := chain.ReadBlockSnapshotVersion(ehash)
					if err != nil {
						panic(err)
					}

					fix_bl, err := chain.Load_BL_FROM_ID(ehash)
					if err != nil {
						panic(err)
					}

					chain.Store.Topo_store.Write(int64(fix_bl.Height), ehash, fix_commit_version, int64(fix_bl.Height))

					//	fmt.Printf("flipped top from %s to %s\n", top_id, ehash)

					return
				}

			}
		}
	}

}
