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

package dvm

// this file implements necessary structure to  SC handling

import "fmt"

//import "bytes"
//import "runtime/debug"
import "encoding/binary"
import "time"
import "math/big"
import "math/rand"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/cryptography/bn256"

import "golang.org/x/xerrors"
import "github.com/deroproject/graviton"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/config"

//import "github.com/deroproject/derohe/transaction"

type Simulator struct {
	ss           *graviton.Snapshot
	balance_tree *graviton.Tree
	sc_tree      *graviton.Tree
	cache        map[crypto.Hash]*graviton.Tree
	height       uint64
	Balances     map[string]map[string]uint64
}

func SimulatorInitialize(ss *graviton.Snapshot) *Simulator {

	var s Simulator
	var err error

	if ss == nil {
		store, err := graviton.NewMemStore()
		if err != nil {
			panic(err)
		}
		ss, err = store.LoadSnapshot(0)
		if err != nil {
			panic(err)
		}
		s.ss = ss
	}
	s.ss = ss

	s.balance_tree, err = ss.GetTree(config.BALANCE_TREE)
	if err != nil {
		panic(err)
	}
	s.sc_tree, err = ss.GetTree(config.SC_META)
	if err != nil {
		panic(err)
	}
	s.cache = map[crypto.Hash]*graviton.Tree{}
	s.Balances = map[string]map[string]uint64{}

	//w_balance_tree := &dvm.Tree_Wrapper{Tree: balance_tree, Entries: map[string][]byte{}}
	//w_sc_tree := &dvm.Tree_Wrapper{Tree: sc_tree, Entries: map[string][]byte{}}

	return &s
}

// this is for testing some edge case in simulator
func (s *Simulator) AccountAddBalance(addr rpc.Address, scid crypto.Hash, balance_value uint64) {
	balance := crypto.ConstructElGamal((*bn256.G1)(addr.PublicKey), crypto.ElGamal_BASE_G) // init zero balance
	nb := crypto.NonceBalance{NonceHeight: 0, Balance: balance}
	s.balance_tree.Put(addr.Compressed(), nb.Serialize())
}

func (s *Simulator) SCInstall(sc_code string, incoming_values map[crypto.Hash]uint64, SCDATA rpc.Arguments, signer_addr *rpc.Address, fees uint64) (scid crypto.Hash, gascompute, gasstorage uint64, err error) {
	var blid crypto.Hash
	rand.Seed(time.Now().Unix())
	rand.Read(scid[:])
	rand.Read(blid[:])

	var sc SmartContract
	if sc, _, err = ParseSmartContract(sc_code); err != nil {
		//logger.V(2).Error(err, "error Parsing sc", "txid", txhash, "pos", pos)
		return
	}

	var meta SC_META_DATA
	if _, ok := sc.Functions["InitializePrivate"]; ok {
		meta.Type = 1
	}

	w_sc_data_tree := Wrapped_tree(s.cache, s.ss, scid)
	w_sc_data_tree.Put(SC_Code_Key(scid), Variable{Type: String, ValueString: sc_code}.MarshalBinaryPanic())
	w_sc_tree := &Tree_Wrapper{Tree: s.sc_tree, Entries: map[string][]byte{}}
	w_sc_tree.Put(SC_Meta_Key(scid), meta.MarshalBinary())

	entrypoint := "Initialize"
	if meta.Type == 1 { // if its a a private SC
		entrypoint = "InitializePrivate"
	}

	gascompute, gasstorage, err = s.common(w_sc_tree, w_sc_data_tree, scid, s.height, s.height, uint64(time.Now().Unix()), blid, scid, sc, entrypoint, 1, 0, signer_addr, incoming_values, SCDATA, fees, true)
	return
}

func (s *Simulator) RunSC(incoming_values map[crypto.Hash]uint64, SCDATA rpc.Arguments, signer_addr *rpc.Address, fees uint64) (gascompute, gasstorage uint64, err error) {
	var blid, txid crypto.Hash
	rand.Read(blid[:])
	rand.Read(txid[:])

	action_code := rpc.SC_ACTION(SCDATA.Value(rpc.SCACTION, rpc.DataUint64).(uint64))

	switch action_code {
	case rpc.SC_INSTALL: // request to install an SC
		err = fmt.Errorf("cannot install code using this api")
		return

	case rpc.SC_CALL: // trigger a CALL
		if !SCDATA.Has(rpc.SCID, rpc.DataHash) { // but only if it is present
			err = fmt.Errorf("no scid provided")
			return
		}
		if !SCDATA.Has("entrypoint", rpc.DataString) { // but only if it is present
			err = fmt.Errorf("no entrypoint provided")
			return
		}

		scid := SCDATA.Value(rpc.SCID, rpc.DataHash).(crypto.Hash)

		w_sc_tree := &Tree_Wrapper{Tree: s.sc_tree, Entries: map[string][]byte{}}
		if _, err = w_sc_tree.Get(SC_Meta_Key(scid)); err != nil {
			err = fmt.Errorf("scid %s not installed", scid)
			return
		}

		w_sc_data_tree := Wrapped_tree(s.cache, s.ss, scid)
		entrypoint := SCDATA.Value("entrypoint", rpc.DataString).(string)
		balance, sc, _ := ReadSC(w_sc_tree, w_sc_data_tree, scid)

		gascompute, gasstorage, err = s.common(w_sc_tree, w_sc_data_tree, scid, s.height, s.height, uint64(time.Now().Unix()), blid, scid, sc, entrypoint, 1, balance, signer_addr, incoming_values, SCDATA, fees, true)
		return
	default:
		err = fmt.Errorf("unknown action_code code %d", action_code)
	}

	return
}

func (s *Simulator) common(w_sc_tree, w_sc_data_tree *Tree_Wrapper, scid crypto.Hash, bl_height, bl_topoheight, bl_timestamp uint64, blid crypto.Hash, txid crypto.Hash, sc SmartContract, entrypoint string, hard_fork_version_current int64, balance_at_start uint64, signer_addr *rpc.Address, incoming_values map[crypto.Hash]uint64, SCDATA rpc.Arguments, fees uint64, simulator bool) (gascompute, gasstorage uint64, err error) {

	var signer [33]byte
	if signer_addr != nil {
		copy(signer[:], signer_addr.Compressed())
	}

	gascompute, gasstorage, err = Execute_sc_function(w_sc_tree, w_sc_data_tree, scid, bl_height, bl_topoheight, uint64(time.Now().Unix()), blid, scid, sc, entrypoint, 1, 0, signer, incoming_values, SCDATA, fees, simulator)
	fmt.Printf("sc execution error %s\n", err)

	// we must commit all the changes
	// check whether we are not overflowing/underflowing, means SC is not over sending
	if err == nil {
		err = SanityCheckExternalTransfers(w_sc_data_tree, s.balance_tree, scid)
	}

	if err != nil { // error occured, give everything to SC, since we may not have information to send them back
		var zeroaddress [33]byte
		if signer != zeroaddress { // if we can identify sender, return funds to him
			ErrorRevert(s.ss, s.cache, s.balance_tree, signer, scid, incoming_values)
		} else { //  we could not extract signer, give burned funds to SC
			ErrorRevert(s.ss, s.cache, s.balance_tree, signer, scid, incoming_values)
		}

		return
	}
	ProcessExternal(s.ss, s.cache, s.balance_tree, signer, scid, w_sc_data_tree, w_sc_tree)
	return

}

// this is core function used to evaluate when we are overflowing/underflowing
func SanityCheckExternalTransfers(w_sc_data_tree *Tree_Wrapper, balance_tree *graviton.Tree, scid crypto.Hash) (err error) {
	total_per_asset := map[crypto.Hash]uint64{}
	for _, transfer := range w_sc_data_tree.Transfere { // do external tranfer
		if transfer.Amount == 0 {
			continue
		}

		// an SCID can generate it's token infinitely
		if transfer.Asset != scid && total_per_asset[transfer.Asset]+transfer.Amount <= total_per_asset[transfer.Asset] {
			err = fmt.Errorf("Balance calculation overflow")
			return
		} else {
			total_per_asset[transfer.Asset] = total_per_asset[transfer.Asset] + transfer.Amount
		}
	}

	if err == nil {
		for asset, value := range total_per_asset {
			stored_value, _ := LoadSCAssetValue(w_sc_data_tree, scid, asset)
			// an SCID can generate it's token infinitely
			if asset != scid && stored_value-value > stored_value {
				err = fmt.Errorf("Balance calculation underflow stored_value %d  transferring %d\n", stored_value, value)
				return
			}

			var new_value [8]byte
			binary.BigEndian.PutUint64(new_value[:], stored_value-value)
			StoreSCValue(w_sc_data_tree, scid, asset[:], new_value[:])
		}
	}

	//also check whether all destinations are registered
	if err == nil {
		for _, transfer := range w_sc_data_tree.Transfere {
			if _, err = balance_tree.Get([]byte(transfer.Address)); err == nil || xerrors.Is(err, graviton.ErrNotFound) {
				// everything is okay
			} else {
				err = fmt.Errorf("account is unregistered")
				//logger.V(1).Error(err, "account is unregistered", "txhash", txhash, "scid", scid, "address", transfer.Address)
				return
			}
		}
	}

	return
}

// any error during this will panic
func ErrorRevert(ss *graviton.Snapshot, cache map[crypto.Hash]*graviton.Tree, balance_tree *graviton.Tree, signer [33]byte, scid crypto.Hash, incoming_values map[crypto.Hash]uint64) {
	var err error

	for scid_asset, burnvalue := range incoming_values {
		var zeroscid crypto.Hash

		var curbtree *graviton.Tree
		switch scid_asset {
		case zeroscid: // main dero balance, handle it
			curbtree = balance_tree
		case scid: // this scid balance, handle it
			curbtree = cache[scid]
		default: // any other asset scid
			var ok bool
			if curbtree, ok = cache[scid_asset]; !ok {
				if curbtree, err = ss.GetTree(string(scid_asset[:])); err != nil {
					panic(err)
				}
				cache[scid_asset] = curbtree
			}
		}

		if curbtree == nil {
			panic("tree cannot be nil at this point in time")
		}

		if balance_serialized, err1 := curbtree.Get(signer[:]); err1 != nil { // no error can occur
			panic(err1) // only disk corruption can reach here
		} else {
			nb := new(crypto.NonceBalance).Deserialize(balance_serialized)
			nb.Balance = nb.Balance.Plus(new(big.Int).SetUint64(burnvalue)) // add back burn value to users balance homomorphically
			curbtree.Put(signer[:], nb.Serialize())                         // reserialize and store
		}
	}
}

func ErrorDeposit(ss *graviton.Snapshot, cache map[crypto.Hash]*graviton.Tree, balance_tree *graviton.Tree, signer [33]byte, scid crypto.Hash, incoming_values map[crypto.Hash]uint64) {

	for scid_asset, burnvalue := range incoming_values {
		var new_value [8]byte

		w_sc_data_tree := Wrapped_tree(cache, ss, scid) // get a new tree, discarding everything

		stored_value, _ := LoadSCAssetValue(w_sc_data_tree, scid, scid_asset)
		binary.BigEndian.PutUint64(new_value[:], stored_value+burnvalue)
		StoreSCValue(w_sc_data_tree, scid, scid_asset[:], new_value[:])

		for k, v := range w_sc_data_tree.Entries { // commit incoming balances to tree
			if err := w_sc_data_tree.Tree.Put([]byte(k), v); err != nil {
				panic(err)
			}
		}

		cache[scid] = w_sc_data_tree.Tree
	}
}

func ProcessExternal(ss *graviton.Snapshot, cache map[crypto.Hash]*graviton.Tree, balance_tree *graviton.Tree, signer [33]byte, scid crypto.Hash, w_sc_data_tree, w_sc_tree *Tree_Wrapper) {
	var err error

	// anything below should never give error
	cache[scid] = w_sc_data_tree.Tree

	for k, v := range w_sc_data_tree.Entries { // commit entire data to tree
		//if _, ok := globals.Arguments["--debug"]; ok && globals.Arguments["--debug"] != nil && chain.simulator {
		//		logger.V(1).Info("Writing", "txid", txhash, "scid", scid, "key", fmt.Sprintf("%x", k), "value", fmt.Sprintf("%x", v))
		//	}
		if len(v) == 0 {
			if err = w_sc_data_tree.Tree.Delete([]byte(k)); err != nil {
				panic(err)
			}
		} else {
			if err = w_sc_data_tree.Tree.Put([]byte(k), v); err != nil {
				panic(err)
			}
		}
	}

	for k, v := range w_sc_tree.Entries {
		if err = w_sc_tree.Tree.Put([]byte(k), v); err != nil {
			panic(err)
		}
	}

	for i, transfer := range w_sc_data_tree.Transfere { // do external tranfer
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

}
