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

package mempool

import "fmt"
import "sync"
import "sort"
import "time"
import "sync/atomic"

import "github.com/go-logr/logr"

import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/metrics"
import "github.com/deroproject/derohe/cryptography/crypto"

// this is only used for sorting and nothing else
type TX_Sorting_struct struct {
	FeesPerByte uint64      // this is fees per byte
	Hash        crypto.Hash // transaction hash
	Size        uint64      // transaction size
}

// NOTE: do NOT consider this code as useless, as it is used to avooid double spending attacks within the block and within the pool
// let me explain, since we are a state machine, we add block to our blockchain
// so, if a double spending attack comes, 2 transactions with same inputs, we reject one of them
// the algo is documented somewhere else  which explains the entire process

// at this point in time, this is an ultrafast written mempool,
// it will not scale for more than 10000 transactions  but is good enough for now
// we can always come back and rewrite it
// NOTE: the pool is now persistant
type Mempool struct {
	txs           sync.Map            //map[crypto.Hash]*mempool_object
	nonces        sync.Map            //map[crypto.Hash]bool // contains key images of all txs
	sorted_by_fee []crypto.Hash       // contains txids sorted by fees
	sorted        []TX_Sorting_struct // contains TX sorting information, so as new block can be forged easily
	modified      bool                // used to monitor whethel mem pool contents have changed,
	height        uint64              // track blockchain height

	// global variable , but don't see it utilisation here except fot tx verification
	//chain *Blockchain
	Exit_Mutex chan bool

	sync.Mutex
}

// this object is serialized  and deserialized
type mempool_object struct {
	Tx         *transaction.Transaction
	Added      uint64 // time in epoch format
	Height     uint64 //  at which height the tx unlocks in the mempool
	Size       uint64 // size in bytes of the TX
	FEEperBYTE uint64 // fee per byte
}

var loggerpool logr.Logger

func Init_Mempool(params map[string]interface{}) (*Mempool, error) {
	var mempool Mempool
	//mempool.chain = params["chain"].(*Blockchain)

	loggerpool = globals.Logger.WithName("MEMPOOL") // all components must use this logger
	loggerpool.Info("Mempool started")
	atomic.AddUint32(&globals.Subsystem_Active, 1) // increment subsystem

	mempool.Exit_Mutex = make(chan bool)

	metrics.Set.GetOrCreateGauge("mempool_count", func() float64 {
		count := float64(0)
		mempool.txs.Range(func(k, value interface{}) bool {
			count++
			return true
		})
		return count
	})

	return &mempool, nil
}

func (pool *Mempool) HouseKeeping(height uint64) {
	pool.height = height

	// this code is executed in  conditions which are as follows
	// we have to purge old txs which can no longer be mined
	var delete_list []crypto.Hash

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		v := value.(*mempool_object)
		if height >= (v.Tx.Height) { // remove all txs
			delete_list = append(delete_list, txhash)
		}
		return true
	})

	for i := range delete_list {
		metrics.Set.GetOrCreateCounter("mempool_discarded_total").Inc()
		pool.Mempool_Delete_TX(delete_list[i])
	}
}

func (pool *Mempool) Shutdown() {
	//TODO save mempool tx somewhere

	close(pool.Exit_Mutex) // stop relaying

	pool.Lock()
	defer pool.Unlock()

	loggerpool.Info("Mempool stopped")
	atomic.AddUint32(&globals.Subsystem_Active, ^uint32(0)) // this decrement 1 fom subsystem

}

// start pool monitoring for changes for some specific time
// this is required so as we can add or discard transactions while selecting work for mining
func (pool *Mempool) Monitor() {
	pool.Lock()
	pool.modified = false
	pool.Unlock()
}

// return whether pool contents have changed
func (pool *Mempool) HasChanged() (result bool) {
	pool.Lock()
	result = pool.modified
	pool.Unlock()
	return
}

// a tx should only be added to pool after verification is complete
func (pool *Mempool) Mempool_Add_TX(tx *transaction.Transaction, Height uint64) (result bool) {
	result = false
	pool.Lock()
	defer pool.Unlock()

	var object mempool_object
	tx_hash := crypto.Hash(tx.GetHash())

	dup_within_tx := map[crypto.Hash]bool{}

	for i := range tx.Payloads {
		if pool.Mempool_Nonce_Used(tx.Payloads[i].Proof.Nonce()) {
			return false
		}
		if _, ok := dup_within_tx[tx.Payloads[i].Proof.Nonce()]; ok {
			return false
		}
		dup_within_tx[tx.Payloads[i].Proof.Nonce()] = true
	}

	// check if tx already exists, skip it
	if _, ok := pool.txs.Load(tx_hash); ok {
		//rlog.Debugf("Pool already contains %s, skipping", tx_hash)
		return false
	}

	for i := range tx.Payloads {
		pool.nonces.Store(tx.Payloads[i].Proof.Nonce(), true)
	}

	// we are here means we can add it to pool
	object.Tx = tx
	object.Height = Height
	object.Added = uint64(time.Now().UTC().Unix())

	object.Size = uint64(len(tx.Serialize()))
	object.FEEperBYTE = tx.Fees() / object.Size

	pool.txs.Store(tx_hash, &object)
	pool.modified = true // pool has been modified

	//pool.sort_list() // sort and update pool list

	return true
}

// check whether a tx exists in the pool
func (pool *Mempool) Mempool_TX_Exist(txid crypto.Hash) (result bool) {
	//pool.Lock()
	//defer pool.Unlock()

	if _, ok := pool.txs.Load(txid); ok {
		return true
	}
	return false
}

// check whether a keyimage exists in the pool
func (pool *Mempool) Mempool_Nonce_Used(ki crypto.Hash) (result bool) {
	//pool.Lock()
	//defer pool.Unlock()

	if _, ok := pool.nonces.Load(ki); ok {
		return true
	}
	return false
}

// delete specific tx from pool and return it
// if nil is returned Tx was not found in pool
func (pool *Mempool) Mempool_Delete_TX(txid crypto.Hash) (tx *transaction.Transaction) {
	//pool.Lock()
	//defer pool.Unlock()

	var ok bool
	var objecti interface{}

	// check if tx already exists, skip it
	if objecti, ok = pool.txs.Load(txid); !ok {
		//		rlog.Warnf("Pool does NOT contain %s, returning nil", txid)
		return nil
	}

	// we reached here means, we have the tx remove it from our list, do maintainance cleapup and discard it
	object := objecti.(*mempool_object)
	tx = object.Tx
	pool.txs.Delete(txid)

	// remove all the key images
	//TODO
	//	for i := 0; i < len(object.Tx.Vin); i++ {
	//		pool.nonces.Delete(object.Tx.Vin[i].(transaction.Txin_to_key).K_image)
	//	}
	for i := range tx.Payloads {
		pool.nonces.Delete(tx.Payloads[i].Proof.Nonce())
	}

	//pool.sort_list()     // sort and update pool list
	pool.modified = true // pool has been modified
	return object.Tx     // return the tx
}

// get specific tx from mem pool without removing it
func (pool *Mempool) Mempool_Get_TX(txid crypto.Hash) (tx *transaction.Transaction) {
	//	pool.Lock()
	//	defer pool.Unlock()

	var ok bool
	var objecti interface{}

	if objecti, ok = pool.txs.Load(txid); !ok {
		//loggerpool.Warnf("Pool does NOT contain %s, returning nil", txid)
		return nil
	}

	// we reached here means, we have the tx, return the pointer back
	//object := pool.txs[txid]
	object := objecti.(*mempool_object)

	return object.Tx
}

// return list of all txs in pool
func (pool *Mempool) Mempool_List_TX() []crypto.Hash {
	//	pool.Lock()
	//	defer pool.Unlock()

	var list []crypto.Hash

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		//v := value.(*mempool_object)
		//objects = append(objects, *v)
		list = append(list, txhash)
		return true
	})

	//pool.sort_list() // sort and update pool list

	// list should be as big as spurce list
	//list := make([]crypto.Hash, len(pool.sorted_by_fee), len(pool.sorted_by_fee))
	//copy(list, pool.sorted_by_fee) // return list sorted by fees

	return list
}

// passes back sorting information and length information for easier new block forging
func (pool *Mempool) Mempool_List_TX_SortedInfo() []TX_Sorting_struct {
	//	pool.Lock()
	//	defer pool.Unlock()

	_, data := pool.sort_list() // sort and update pool list
	return data

	/*	// list should be as big as spurce list
		list := make([]TX_Sorting_struct, len(pool.sorted), len(pool.sorted))
		copy(list, pool.sorted) // return list sorted by fees

		return list
	*/
}

// print current mempool txs
// TODO add sorting
func (pool *Mempool) Mempool_Print() {
	pool.Lock()
	defer pool.Unlock()

	var klist []crypto.Hash
	var vlist []*mempool_object

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		v := value.(*mempool_object)
		//objects = append(objects, *v)
		klist = append(klist, txhash)
		vlist = append(vlist, v)

		return true
	})

	loggerpool.Info(fmt.Sprintf("Total TX in mempool = %d\n", len(klist)))
	loggerpool.Info(fmt.Sprintf("%20s  %7s %6s %32s\n", "Added", "Size", "Height", "TXID"))

	for i := range klist {
		k := klist[i]
		v := vlist[i]
		loggerpool.Info(fmt.Sprintf("%20s  %7d %6d %32s\n", time.Unix(int64(v.Added), 0).UTC().Format(time.RFC3339),
			len(v.Tx.Serialize()), v.Height, k))
	}
}

// flush mempool
func (pool *Mempool) Mempool_flush() {
	var list []crypto.Hash

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		//v := value.(*mempool_object)
		//objects = append(objects, *v)
		list = append(list, txhash)
		return true
	})

	loggerpool.Info("Total TX in mempool", "txcount", len(list))
	loggerpool.Info("Flushing mempool")
	for i := range list {
		pool.Mempool_Delete_TX(list[i])
	}
}

// sorts the pool internally
// this function assummes lock is already taken
// ??? if we  selecting transactions randomly, why to keep them sorted
func (pool *Mempool) sort_list() ([]crypto.Hash, []TX_Sorting_struct) {

	data := make([]TX_Sorting_struct, 0, 512) // we are rarely expectingmore than this entries in mempool
	// collect data from pool for sorting

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		v := value.(*mempool_object)
		if v.Height <= pool.height {
			data = append(data, TX_Sorting_struct{Hash: txhash, FeesPerByte: v.FEEperBYTE, Size: v.Size})
		}
		return true
	})

	// inverted comparision sort to do descending sort
	sort.SliceStable(data, func(i, j int) bool { return data[i].FeesPerByte > data[j].FeesPerByte })

	sorted_list := make([]crypto.Hash, 0, len(data))
	//pool.sorted_by_fee = pool.sorted_by_fee[:0] // empty old slice

	for i := range data {
		sorted_list = append(sorted_list, data[i].Hash)
	}
	//pool.sorted = data
	return sorted_list, data

}
