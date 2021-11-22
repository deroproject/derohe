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
import "bufio"
import "strings"
import "strconv"
import "runtime/debug"
import "encoding/hex"
import "encoding/binary"
import "math/big"
import "golang.org/x/xerrors"

import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/cryptography/bn256"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/premine"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/dvm"
import "github.com/deroproject/graviton"

// convert bitcoin model to our, but skip initial 4 years of supply, so our total supply gets to 10.5 million
const RewardReductionInterval = 210000 * 600 / config.BLOCK_TIME // 210000 comes from bitcoin
const BaseReward = (50 * 100000 * config.BLOCK_TIME) / 600       // convert bitcoin reward system to our block

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
func (chain *Blockchain) process_miner_transaction(bl *block.Block, genesis bool, balance_tree *graviton.Tree, fees uint64, height uint64) {

	tx := bl.Miner_TX
	var acckey crypto.Point
	if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
		panic(err)
	}

	if genesis == true { // process premine ,register genesis block, dev key
		balance := crypto.ConstructElGamal(acckey.G1(), crypto.ElGamal_BASE_G) // init zero balance
		balance = balance.Plus(new(big.Int).SetUint64(tx.Value << 1))          // add premine to users balance homomorphically
		nb := crypto.NonceBalance{NonceHeight: 0, Balance: balance}
		balance_tree.Put(tx.MinerAddress[:], nb.Serialize()) // reserialize and store

		// we must process premine list and register and give them balance,
		premine_count := 0
		scanner := bufio.NewScanner(strings.NewReader(premine.List))
		for scanner.Scan() {
			data := strings.Split(scanner.Text(), ",")
			if len(data) < 2 {
				panic("invalid premine list")
			}

			var raw_tx [4096]byte
			var rtx transaction.Transaction
			if ramount, err := strconv.ParseUint(data[0], 10, 64); err != nil {
				panic(err)
			} else if n, err := hex.Decode(raw_tx[:], []byte(data[1])); err != nil {
				panic(err)
			} else if err := rtx.Deserialize(raw_tx[:n]); err != nil {
				panic(err)
			} else if !rtx.IsRegistration() {
				panic("tx is not registration")
			} else if !rtx.IsRegistrationValid() {
				panic("tx registration signature is invalid")
			} else {

				var racckey crypto.Point
				if err := racckey.DecodeCompressed(rtx.MinerAddress[:]); err != nil {
					panic(err)
				}

				balance := crypto.ConstructElGamal(racckey.G1(), crypto.ElGamal_BASE_G) // init zero balance
				balance = balance.Plus(new(big.Int).SetUint64(ramount))                 // add premine to users balance homomorphically
				nb := crypto.NonceBalance{NonceHeight: 0, Balance: balance}
				balance_tree.Put(rtx.MinerAddress[:], nb.Serialize()) // reserialize and store
				premine_count++
			}
		}

		logger.V(1).Info("successfully added premine accounts", "count", premine_count)

		return
	}

	// general coin base transaction
	base_reward := CalcBlockReward(uint64(height))
	full_reward := base_reward + fees

	//full_reward is divided into equal parts for all miner blocks + miner address
	// since perfect division is not possible, ( see money handling)
	// any left over change is delivered to main miner who integrated the full block

	share := full_reward / uint64(len(bl.MiniBlocks)+1)              //  one block integrator, this is integer division
	leftover := full_reward - (share * uint64(len(bl.MiniBlocks)+1)) // only integrator will get this

	{ // giver integrator his reward
		balance_serialized, err := balance_tree.Get(tx.MinerAddress[:])
		if err != nil {
			panic(err)
		}
		nb := new(crypto.NonceBalance).Deserialize(balance_serialized)
		nb.Balance = nb.Balance.Plus(new(big.Int).SetUint64(share + leftover)) // add miners reward to miners balance homomorphically
		balance_tree.Put(tx.MinerAddress[:], nb.Serialize())                   // reserialize and store
	}

	// all the other miniblocks will get their share
	for _, mbl := range bl.MiniBlocks {
		_, key_compressed, balance_serialized, err := balance_tree.GetKeyValueFromHash(mbl.KeyHash[:16])
		if err != nil {
			panic(err)
		}

		nb := new(crypto.NonceBalance).Deserialize(balance_serialized)
		nb.Balance = nb.Balance.Plus(new(big.Int).SetUint64(share)) // add miners reward to miners balance homomorphically
		balance_tree.Put(key_compressed[:], nb.Serialize())         // reserialize and store

	}

	return

}

// process the tx, giving fees, miner rewatd etc
// this should be atomic, either all should be done or none at all
func (chain *Blockchain) process_transaction(changed map[crypto.Hash]*graviton.Tree, tx transaction.Transaction, balance_tree *graviton.Tree, height uint64) uint64 {

	logger.V(2).Info("Processing/Executing transaction", "txid", tx.GetHash(), "type", tx.TransactionType.String())
	switch tx.TransactionType {

	case transaction.REGISTRATION: // miner address represents registration
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

		if !globals.IsMainnet() { // give testnet users a dummy amount to play
			zerobalance = zerobalance.Plus(new(big.Int).SetUint64(800000)) // add fix amount to every wallet to users balance for more testing
		}

		nb := crypto.NonceBalance{NonceHeight: 0, Balance: zerobalance}

		balance_tree.Put(tx.MinerAddress[:], nb.Serialize())

		return 0 // registration doesn't give any fees . why & how ?

	case transaction.BURN_TX, transaction.NORMAL, transaction.SC_TX: // burned amount is not added anywhere and thus lost forever
		for t := range tx.Payloads {
			var tree *graviton.Tree
			if tx.Payloads[t].SCID.IsZero() {
				tree = balance_tree
			} else {
				tree = changed[tx.Payloads[t].SCID]
			}

			parity := tx.Payloads[t].Proof.Parity()
			for i := 0; i < int(tx.Payloads[t].Statement.RingSize); i++ {
				key_pointer := tx.Payloads[t].Statement.Publickeylist_pointers[i*int(tx.Payloads[t].Statement.Bytes_per_publickey) : (i+1)*int(tx.Payloads[t].Statement.Bytes_per_publickey)]
				_, key_compressed, balance_serialized, err := tree.GetKeyValueFromHash(key_pointer)

				if err != nil && !tx.Payloads[t].SCID.IsZero() {
					if xerrors.Is(err, graviton.ErrNotFound) { // if the address is not found, lookup in main tree
						_, key_compressed, _, err = balance_tree.GetKeyValueFromHash(key_pointer)
						if err == nil {
							var p bn256.G1
							if err = p.DecodeCompressed(key_compressed[:]); err != nil {
								panic(fmt.Errorf("key %d could not be decompressed", i))
							}

							balance := crypto.ConstructElGamal(&p, crypto.ElGamal_BASE_G) // init zero balance
							nb := crypto.NonceBalance{NonceHeight: 0, Balance: balance}
							balance_serialized = nb.Serialize()
						}

					}
				}
				if err != nil {
					panic(fmt.Errorf("balance not obtained err %s\n", err))
				}

				nb := new(crypto.NonceBalance).Deserialize(balance_serialized)
				echanges := crypto.ConstructElGamal(tx.Payloads[t].Statement.C[i], tx.Payloads[t].Statement.D)

				nb.Balance = nb.Balance.Add(echanges) // homomorphic addition of changes

				if (i%2 == 0) == parity { // this condition is well thought out and works good enough
					nb.NonceHeight = height
				}
				tree.Put(key_compressed, nb.Serialize()) // reserialize and store

			}
		}

		return tx.Fees()

	default:
		panic("unknown transaction, do not know how to process it")
		return 0
	}
}

type Tree_Wrapper struct {
	tree      *graviton.Tree
	entries   map[string][]byte
	transfere []dvm.TransferExternal
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

// checks cache and returns a wrapped tree if possible
func wrapped_tree(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, id crypto.Hash) *Tree_Wrapper {
	if cached_tree, ok := cache[id]; ok { // tree is in cache return it
		return &Tree_Wrapper{tree: cached_tree, entries: map[string][]byte{}}
	}

	if tree, err := ss.GetTree(string(id[:])); err != nil {
		panic(err)
	} else {
		return &Tree_Wrapper{tree: tree, entries: map[string][]byte{}}
	}
}

// does additional processing for SC
// all processing occurs in wrapped trees, if any error occurs we dicard all trees
func (chain *Blockchain) process_transaction_sc(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, bl_height, bl_topoheight, bl_timestamp uint64, blid crypto.Hash, tx transaction.Transaction, balance_tree *graviton.Tree, sc_tree *graviton.Tree) (gas uint64, err error) {

	if len(tx.SCDATA) == 0 {
		return tx.Fees(), nil
	}

	gas = tx.Fees()

	w_balance_tree := &Tree_Wrapper{tree: balance_tree, entries: map[string][]byte{}}
	w_sc_tree := &Tree_Wrapper{tree: sc_tree, entries: map[string][]byte{}}

	_ = w_balance_tree
	var w_sc_data_tree *Tree_Wrapper

	txhash := tx.GetHash()
	scid := txhash

	defer func() {
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recover while executing SC ", "txid", txhash, "error", r, "stack", fmt.Sprintf("%s", string(debug.Stack())))

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

		if sc, pos, err = dvm.ParseSmartContract(sc_code); err != nil {
			logger.V(2).Error(err, "error Parsing sc", "txid", txhash, "pos", pos)
			break
		}

		meta := SC_META_DATA{}
		if _, ok := sc.Functions["InitializePrivate"]; ok {
			meta.Type = 1
		}

		w_sc_data_tree = wrapped_tree(cache, ss, scid)

		// install SC, should we check for sanity now, why or why not
		w_sc_data_tree.Put(SC_Code_Key(scid), dvm.Variable{Type: dvm.String, ValueString: sc_code}.MarshalBinaryPanic())
		w_sc_tree.Put(SC_Meta_Key(scid), meta.MarshalBinary())

		if meta.Type == 1 { // if its a a private SC
			gas, err = chain.execute_sc_function(w_sc_tree, w_sc_data_tree, scid, bl_height, bl_topoheight, bl_timestamp, blid, tx, "InitializePrivate", 1)
		} else {
			gas, err = chain.execute_sc_function(w_sc_tree, w_sc_data_tree, scid, bl_height, bl_topoheight, bl_timestamp, blid, tx, "Initialize", 1)
		}

		if err != nil {
			return
		}

		//fmt.Printf("Error status after initializing SC %s\n",err)

	case rpc.SC_CALL: // trigger a CALL
		if !tx.SCDATA.Has(rpc.SCID, rpc.DataHash) { // but only if it is present
			err = fmt.Errorf("no scid provided")
			break
		}
		if !tx.SCDATA.Has("entrypoint", rpc.DataString) { // but only if it is present
			err = fmt.Errorf("no entrypoint provided")
			break
		}

		scid = tx.SCDATA.Value(rpc.SCID, rpc.DataHash).(crypto.Hash)

		if _, err = w_sc_tree.Get(SC_Meta_Key(scid)); err != nil {
			err = fmt.Errorf("scid %s not installed", scid)
			return
		}

		w_sc_data_tree = wrapped_tree(cache, ss, scid)

		entrypoint := tx.SCDATA.Value("entrypoint", rpc.DataString).(string)
		//fmt.Printf("We must call the SC %s function\n", entrypoint)

		gas, err = chain.execute_sc_function(w_sc_tree, w_sc_data_tree, scid, bl_height, bl_topoheight, bl_timestamp, blid, tx, entrypoint, 1)

	default: // unknown  what to do
		err = fmt.Errorf("unknown action what to do scid %x", scid)
		return
	}

	// we must commit all the changes
	// check whether we are not overflowing/underflowing, means SC is not over sending
	if err == nil {
		total_per_asset := map[crypto.Hash]uint64{}
		for _, transfer := range w_sc_data_tree.transfere { // do external tranfer
			if transfer.Amount == 0 {
				continue
			}

			// an SCID can generate it's token infinitely
			if transfer.Asset != scid && total_per_asset[transfer.Asset]+transfer.Amount <= total_per_asset[transfer.Asset] {
				err = fmt.Errorf("Balance calculation overflow")
				break
			} else {
				total_per_asset[transfer.Asset] = total_per_asset[transfer.Asset] + transfer.Amount
			}
		}

		if err == nil {
			for asset, value := range total_per_asset {
				stored_value, _ := chain.LoadSCAssetValue(w_sc_data_tree, scid, asset)
				// an SCID can generate it's token infinitely
				if asset != scid && stored_value-value > stored_value {
					err = fmt.Errorf("Balance calculation underflow stored_value %d  transferring %d\n", stored_value, value)
					break
				}

				var new_value [8]byte
				binary.BigEndian.PutUint64(new_value[:], stored_value-value)
				chain.StoreSCValue(w_sc_data_tree, scid, asset[:], new_value[:])
			}
		}

		//also check whether all destinations are registered
		if err == nil {
			for _, transfer := range w_sc_data_tree.transfere {
				if _, err = balance_tree.Get([]byte(transfer.Address)); err == nil || xerrors.Is(err, graviton.ErrNotFound) {
					// everything is okay
				} else {
					err = fmt.Errorf("account is unregistered")
					logger.V(1).Error(err, "account is unregistered", "txhash", txhash, "scid", scid, "address", transfer.Address)
					break
				}
			}
		}
	}

	if err != nil { // error occured, give everything to SC, since we may not have information to send them back
		if chain.simulator {
			logger.Error(err, "error executing sc", "txid", txhash)
		}

		for _, payload := range tx.Payloads {
			var new_value [8]byte

			w_sc_data_tree = wrapped_tree(cache, ss, scid) // get a new tree, discarding everything

			stored_value, _ := chain.LoadSCAssetValue(w_sc_data_tree, scid, payload.SCID)
			binary.BigEndian.PutUint64(new_value[:], stored_value+payload.BurnValue)
			chain.StoreSCValue(w_sc_data_tree, scid, payload.SCID[:], new_value[:])

			for k, v := range w_sc_data_tree.entries { // commit incoming balances to tree
				if err = w_sc_data_tree.tree.Put([]byte(k), v); err != nil {
					return
				}
			}

			//for k, v := range w_sc_tree.entries {
			//	if err = w_sc_tree.tree.Put([]byte(k), v); err != nil {
			//		return
			//	}
			//}

		}

		return
	}

	// anything below should never give error
	if _, ok := cache[scid]; !ok {
		cache[scid] = w_sc_data_tree.tree
	}

	for k, v := range w_sc_data_tree.entries { // commit entire data to tree
		if _, ok := globals.Arguments["--debug"]; ok && globals.Arguments["--debug"] != nil && chain.simulator {
			logger.V(1).Info("Writing", "txid", txhash, "scid", scid, "key", fmt.Sprintf("%x", k), "value", fmt.Sprintf("%x", v))
		}
		if err = w_sc_data_tree.tree.Put([]byte(k), v); err != nil {
			return
		}
	}

	for k, v := range w_sc_tree.entries {
		if err = w_sc_tree.tree.Put([]byte(k), v); err != nil {
			return
		}
	}

	for i, transfer := range w_sc_data_tree.transfere { // do external tranfer
		if transfer.Amount == 0 {
			continue
		}
		//fmt.Printf("%d sending to external %s %x\n", i,transfer.Asset,transfer.Address)
		var zeroscid crypto.Hash

		var curbtree *graviton.Tree
		switch transfer.Asset {
		case zeroscid: // main dero balance, handle it
			curbtree = balance_tree
		case scid: // this scid balance, handle it
			curbtree = cache[scid]
		default: // any other asset scid
			var ok bool
			if curbtree, ok = cache[transfer.Asset]; !ok {
				if curbtree, err = ss.GetTree(string(transfer.Asset[:])); err != nil {
					panic(err)
				}
				cache[transfer.Asset] = curbtree
			}
		}

		if curbtree == nil {
			panic("tree cannot be nil at this point in time")
		}

		addr_bytes := []byte(transfer.Address)
		if _, err = balance_tree.Get(addr_bytes); err != nil { // first check whether address is registered
			err = fmt.Errorf("sending to non registered account acc %x err %s", addr_bytes, err) // this can only occur, if account no registered or dis corruption
			panic(err)
		}

		var balance_serialized []byte
		balance_serialized, err = curbtree.Get(addr_bytes)
		if err != nil && xerrors.Is(err, graviton.ErrNotFound) { // if the address is not found, lookup in main tree
			var p bn256.G1
			if err = p.DecodeCompressed(addr_bytes[:]); err != nil {
				panic(fmt.Errorf("key %x could not be decompressed", addr_bytes))
			}

			balance := crypto.ConstructElGamal(&p, crypto.ElGamal_BASE_G) // init zero balance
			nb := crypto.NonceBalance{NonceHeight: 0, Balance: balance}
			balance_serialized = nb.Serialize()
		} else if err != nil {
			fmt.Printf("%s %d  could not transfer %d  %+v\n", scid, i, transfer.Amount, addr_bytes)
			panic(err) // only disk corruption can reach here
		}

		nb := new(crypto.NonceBalance).Deserialize(balance_serialized)
		nb.Balance = nb.Balance.Plus(new(big.Int).SetUint64(transfer.Amount)) // add transfer to users balance homomorphically
		curbtree.Put(addr_bytes, nb.Serialize())                              // reserialize and store
	}

	//c := w_sc_data_tree.tree.Cursor()
	//for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
	//	fmt.Printf("key=%s (%x), value=%s\n", k, k, v)
	//}
	//fmt.Printf("cursor complete\n")

	//h, err := data_tree.Hash()
	//fmt.Printf("%s successfully executed sc_call data_tree hash %x %s\n", scid, h, err)

	return tx.Fees(), nil
}

// func extract signer from a tx, if possible
// extract signer is only possible if ring size is 2
func extract_signer(tx *transaction.Transaction) (signer [33]byte, err error) {
	for t := range tx.Payloads {
		if uint64(len(tx.Payloads[t].Statement.Publickeylist_compressed)) != tx.Payloads[t].Statement.RingSize {
			return signer, fmt.Errorf("tx is not expanded")
		}
		if tx.Payloads[t].SCID.IsZero() && tx.Payloads[t].Statement.RingSize == 2 {
			parity := tx.Payloads[t].Proof.Parity()
			for i := 0; i < int(tx.Payloads[t].Statement.RingSize); i++ {
				if (i%2 == 0) == parity { // this condition is well thought out and works good enough
					copy(signer[:], tx.Payloads[t].Statement.Publickeylist_compressed[i][:])
					return
				}
			}

		}
	}

	return signer, fmt.Errorf("unknown signer")
}
