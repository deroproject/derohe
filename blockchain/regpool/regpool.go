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

import "os"
import "fmt"
import "sync"
import "time"
import "sync/atomic"
import "path/filepath"
import "encoding/hex"
import "encoding/json"

import "github.com/romana/rlog"
import log "github.com/sirupsen/logrus"

import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/crypto"

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

	relayer        chan crypto.Hash // used for immediate relay
	P2P_TX_Relayer p2p_TX_Relayer   // actual pointer, setup by the dero daemon during runtime

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

var loggerpool *log.Entry

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
	err = obj.Tx.DeserializeHeader(tx_bytes)

	if err == nil {
		obj.FEEperBYTE = obj.Tx.Statement.Fees / obj.Size
	}
	return err
}

func Init_Regpool(params map[string]interface{}) (*Regpool, error) {
	var regpool Regpool
	//regpool.chain = params["chain"].(*Blockchain)

	loggerpool = globals.Logger.WithFields(log.Fields{"com": "REGPOOL"}) // all components must use this logger
	loggerpool.Infof("Regpool started")
	atomic.AddUint32(&globals.Subsystem_Active, 1) // increment subsystem

	regpool.relayer = make(chan crypto.Hash, 1024*10)
	regpool.Exit_Mutex = make(chan bool)

	// initialize maps
	//regpool.txs = map[crypto.Hash]*regpool_object{}
	//regpool.address_map = map[crypto.Hash]bool{}

	//TODO load any trasactions saved at previous exit

	regpool_file := filepath.Join(globals.GetDataDirectory(), "regpool.json")

	file, err := os.Open(regpool_file)
	if err != nil {
		loggerpool.Warnf("Error opening regpool data file %s err %s", regpool_file, err)
	} else {
		defer file.Close()

		var objects []regpool_object
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&objects)
		if err != nil {
			loggerpool.Warnf("Error unmarshalling regpool data err %s", err)
		} else { // successfully unmarshalled data, add it to regpool
			loggerpool.Debugf("Will try to load %d txs from regpool file", (len(objects)))
			for i := range objects {
				result := regpool.Regpool_Add_TX(objects[i].Tx, 0)
				if result { // setup time
					//regpool.txs[objects[i].Tx.GetHash()] = &objects[i] // setup time and other artifacts
					regpool.txs.Store(objects[i].Tx.GetHash(), &objects[i])
				}
			}
		}
	}

	go regpool.Relayer_and_Cleaner()

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

	regpool_file := filepath.Join(globals.GetDataDirectory(), "regpool.json")

	// collect all txs in pool and serialize them and store them
	var objects []regpool_object

	pool.txs.Range(func(k, value interface{}) bool {
		v := value.(*regpool_object)
		objects = append(objects, *v)
		return true
	})

	/*for _, v := range pool.txs {
		objects = append(objects, *v)
	}*/

	var file, err = os.Create(regpool_file)
	if err == nil {
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "\t")
		err = encoder.Encode(objects)

		if err != nil {
			loggerpool.Warnf("Error marshaling regpool data err %s", err)
		}

	} else {
		loggerpool.Warnf("Error creating new file to store regpool data file %s err %s", regpool_file, err)
	}

	loggerpool.Infof("Succesfully saved %d txs to file", (len(objects)))

	loggerpool.Infof("Regpool stopped")
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
	pool.relayer <- tx_hash
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
		rlog.Warnf("Pool does NOT contain %s, returning nil", txid)
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

	fmt.Printf("Total TX in regpool = %d\n", len(klist))
	fmt.Printf("%20s  %14s %7s %7s %6s %32s\n", "Added", "Last Relayed", "Relayed", "Size", "Height", "TXID")

	for i := range klist {
		k := klist[i]
		v := vlist[i]
		fmt.Printf("%20s  %14s %7d %7d %6d %32s\n", time.Unix(int64(v.Added), 0).UTC().Format(time.RFC3339), time.Duration(v.RelayedAt)*time.Second, v.Relayed,
			len(v.Tx.Serialize()), v.Height, k)
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

	fmt.Printf("Total TX in regpool = %d \n", len(list))
	fmt.Printf("Flushing regpool \n")

	for i := range list {
		pool.Regpool_Delete_TX(list[i])
	}
}

type p2p_TX_Relayer func(*transaction.Transaction, uint64) int // function type, exported in p2p but cannot use due to cyclic dependency

// this tx relayer keeps on relaying tx and cleaning regpool
// if a tx has been relayed less than 10 peers, tx relaying is agressive
// otherwise the tx are relayed every 30 minutes, till it has been relayed to 20
// then the tx is relayed every 3 hours, just in case
func (pool *Regpool) Relayer_and_Cleaner() {

	for {

		select {
		case txid := <-pool.relayer:
			if objecti, ok := pool.txs.Load(txid); !ok {
				break
			} else {
				// we reached here means, we have the tx, return the pointer back
				object := objecti.(*regpool_object)
				if pool.P2P_TX_Relayer != nil {
					relayed_count := pool.P2P_TX_Relayer(object.Tx, 0)
					//relayed_count := 0
					if relayed_count > 0 {
						object.Relayed += relayed_count
						rlog.Tracef(1, "Relayed %s to %d peers (%d %d)", txid, relayed_count, object.Relayed, (time.Now().Unix() - object.RelayedAt))
						object.RelayedAt = time.Now().Unix()
					}
				}
			}
		case <-pool.Exit_Mutex:
			return
		case <-time.After(400 * time.Millisecond):

		}

		pool.txs.Range(func(ktmp, value interface{}) bool {
			k := ktmp.(crypto.Hash)
			v := value.(*regpool_object)

			select { // exit fast of possible
			case <-pool.Exit_Mutex:
				return false
			default:
			}

			if v.Relayed < 10 || // relay it now
				(v.Relayed >= 4 && v.Relayed <= 20 && (time.Now().Unix()-v.RelayedAt) > 5) || // relay it now
				(time.Now().Unix()-v.RelayedAt) > 4 {
				if pool.P2P_TX_Relayer != nil {

					relayed_count := pool.P2P_TX_Relayer(v.Tx, 0)
					//relayed_count := 0
					if relayed_count > 0 {
						v.Relayed += relayed_count

						//loggerpool.Debugf("%d  %d\n",time.Now().Unix(), v.RelayedAt)
						rlog.Tracef(1, "Relayed %s to %d peers (%d %d)", k, relayed_count, v.Relayed, (time.Now().Unix() - v.RelayedAt))
						v.RelayedAt = time.Now().Unix()
						//loggerpool.Debugf("%d  %d",time.Now().Unix(), v.RelayedAt)
					}
				}
			}

			return true
		})

		// loggerpool.Warnf("send Pool lock released")
		//pool.Unlock()
	}
}
