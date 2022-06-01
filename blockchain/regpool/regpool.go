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

package regpool

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/stratumfarm/derohe/cryptography/crypto"
	"github.com/stratumfarm/derohe/globals"
	"github.com/stratumfarm/derohe/metrics"
	"github.com/stratumfarm/derohe/transaction"
)

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

// at this point in time, this is an ultrafast written regpool,
// it will not scale for more than 10000 transactions  but is good enough for now
// we can always come back and rewrite it
// NOTE: the pool is now persistant
type Regpool struct {
	txs           sync.Map            //map[crypto.Hash]*regpool_object
	address_map   sync.Map            //map[crypto.Hash]bool // contains key images of all txs
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
type regpool_object struct {
	Tx         *transaction.Transaction
	Added      uint64 // time in epoch format
	Height     uint64 //  at which height the tx unlocks in the regpool
	Relayed    int    // relayed count
	RelayedAt  int64  // when was tx last relayed
	Size       uint64 // size in bytes of the TX
	FEEperBYTE uint64 // fee per byte
}

var loggerpool logr.Logger

// marshal object as json
func (obj *regpool_object) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Tx        string `json:"tx"` // hex encoding
		Added     uint64 `json:"added"`
		Height    uint64 `json:"height"`
		Relayed   int    `json:"relayed"`
		RelayedAt int64  `json:"relayedat"`
	}{
		Tx:        hex.EncodeToString(obj.Tx.Serialize()),
		Added:     obj.Added,
		Height:    obj.Height,
		Relayed:   obj.Relayed,
		RelayedAt: obj.RelayedAt,
	})
}

// unmarshal object from json encoding
func (obj *regpool_object) UnmarshalJSON(data []byte) error {
	aux := &struct {
		Tx        string `json:"tx"`
		Added     uint64 `json:"added"`
		Height    uint64 `json:"height"`
		Relayed   int    `json:"relayed"`
		RelayedAt int64  `json:"relayedat"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	obj.Added = aux.Added
	obj.Height = aux.Height
	obj.Relayed = aux.Relayed
	obj.RelayedAt = aux.RelayedAt

	tx_bytes, err := hex.DecodeString(aux.Tx)
	if err != nil {
		return err
	}
	obj.Size = uint64(len(tx_bytes))

	obj.Tx = &transaction.Transaction{}
	err = obj.Tx.Deserialize(tx_bytes)

	if err == nil {
		obj.FEEperBYTE = 0
	}
	return err
}

func Init_Regpool(params map[string]interface{}) (*Regpool, error) {
	var regpool Regpool
	//regpool.chain = params["chain"].(*Blockchain)

	loggerpool = globals.Logger.WithName("REGPOOL") // all components must use this logger
	loggerpool.Info("Regpool started")
	atomic.AddUint32(&globals.Subsystem_Active, 1) // increment subsystem

	regpool.Exit_Mutex = make(chan bool)

	metrics.Set.GetOrCreateGauge("regpool_count", func() float64 {
		count := float64(0)
		regpool.txs.Range(func(k, value interface{}) bool {
			count++
			return true
		})
		return count
	})

	// initialize maps
	//regpool.txs = map[crypto.Hash]*regpool_object{}
	//regpool.address_map = map[crypto.Hash]bool{}

	//TODO load any trasactions saved at previous exit

	return &regpool, nil
}

// this is created per incoming block and then discarded
// This does not require shutting down and will be garbage collected automatically
//func Init_Block_Regpool(params map[string]interface{}) (*Regpool, error) {
//	var regpool Regpool
//	return &regpool, nil
//}

func (pool *Regpool) HouseKeeping(height uint64, Verifier func(*transaction.Transaction) bool) {
	pool.height = height

	// this code is executed in conditions where a registered user tries to register again
	var delete_list []crypto.Hash

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		v := value.(*regpool_object)

		if !Verifier(v.Tx) { // this tx  user has already registered
			delete_list = append(delete_list, txhash)
		}
		return true
	})

	for i := range delete_list {
		pool.Regpool_Delete_TX(delete_list[i])
	}
}

func (pool *Regpool) Shutdown() {
	//TODO save regpool tx somewhere

	close(pool.Exit_Mutex) // stop relaying

	pool.Lock()
	defer pool.Unlock()

	loggerpool.Info("Regpool stopped")
	atomic.AddUint32(&globals.Subsystem_Active, ^uint32(0)) // this decrement 1 fom subsystem

}

// start pool monitoring for changes for some specific time
// this is required so as we can add or discard transactions while selecting work for mining
func (pool *Regpool) Monitor() {
	pool.Lock()
	pool.modified = false
	pool.Unlock()
}

// return whether pool contents have changed
func (pool *Regpool) HasChanged() (result bool) {
	pool.Lock()
	result = pool.modified
	pool.Unlock()
	return
}

// a tx should only be added to pool after verification is complete
func (pool *Regpool) Regpool_Add_TX(tx *transaction.Transaction, Height uint64) (result bool) {
	result = false
	pool.Lock()
	defer pool.Unlock()

	if !tx.IsRegistration() {
		return false
	}

	var object regpool_object

	if pool.Regpool_Address_Present(tx.MinerAddress) {
		//   loggerpool.Infof("Rejecting TX, since address already has registration information")
		return false
	}

	tx_hash := crypto.Hash(tx.GetHash())

	// check if tx already exists, skip it
	if _, ok := pool.txs.Load(tx_hash); ok {
		//rlog.Debugf("Pool already contains %s, skipping", tx_hash)
		return false
	}

	if !tx.IsRegistrationValid() {
		return false
	}

	// add all the key images to check double spend attack within the pool
	//TODO
	//	for i := 0; i < len(tx.Vin); i++ {
	//		pool.address_map.Store(tx.Vin[i].(transaction.Txin_to_key).K_image,true) // add element to map for next check
	//	}

	pool.address_map.Store(tx.MinerAddress, true)

	// we are here means we can add it to pool
	object.Tx = tx
	object.Height = Height
	object.Added = uint64(time.Now().UTC().Unix())

	object.Size = uint64(len(tx.Serialize()))

	pool.txs.Store(tx_hash, &object)
	pool.modified = true // pool has been modified

	//pool.sort_list() // sort and update pool list

	return true
}

// check whether a tx exists in the pool
func (pool *Regpool) Regpool_TX_Exist(txid crypto.Hash) (result bool) {
	//pool.Lock()
	//defer pool.Unlock()

	if _, ok := pool.txs.Load(txid); ok {
		return true
	}
	return false
}

// check whether a keyimage exists in the pool
func (pool *Regpool) Regpool_Address_Present(ki [33]byte) (result bool) {
	//pool.Lock()
	//defer pool.Unlock()

	if _, ok := pool.address_map.Load(ki); ok {
		return true
	}
	return false
}

// delete specific tx from pool and return it
// if nil is returned Tx was not found in pool
func (pool *Regpool) Regpool_Delete_TX(txid crypto.Hash) (tx *transaction.Transaction) {
	//pool.Lock()
	//defer pool.Unlock()

	var ok bool
	var objecti interface{}

	// check if tx already exists, skip it
	if objecti, ok = pool.txs.Load(txid); !ok {
		//rlog.Warnf("Pool does NOT contain %s, returning nil", txid)
		return nil
	}

	// we reached here means, we have the tx remove it from our list, do maintainance cleapup and discard it
	object := objecti.(*regpool_object)
	tx = object.Tx
	pool.txs.Delete(txid)

	// remove all the key images
	//TODO
	//	for i := 0; i < len(object.Tx.Vin); i++ {
	//		pool.address_map.Delete(object.Tx.Vin[i].(transaction.Txin_to_key).K_image)
	//	}
	pool.address_map.Delete(tx.MinerAddress)

	//pool.sort_list()     // sort and update pool list
	pool.modified = true // pool has been modified
	return object.Tx     // return the tx
}

// get specific tx from mem pool without removing it
func (pool *Regpool) Regpool_Get_TX(txid crypto.Hash) (tx *transaction.Transaction) {
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
	object := objecti.(*regpool_object)

	return object.Tx
}

// return list of all txs in pool
func (pool *Regpool) Regpool_List_TX() []crypto.Hash {
	//	pool.Lock()
	//	defer pool.Unlock()

	var list []crypto.Hash

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		//v := value.(*regpool_object)
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

// print current regpool txs
// TODO add sorting
func (pool *Regpool) Regpool_Print() {
	pool.Lock()
	defer pool.Unlock()

	var klist []crypto.Hash
	var vlist []*regpool_object

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		v := value.(*regpool_object)
		//objects = append(objects, *v)
		klist = append(klist, txhash)
		vlist = append(vlist, v)

		return true
	})

	loggerpool.Info(fmt.Sprintf("Total TX in regpool = %d\n", len(klist)))
	loggerpool.Info(fmt.Sprintf("%20s  %14s %7s %7s %6s %32s\n", "Added", "Last Relayed", "Relayed", "Size", "Height", "TXID"))

	for i := range klist {
		k := klist[i]
		v := vlist[i]
		loggerpool.Info(fmt.Sprintf("%20s  %14s %7d %7d %6d %32s\n", time.Unix(int64(v.Added), 0).UTC().Format(time.RFC3339), time.Duration(v.RelayedAt)*time.Second, v.Relayed,
			len(v.Tx.Serialize()), v.Height, k))
	}
}

// flush regpool
func (pool *Regpool) Regpool_flush() {
	var list []crypto.Hash

	pool.txs.Range(func(k, value interface{}) bool {
		txhash := k.(crypto.Hash)
		//v := value.(*regpool_object)
		//objects = append(objects, *v)
		list = append(list, txhash)
		return true
	})

	loggerpool.Info("Total TX in regpool", "txcount", len(list))
	loggerpool.Info("Flushing regpool")

	for i := range list {
		pool.Regpool_Delete_TX(list[i])
	}
}
