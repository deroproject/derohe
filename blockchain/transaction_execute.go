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

// this file implements  core execution of all changes to block chain homomorphically

import "fmt"
import "math/big"
import "golang.org/x/xerrors"

import "github.com/romana/rlog"

import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/dvm"
import "github.com/deroproject/graviton"

// convert bitcoin model to our, but skip initial 4 years of supply, so our total supply gets to 10.5 million
const RewardReductionInterval = 210000 * 600 / config.BLOCK_TIME // 210000 comes from bitcoin
const BaseReward = 50 * 100000 * config.BLOCK_TIME / 600         // convert bitcoin reward system to our block

// CalcBlockSubsidy returns the subsidy amount a block at the provided height
// should have. This is mainly used for determining how much the coinbase for
// newly generated blocks awards as well as validating the coinbase for blocks
// has the expected value.
//
// The subsidy is halved every SubsidyReductionInterval blocks.  Mathematically
// this is: baseSubsidy / 2^(height/SubsidyReductionInterval)
//
// At the target block generation rate for the main network, this is
// approximately every 4 years.
//

// basically out of of the bitcoin supply, we have wiped of initial interval ( this wipes of  10.5 million, so total remaining is around 10.5 million
func CalcBlockReward(height uint64) uint64 {
	return BaseReward >> ((height + RewardReductionInterval) / RewardReductionInterval)
}

// process the miner tx, giving fees, miner rewatd etc
func (chain *Blockchain) process_miner_transaction(tx transaction.Transaction, genesis bool, balance_tree *graviton.Tree, fees uint64, height uint64) {
	var acckey crypto.Point
	if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
		panic(err)
	}

	if genesis == true { // process premine ,register genesis block, dev key
		balance := crypto.ConstructElGamal(acckey.G1(), crypto.ElGamal_BASE_G) // init zero balance
		balance = balance.Plus(new(big.Int).SetUint64(tx.Value))               // add premine to users balance homomorphically
		balance_tree.Put(tx.MinerAddress[:], balance.Serialize())              // reserialize and store
		return
	}

	// general coin base transaction
	base_reward := CalcBlockReward(uint64(height))
	full_reward := base_reward + fees

	dev_reward := (full_reward * config.DEVSHARE) / 10000 // take % from reward
	miner_reward := full_reward - dev_reward              // it's value, do subtraction

	{ // giver miner reward
		balance_serialized, err := balance_tree.Get(tx.MinerAddress[:])
		if err != nil {
			panic(err)
		}
		balance := new(crypto.ElGamal).Deserialize(balance_serialized)
		balance = balance.Plus(new(big.Int).SetUint64(miner_reward)) // add miners reward to miners balance homomorphically
		balance_tree.Put(tx.MinerAddress[:], balance.Serialize())    // reserialize and store
	}

	{ // give devs reward
		balance_serialized, err := balance_tree.Get(chain.Dev_Address_Bytes[:])
		if err != nil {
			panic(err)
		}
		balance := new(crypto.ElGamal).Deserialize(balance_serialized)
		balance = balance.Plus(new(big.Int).SetUint64(dev_reward))        // add devs reward to devs balance homomorphically
		balance_tree.Put(chain.Dev_Address_Bytes[:], balance.Serialize()) // reserialize and store
	}

	return

}

// process the tx, giving fees, miner rewatd etc
// this should be atomic, either all should be done or none at all
func (chain *Blockchain) process_transaction(changed map[crypto.Hash]*graviton.Tree, tx transaction.Transaction, balance_tree *graviton.Tree) uint64 {

	//fmt.Printf("Processing/Executing transaction %s %s\n", tx.GetHash(), tx.TransactionType.String())
	switch tx.TransactionType {

	case transaction.REGISTRATION:
		if _, err := balance_tree.Get(tx.MinerAddress[:]); err != nil {
			if !xerrors.Is(err, graviton.ErrNotFound) { // any other err except not found panic
				panic(err)
			}
		} // address needs registration

		var acckey crypto.Point
		if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
			panic(err)
		}

		zerobalance := crypto.ConstructElGamal(acckey.G1(), crypto.ElGamal_BASE_G)
		zerobalance = zerobalance.Plus(new(big.Int).SetUint64(800000)) // add fix amount to every wallet to users balance for more testing

		balance_tree.Put(tx.MinerAddress[:], zerobalance.Serialize())

		return 0 // registration doesn't give any fees . why & how ?

	case transaction.BURN_TX, transaction.NORMAL, transaction.SC_TX: // burned amount is not added anywhere and thus lost forever

		for t := range tx.Payloads {
			var tree *graviton.Tree
			if tx.Payloads[t].SCID.IsZero() {
				tree = balance_tree
			} else {
				tree = changed[tx.Payloads[t].SCID]
			}
			for i := 0; i < int(tx.Payloads[t].Statement.RingSize); i++ {
				key_pointer := tx.Payloads[t].Statement.Publickeylist_pointers[i*int(tx.Payloads[t].Statement.Bytes_per_publickey) : (i+1)*int(tx.Payloads[t].Statement.Bytes_per_publickey)]
				if _, key_compressed, balance_serialized, err := tree.GetKeyValueFromHash(key_pointer); err == nil {

					balance := new(crypto.ElGamal).Deserialize(balance_serialized)
					echanges := crypto.ConstructElGamal(tx.Payloads[t].Statement.C[i], tx.Payloads[t].Statement.D)

					balance = balance.Add(echanges)               // homomorphic addition of changes
					tree.Put(key_compressed, balance.Serialize()) // reserialize and store
				} else {
					panic(err) // if balance could not be obtained panic ( we can never reach here, otherwise how tx got verified)
				}
			}
		}

		return tx.Fees()

	default:
		panic("unknown transaction, do not know how to process it")
		return 0
	}
}

type Tree_Wrapper struct {
	tree             *graviton.Tree
	entries          map[string][]byte
	leftover_balance uint64
	transfere        []dvm.TransferExternal
}

func (t *Tree_Wrapper) Get(key []byte) ([]byte, error) {
	if value, ok := t.entries[string(key)]; ok {
		return value, nil
	} else {
		return t.tree.Get(key)
	}
}

func (t *Tree_Wrapper) Put(key []byte, value []byte) error {
	t.entries[string(key)] = append([]byte{}, value...)
	return nil
}

// does additional processing for SC
func (chain *Blockchain) process_transaction_sc(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, bl_height, bl_topoheight uint64, blid crypto.Hash, tx transaction.Transaction, balance_tree *graviton.Tree, sc_tree *graviton.Tree) (gas uint64, err error) {

	if len(tx.SCDATA) == 0 {
		return tx.Fees(), nil
	}

	success := false
	w_balance_tree := &Tree_Wrapper{tree: balance_tree, entries: map[string][]byte{}}
	w_sc_tree := &Tree_Wrapper{tree: sc_tree, entries: map[string][]byte{}}

	_ = w_balance_tree

	var sc_data_tree *graviton.Tree // SC data tree
	var w_sc_data_tree *Tree_Wrapper

	txhash := tx.GetHash()
	scid := txhash

	defer func() {
		if success { // merge the trees

		}
	}()

	if !tx.SCDATA.Has(rpc.SCACTION, rpc.DataUint64) { //  tx doesn't have sc action
		//err = fmt.Errorf("no scid provided")
		return tx.Fees(), nil
	}

	action_code := rpc.SC_ACTION(tx.SCDATA.Value(rpc.SCACTION, rpc.DataUint64).(uint64))

	switch action_code {
	case rpc.SC_INSTALL: // request to install an SC
		if !tx.SCDATA.Has(rpc.SCCODE, rpc.DataString) { // but only it is present
			break
		}
		sc_code := tx.SCDATA.Value(rpc.SCCODE, rpc.DataString).(string)
		if sc_code == "" { // no code provided nothing to do
			err = fmt.Errorf("no code provided")
			break
		}

		// check whether sc can be parsed
		//var sc_parsed dvm.SmartContract
		pos := ""
		var sc dvm.SmartContract

		sc, pos, err = dvm.ParseSmartContract(sc_code)
		if err != nil {
			rlog.Warnf("error Parsing sc txid %s err %s pos %s\n", txhash, err, pos)
			break
		}

		meta := SC_META_DATA{Balance: tx.Value}

		if _, ok := sc.Functions["InitializePrivate"]; ok {
			meta.Type = 1
		}
		if sc_data_tree, err = ss.GetTree(string(scid[:])); err != nil {
			break
		} else {
			w_sc_data_tree = &Tree_Wrapper{tree: sc_data_tree, entries: map[string][]byte{}}
		}

		// install SC, should we check for sanity now, why or why not
		w_sc_data_tree.Put(SC_Code_Key(scid), dvm.Variable{Type: dvm.String, Value: sc_code}.MarshalBinaryPanic())

		w_sc_tree.Put(SC_Meta_Key(scid), meta.MarshalBinary())

		// at this point we must trigger the initialize call in the DVM
		//fmt.Printf("We must call the SC initialize function\n")

		if meta.Type == 1 { // if its a a private SC
			gas, err = chain.execute_sc_function(w_sc_tree, w_sc_data_tree, scid, bl_height, bl_topoheight, blid, tx, "InitializePrivate", 1)
		} else {
			gas, err = chain.execute_sc_function(w_sc_tree, w_sc_data_tree, scid, bl_height, bl_topoheight, blid, tx, "Initialize", 1)
		}

	case rpc.SC_CALL: // trigger a CALL
		if !tx.SCDATA.Has(rpc.SCID, rpc.DataHash) { // but only it is present
			err = fmt.Errorf("no scid provided")
			break
		}
		if !tx.SCDATA.Has("entrypoint", rpc.DataString) { // but only it is present
			err = fmt.Errorf("no entrypoint provided")
			break
		}

		scid = tx.SCDATA.Value(rpc.SCID, rpc.DataHash).(crypto.Hash)

		if _, err = w_sc_tree.Get(SC_Meta_Key(scid)); err != nil {
			err = fmt.Errorf("scid %s not installed", scid)
			return
		}

		if sc_data_tree, err = ss.GetTree(string(scid[:])); err != nil {

			return
		} else {
			w_sc_data_tree = &Tree_Wrapper{tree: sc_data_tree, entries: map[string][]byte{}}
		}

		entrypoint := tx.SCDATA.Value("entrypoint", rpc.DataString).(string)
		//fmt.Printf("We must call the SC %s function\n", entrypoint)

		gas, err = chain.execute_sc_function(w_sc_tree, w_sc_data_tree, scid, bl_height, bl_topoheight, blid, tx, entrypoint, 1)

	default: // unknown  what to do
		err = fmt.Errorf("unknown action what to do", scid)
		return
	}

	if err == nil { // we must commit the changes
		var data_tree *graviton.Tree
		var ok bool
		if data_tree, ok = cache[scid]; !ok {
			data_tree = w_sc_data_tree.tree
			cache[scid] = w_sc_data_tree.tree
		}

		// commit entire data to tree
		for k, v := range w_sc_data_tree.entries {
			//fmt.Printf("persisting %x %x\n", k, v)
			if err = data_tree.Put([]byte(k), v); err != nil {
				return
			}
		}

		for k, v := range w_sc_tree.entries { // these entries are only partial
			if err = sc_tree.Put([]byte(k), v); err != nil {
				return
			}
		}

		// at this point, settle the balances, how ??
		var meta_bytes []byte
		meta_bytes, err = w_sc_tree.Get(SC_Meta_Key(scid))
		if err != nil {
			return
		}

		var meta SC_META_DATA // the meta contains the link to the SC bytes
		if err = meta.UnmarshalBinary(meta_bytes); err != nil {
			return
		}
		meta.Balance = w_sc_data_tree.leftover_balance

		//fmt.Printf("SC %s balance %d\n", scid, w_sc_data_tree.leftover_balance)
		sc_tree.Put(SC_Meta_Key(scid), meta.MarshalBinary())

		for i, transfer := range w_sc_data_tree.transfere { // give devs reward
			var balance_serialized []byte
			addr_bytes := []byte(transfer.Address)
			balance_serialized, err = balance_tree.Get(addr_bytes)
			if err != nil {
				fmt.Printf("%s %d  could not transfer %d  %+v\n", scid, i, transfer.Amount, addr_bytes)
				return
			}
			balance := new(crypto.ElGamal).Deserialize(balance_serialized)
			balance = balance.Plus(new(big.Int).SetUint64(transfer.Amount)) // add devs reward to devs balance homomorphically
			balance_tree.Put(addr_bytes, balance.Serialize())               // reserialize and store

			//fmt.Printf("%s paid back %d\n", scid, transfer.Amount)

		}

		/*
			c := data_tree.Cursor()
			for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
				fmt.Printf("key=%s (%x), value=%s\n", k, k, v)
			}
			fmt.Printf("cursor complete\n")
		*/

		//h, err := data_tree.Hash()
		//fmt.Printf("%s successfully executed sc_call data_tree hash %x %s\n", scid, h, err)

	}

	return tx.Fees(), nil
}
