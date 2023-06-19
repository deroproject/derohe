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

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/deroproject/derohe/block"
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
	"github.com/deroproject/graviton"
	"golang.org/x/xerrors"
)

/*import "bytes"
import "encoding/binary"

import "github.com/romana/rlog"

*/

// caches x of transactions validity
// it is always atomic
// the cache is txhash -> validity mapping
// if the entry exist, the tx is valid
// it stores special hash and first seen time
var transaction_valid_cache sync.Map

// this go routine continuously scans and cleans up the cache for expired entries
func clean_up_valid_cache() {
	current_time := time.Now()
	transaction_valid_cache.Range(func(k, value interface{}) bool {
		first_seen := value.(time.Time)
		if current_time.Sub(first_seen).Round(time.Second).Seconds() > 360 {
			transaction_valid_cache.Delete(k)
		}
		return true
	})
}

// Coinbase transactions need to verify registration
func (chain *Blockchain) Verify_Transaction_Coinbase(cbl *block.Complete_Block, minertx *transaction.Transaction) (err error) {
	if !minertx.IsCoinbase() { // transaction is not coinbase, return failed
		return fmt.Errorf("tx is not coinbase")
	}

	return nil // success comes last
}

// this checks the nonces of a tx agains the current chain state, this basically does a comparision of state trees in limited form
func (chain *Blockchain) Verify_Transaction_NonCoinbase_CheckNonce_Tips(hf_version int64, tx *transaction.Transaction, tips []crypto.Hash) (err error) {
	var tx_hash crypto.Hash
	defer func() { // safety so if anything wrong happens, verification fails
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recovered while verifying tx", "txid", tx_hash, "r", r, "stack", debug.Stack())
			err = fmt.Errorf("Stack Trace %s", debug.Stack())
		}
	}()
	tx_hash = tx.GetHash()

	if tx.TransactionType == transaction.REGISTRATION { // all other tx must be checked
		return nil
	}

	if len(tips) < 1 {
		return fmt.Errorf("no tips provided, cannot verify")
	}

	tips_string := tx_hash.String()
	for _, tip := range tips {
		tips_string += fmt.Sprintf("%s", tip.String())
	}
	if _, found := chain.cache_IsNonceValidTips.Get(tips_string); found {
		return nil
	}

	// transaction needs to be expanded. this expansion needs  balance state
	version, err := chain.ReadBlockSnapshotVersion(tx.BLID)
	if err != nil {
		return err
	}

	ss_tx, err := chain.Store.Balance_store.LoadSnapshot(version)
	if err != nil {
		return err
	}

	var tx_balance_tree *graviton.Tree
	if tx_balance_tree, err = ss_tx.GetTree(config.BALANCE_TREE); err != nil {
		return err
	}

	if tx_balance_tree == nil {
		return fmt.Errorf("mentioned balance tree not found, cannot verify TX")
	}

	// now we must solve the tips, against which the nonces will be verified
	for _, tip := range tips {
		var tip_balance_tree *graviton.Tree

		version, err := chain.ReadBlockSnapshotVersion(tip)
		if err != nil {
			return err
		}
		ss_tip, err := chain.Store.Balance_store.LoadSnapshot(version)
		if err != nil {
			return err
		}

		if tip_balance_tree, err = ss_tip.GetTree(config.BALANCE_TREE); err != nil {
			return err
		}

		if tip_balance_tree == nil {
			return fmt.Errorf("mentioned tip  balance tree not found, cannot verify TX")
		}

		for t := range tx.Payloads {
			parity := tx.Payloads[t].Proof.Parity()

			var tip_tree, tx_tree *graviton.Tree
			if tx.Payloads[t].SCID.IsZero() { // choose whether we use main tree or sc tree
				tip_tree = tip_balance_tree
				tx_tree = tx_balance_tree
			} else {
				if tip_tree, err = ss_tip.GetTree(string(tx.Payloads[t].SCID[:])); err != nil {
					return err
				}

				if tx_tree, err = ss_tx.GetTree(string(tx.Payloads[t].SCID[:])); err != nil {
					return err
				}
			}

			for i := 0; i < int(tx.Payloads[t].Statement.RingSize); i++ {
				if (i%2 == 0) != parity { // this condition is well thought out and works good enough
					continue
				}
				key_pointer := tx.Payloads[t].Statement.Publickeylist_pointers[i*int(tx.Payloads[t].Statement.Bytes_per_publickey) : (i+1)*int(tx.Payloads[t].Statement.Bytes_per_publickey)]
				_, key_compressed, tx_balance_serialized, err := tx_tree.GetKeyValueFromHash(key_pointer)
				if err != nil && tx.Payloads[t].SCID.IsZero() {
					return err
				}
				if err != nil && xerrors.Is(err, graviton.ErrNotFound) && !tx.Payloads[t].SCID.IsZero() { // SC used a ring member not yet part
					continue
				}

				var tx_nb, tip_nb crypto.NonceBalance
				tx_nb.UnmarshalNonce(tx_balance_serialized)

				_, _, tip_balance_serialized, err := tip_tree.GetKeyValueFromKey(key_compressed)

				if err != nil && xerrors.Is(err, graviton.ErrNotFound) {
					continue
				}
				if err != nil {
					return err
				}
				tip_nb.UnmarshalNonce(tip_balance_serialized)

				//fmt.Printf("tx nonce %d  tip nonce %d\n", tx_nb.NonceHeight, tip_nb.NonceHeight)
				if tip_nb.NonceHeight > tx_nb.NonceHeight {
					addr, err1 := rpc.NewAddressFromCompressedKeys(key_compressed)
					if err1 != nil {
						panic(err1)
					}
					addr.Mainnet = globals.IsMainnet()
					return fmt.Errorf("Invalid Nonce, not usable, expected %d actual %d address %s", tip_nb.NonceHeight, tx_nb.NonceHeight, addr.String())
				}
			}
		}
	}

	if chain.cache_enabled {
		chain.cache_IsNonceValidTips.Add(tips_string, true) // set in cache
	}
	return nil
}

func (chain *Blockchain) Verify_Transaction_NonCoinbase(tx *transaction.Transaction) (err error) {
	return chain.verify_Transaction_NonCoinbase_internal(false, tx)
}
func (chain *Blockchain) Expand_Transaction_NonCoinbase(tx *transaction.Transaction) (err error) {
	return chain.verify_Transaction_NonCoinbase_internal(true, tx)
}

// all non miner tx must be non-coinbase tx
// each check is placed in a separate  block of code, to avoid ambigous code or faulty checks
// all check are placed and not within individual functions ( so as we cannot skip a check )
// This function verifies tx fully, means all checks,
// if the transaction has passed the check it can be added to mempool, relayed or added to blockchain
// the transaction has already been deserialized thats it
// It also expands the transactions, using the repective state trie
func (chain *Blockchain) verify_Transaction_NonCoinbase_internal(skip_proof bool, tx *transaction.Transaction) (err error) {

	var tx_hash crypto.Hash
	defer func() { // safety so if anything wrong happens, verification fails
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recovered while verifying tx", "txid", tx_hash, "r", r, "stack", debug.Stack())
			err = fmt.Errorf("Stack Trace %s", debug.Stack())
		}
	}()

	if tx.Version != 1 {
		return fmt.Errorf("TX should be version 1")
	}

	tx_hash = tx.GetHash()

	if tx.TransactionType == transaction.REGISTRATION {
		if _, ok := transaction_valid_cache.Load(tx_hash); ok {
			return nil //logger.Infof("Found in cache %s ",tx_hash)
		} else {
			//logger.Infof("TX not found in cache %s len %d ",tx_hash, len(tmp_buffer))
		}

		if tx.IsRegistrationValid() {
			if chain.cache_enabled {
				transaction_valid_cache.Store(tx_hash, time.Now()) // signature got verified, cache it
			}
			return nil
		}
		return fmt.Errorf("Registration has invalid signature")
	}

	// currently we allow following types of transaction
	if !(tx.TransactionType == transaction.NORMAL || tx.TransactionType == transaction.SC_TX || tx.TransactionType == transaction.BURN_TX) {
		return fmt.Errorf("Unknown transaction type")
	}

	if tx.TransactionType == transaction.BURN_TX {
		if tx.Value == 0 {
			return fmt.Errorf("Burn Value cannot be zero")
		}
	}

	// avoid some bugs lurking elsewhere
	if tx.Height != uint64(int64(tx.Height)) {
		return fmt.Errorf("invalid tx height")
	}

	if len(tx.Payloads) < 1 {
		return fmt.Errorf("tx must have at least one payload")
	}

	{ // we can not deduct fees, if no base, so make sure base is there
		// this restriction should be lifted under suitable conditions
		has_base := false
		for i := range tx.Payloads {
			if tx.Payloads[i].SCID.IsZero() {
				has_base = true
			}
		}
		if !has_base {
			return fmt.Errorf("tx does not contains base")
		}
	}
	for t := range tx.Payloads {
		if tx.Payloads[t].Statement.Roothash != tx.Payloads[0].Statement.Roothash {
			return fmt.Errorf("Roothash corrupted")
		}
	}

	for t := range tx.Payloads {
		// check sanity
		if tx.Payloads[t].Statement.RingSize != uint64(len(tx.Payloads[t].Statement.Publickeylist_pointers)/int(tx.Payloads[t].Statement.Bytes_per_publickey)) {
			return fmt.Errorf("corrupted key pointers ringsize")
		}

		if tx.Payloads[t].Statement.RingSize < 2 { // ring size minimum 2
			return fmt.Errorf("RingSize for %d statement cannot be less than 2 actual %d", t, tx.Payloads[t].Statement.RingSize)
		}

		if tx.Payloads[t].Statement.RingSize > 128 { // ring size current limited to 128
			return fmt.Errorf("RingSize for %d statement cannot be more than 128.Actual %d", t, tx.Payloads[t].Statement.RingSize)
		}

		if !crypto.IsPowerOf2(len(tx.Payloads[t].Statement.Publickeylist_pointers) / int(tx.Payloads[t].Statement.Bytes_per_publickey)) {
			return fmt.Errorf("corrupted key pointers")
		}

		// check duplicate ring members within the tx
		{
			key_map := map[string]bool{}
			for i := 0; i < int(tx.Payloads[t].Statement.RingSize); i++ {
				key_map[string(tx.Payloads[t].Statement.Publickeylist_pointers[i*int(tx.Payloads[t].Statement.Bytes_per_publickey):(i+1)*int(tx.Payloads[t].Statement.Bytes_per_publickey)])] = true
			}
			if len(key_map) != int(tx.Payloads[t].Statement.RingSize) {
				return fmt.Errorf("key_map does not contain ringsize members, ringsize %d , bytesperkey %d data %x", tx.Payloads[t].Statement.RingSize, tx.Payloads[t].Statement.Bytes_per_publickey, tx.Payloads[t].Statement.Publickeylist_pointers[:])
			}
		}
		tx.Payloads[t].Statement.CLn = tx.Payloads[t].Statement.CLn[:0]
		tx.Payloads[t].Statement.CRn = tx.Payloads[t].Statement.CRn[:0]
	}

	// transaction needs to be expanded. this expansion needs  balance state
	version, err := chain.ReadBlockSnapshotVersion(tx.BLID)
	if err != nil {
		return err
	}
	hash, err := chain.Load_Merkle_Hash(version)
	if err != nil {
		return err
	}

	if hash != tx.Payloads[0].Statement.Roothash {
		return fmt.Errorf("Tx statement roothash mismatch ref blid %x expected %x actual %x", tx.BLID, tx.Payloads[0].Statement.Roothash, hash[:])
	}
	// we have found the balance tree with which it was built now lets verify

	ss, err := chain.Store.Balance_store.LoadSnapshot(version)
	if err != nil {
		return err
	}

	var balance_tree *graviton.Tree
	if balance_tree, err = ss.GetTree(config.BALANCE_TREE); err != nil {
		return err
	}

	if balance_tree == nil {
		return fmt.Errorf("mentioned balance tree not found, cannot verify TX")
	}

	//logger.Infof("dTX  state tree has been found")

	trees := map[crypto.Hash]*graviton.Tree{}

	var zerohash crypto.Hash
	trees[zerohash] = balance_tree // initialize main tree by default

	for t := range tx.Payloads {
		tx.Payloads[t].Statement.Publickeylist_compressed = tx.Payloads[t].Statement.Publickeylist_compressed[:0]
		tx.Payloads[t].Statement.Publickeylist = tx.Payloads[t].Statement.Publickeylist[:0]

		var tree *graviton.Tree

		if _, ok := trees[tx.Payloads[t].SCID]; ok {
			tree = trees[tx.Payloads[t].SCID]
		} else {

			//	fmt.Printf("SCID loading %s tree\n", tx.Payloads[t].SCID)
			tree, _ = ss.GetTree(string(tx.Payloads[t].SCID[:]))
			trees[tx.Payloads[t].SCID] = tree
		}

		// now lets calculate CLn and CRn
		for i := 0; i < int(tx.Payloads[t].Statement.RingSize); i++ {
			key_pointer := tx.Payloads[t].Statement.Publickeylist_pointers[i*int(tx.Payloads[t].Statement.Bytes_per_publickey) : (i+1)*int(tx.Payloads[t].Statement.Bytes_per_publickey)]
			_, key_compressed, balance_serialized, err := tree.GetKeyValueFromHash(key_pointer)

			// if destination address could be found be found in sc balance tree, assume its zero balance
			needs_init := false
			if err != nil && !tx.Payloads[t].SCID.IsZero() {
				if xerrors.Is(err, graviton.ErrNotFound) { // if the address is not found, lookup in main tree
					_, key_compressed, _, err = balance_tree.GetKeyValueFromHash(key_pointer)
					if err != nil {
						return fmt.Errorf("balance not obtained err %s\n", err)
					}
					needs_init = true
				}
			}
			if err != nil {
				return fmt.Errorf("balance not obtained err %s\n", err)
			}

			// decode public key and expand
			{
				var p bn256.G1
				var pcopy [33]byte
				copy(pcopy[:], key_compressed)
				if err = p.DecodeCompressed(key_compressed[:]); err != nil {
					return fmt.Errorf("key %d could not be decompressed", i)
				}
				tx.Payloads[t].Statement.Publickeylist_compressed = append(tx.Payloads[t].Statement.Publickeylist_compressed, pcopy)
				tx.Payloads[t].Statement.Publickeylist = append(tx.Payloads[t].Statement.Publickeylist, &p)

				if needs_init {
					var nb crypto.NonceBalance
					nb.Balance = crypto.ConstructElGamal(&p, crypto.ElGamal_BASE_G) // init zero balance
					balance_serialized = nb.Serialize()
				}
			}

			var ll, rr bn256.G1
			nb := new(crypto.NonceBalance).Deserialize(balance_serialized)
			ebalance := nb.Balance

			ll.Add(ebalance.Left, tx.Payloads[t].Statement.C[i])
			tx.Payloads[t].Statement.CLn = append(tx.Payloads[t].Statement.CLn, &ll)
			rr.Add(ebalance.Right, tx.Payloads[t].Statement.D)
			tx.Payloads[t].Statement.CRn = append(tx.Payloads[t].Statement.CRn, &rr)

			// prepare for another sub transaction
			echanges := crypto.ConstructElGamal(tx.Payloads[t].Statement.C[i], tx.Payloads[t].Statement.D)
			nb = new(crypto.NonceBalance).Deserialize(balance_serialized)
			nb.Balance = nb.Balance.Add(echanges)    // homomorphic addition of changes
			tree.Put(key_compressed, nb.Serialize()) // reserialize and store temporarily, tree will be discarded after verification

		}
	}

	if _, ok := transaction_valid_cache.Load(tx_hash); ok {
		logger.V(2).Info("Found in cache, skipping verification", "txid", tx_hash)
		return nil
	} else {
		//logger.Infof("TX not found in cache %s len %d ",tx_hash, len(tmp_buffer))
	}

	if skip_proof {
		return nil
	}

	// at this point TX has been completely expanded, verify the tx statement
	scid_map := map[crypto.Hash]int{}
	for t := range tx.Payloads {

		index := scid_map[tx.Payloads[t].SCID]
		if !tx.Payloads[t].Proof.Verify(tx.Payloads[t].SCID, index, &tx.Payloads[t].Statement, tx.GetHash(), tx.Payloads[t].BurnValue) {
			//			fmt.Printf("Statement %+v\n", tx.Payloads[t].Statement)
			//			fmt.Printf("Proof %+v\n", tx.Payloads[t].Proof)

			return fmt.Errorf("transaction statement %d verification failed", t)
		}

		scid_map[tx.Payloads[t].SCID] = scid_map[tx.Payloads[t].SCID] + 1 // increment scid counter
	}

	// these transactions are done
	if tx.TransactionType == transaction.NORMAL || tx.TransactionType == transaction.BURN_TX || tx.TransactionType == transaction.SC_TX {
		if chain.cache_enabled {
			transaction_valid_cache.Store(tx_hash, time.Now()) // signature got verified, cache it
		}

		return nil
	}

	return nil

}
