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
import "bytes"
import "runtime/debug"
import "encoding/binary"
import "github.com/deroproject/derohe/cryptography/crypto"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/graviton"

//import "github.com/deroproject/derohe/transaction"

// currently DERO hash 2 contract types
// 1 OPEN
// 2 PRIVATE
type SC_META_DATA struct {
	Type     byte        // 0  Open, 1 Private
	DataHash crypto.Hash // hash of SC data tree is here, so as the meta tree verifies all  SC DATA
}

// serialize the structure
func (meta SC_META_DATA) MarshalBinary() (buf []byte) {
	buf = make([]byte, 33, 33)
	buf[0] = meta.Type
	copy(buf[1+len(meta.DataHash):], meta.DataHash[:])
	return
}

func (meta *SC_META_DATA) UnmarshalBinary(buf []byte) (err error) {
	if len(buf) != 1+32 {
		return fmt.Errorf("input buffer should be of 33 bytes in length")
	}
	meta.Type = buf[0]
	copy(meta.DataHash[:], buf[1+len(meta.DataHash):])
	return nil
}

// serialize the structure
func (meta SC_META_DATA) MarshalBinaryGood() (buf []byte) {
	buf = make([]byte, 0, 33)
	buf = append(buf, meta.Type)
	buf = append(buf, meta.DataHash[:]...)
	return
}

func (meta *SC_META_DATA) UnmarshalBinaryGood(buf []byte) (err error) {
	if len(buf) != 1+32 {
		return fmt.Errorf("input buffer should be of 33 bytes in length")
	}
	meta.Type = buf[0]
	copy(meta.DataHash[:], buf[1:])
	return nil
}

func SC_Meta_Key(scid crypto.Hash) []byte {
	return scid[:]
}
func SC_Code_Key(scid crypto.Hash) []byte {
	return Variable{Type: String, ValueString: "C"}.MarshalBinaryPanic()
}
func SC_Asset_Key(asset crypto.Hash) []byte {
	return asset[:]
}

// used to wrap a graviton tree, so it could be discarded at any time
type Tree_Wrapper struct {
	Tree      *graviton.Tree
	Entries   map[string][]byte
	Transfere []TransferExternal
}

func (t *Tree_Wrapper) Get(key []byte) ([]byte, error) {
	if value, ok := t.Entries[string(key)]; ok {
		return value, nil
	} else {
		return t.Tree.Get(key)
	}
}

func (t *Tree_Wrapper) Put(key []byte, value []byte) error {
	t.Entries[string(key)] = append([]byte{}, value...)
	return nil
}

// checks cache and returns a wrapped tree if possible
func Wrapped_tree(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, id crypto.Hash) *Tree_Wrapper {
	if cached_tree, ok := cache[id]; ok { // tree is in cache return it
		return &Tree_Wrapper{Tree: cached_tree, Entries: map[string][]byte{}}
	}

	if tree, err := ss.GetTree(string(id[:])); err != nil {
		panic(err)
	} else {
		return &Tree_Wrapper{Tree: tree, Entries: map[string][]byte{}}
	}
}

// this will process the SC transaction
// the tx should only be processed , if it has been processed

func Execute_sc_function(w_sc_tree *Tree_Wrapper, data_tree *Tree_Wrapper, scid crypto.Hash, bl_height, bl_topoheight, bl_timestamp uint64, blid crypto.Hash, txid crypto.Hash, sc_parsed SmartContract, entrypoint string, hard_fork_version_current int64, balance_at_start uint64, signer [33]byte, incoming_value map[crypto.Hash]uint64, SCDATA rpc.Arguments, gasstorage_incoming uint64, simulator bool) (gascompute, gasstorage uint64, err error) {
	defer func() {
		if r := recover(); r != nil { // safety so if anything wrong happens, verification fails
			if err == nil {
				err = fmt.Errorf("Stack trace  \n%s", debug.Stack())
			}
			//logger.V(1).Error(err, "Recovered while rewinding chain,", "r", r, "stack trace", string(debug.Stack()))
		}
	}()

	//fmt.Printf("executing entrypoint %s  values %+v feees %d\n", entrypoint, incoming_value, fees)

	tx_store := Initialize_TX_store()

	// used as value loader from disk
	// this function is used to load any data required by the SC
	balance_loader := func(key DataKey) (result uint64) {
		var found bool
		_ = found
		result, found = LoadSCAssetValue(data_tree, key.SCID, key.Asset)
		return result
	}

	diskloader := func(key DataKey, found *uint64) (result Variable) {
		var exists bool
		if result, exists = LoadSCValue(data_tree, key.SCID, key.MarshalBinaryPanic()); exists {
			*found = uint64(1)
		}
		//fmt.Printf("Loading from disk %+v  result %+v found status %+v \n", key, result, exists)

		return
	}

	diskloader_raw := func(key []byte) (value []byte, found bool) {
		var err error
		value, err = data_tree.Get(key[:])
		if err != nil {
			return value, false
		}

		if len(value) == 0 {
			return value, false
		}
		//fmt.Printf("Loading from disk %+v  result %+v found status %+v \n", key, result, exists)

		return value, true
	}

	//fmt.Printf("sc_parsed %+v\n", sc_parsed)
	// if we found the SC in parsed form, check whether entrypoint is found
	function, ok := sc_parsed.Functions[entrypoint]
	if !ok {
		err = fmt.Errorf("stored SC  does not contain entrypoint '%s' scid %s \n", entrypoint, scid)
		return
	}

	// setup block hash, height, topoheight correctly
	state := &Shared_State{
		Store:    tx_store,
		Assets:   map[crypto.Hash]uint64{},
		RamStore: map[Variable]Variable{},
		SCIDSELF: scid,
		Chain_inputs: &Blockchain_Input{
			BL_HEIGHT:     bl_height,
			BL_TOPOHEIGHT: uint64(bl_topoheight),
			BL_TIMESTAMP:  bl_timestamp,
			SCID:          scid,
			BLID:          blid,
			TXID:          txid,
			Signer:        string(signer[:]),
		},
	}

	tx_store.DiskLoader = diskloader // hook up loading from chain
	tx_store.DiskLoaderRaw = diskloader_raw
	tx_store.BalanceLoader = balance_loader
	tx_store.BalanceAtStart = balance_at_start
	tx_store.SCID = scid
	tx_store.State = state

	if _, ok = globals.Arguments["--debug"]; ok && globals.Arguments["--debug"] != nil && simulator {
		state.Trace = true // enable tracing for dvm simulator
	}

	for asset, value := range incoming_value {
		var new_value [8]byte
		stored_value, _ := LoadSCAssetValue(data_tree, scid, asset)
		binary.BigEndian.PutUint64(new_value[:], stored_value+value)
		StoreSCValue(data_tree, scid, asset[:], new_value[:])
		state.Assets[asset] += value
	}

	// we have an entrypoint, now we must setup parameters and dvm
	// all parameters are in string form to bypass translation issues in middle layers
	params := map[string]interface{}{}

	for _, p := range function.Params {
		var zerohash crypto.Hash
		switch {
		case p.Type == Uint64 && p.Name == "value":
			params[p.Name] = fmt.Sprintf("%d", state.Assets[zerohash]) // overide value
		case p.Type == Uint64 && SCDATA.Has(p.Name, rpc.DataUint64):
			params[p.Name] = fmt.Sprintf("%d", SCDATA.Value(p.Name, rpc.DataUint64).(uint64))
		case p.Type == String && SCDATA.Has(p.Name, rpc.DataString):
			params[p.Name] = SCDATA.Value(p.Name, rpc.DataString).(string)
		case p.Type == String && SCDATA.Has(p.Name, rpc.DataHash):
			h := SCDATA.Value(p.Name, rpc.DataHash).(crypto.Hash)
			params[p.Name] = string(h[:])
			//fmt.Printf("%s:%x\n", p.Name, string(h[:]))

		default:
			err = fmt.Errorf("entrypoint '%s' parameter type missing or not yet supported (%+v)", entrypoint, p)
			return
		}
	}

	state.GasComputeLimit = int64(10000000) // everyone has fixed amount of compute gas
	if state.GasComputeLimit > 0 {
		state.GasComputeCheck = true
	}

	// gas consumed in parameters to avoid tx bloats
	if gasstorage_incoming > 0 {
		if gasstorage_incoming > config.MAX_STORAGE_GAS_ATOMIC_UNITS {
			gasstorage_incoming = config.MAX_STORAGE_GAS_ATOMIC_UNITS // whatever gas may be provided, upper limit of gas is this
		}

		state.GasStoreLimit = int64(gasstorage_incoming)
		state.GasStoreCheck = true
	}

	// deduct gas from whatever has been included in TX
	var scdata_bytes []byte
	if scdata_bytes, err = SCDATA.MarshalBinary(); err != nil {
		return
	}

	scdata_length := len(scdata_bytes)
	state.ConsumeStorageGas(int64(scdata_length))

	result, err := RunSmartContract(&sc_parsed, entrypoint, state, params)

	if state.GasComputeUsed > 0 {
		gascompute = uint64(state.GasComputeUsed)
	}
	if state.GasStoreUsed > 0 {
		gasstorage = uint64(state.GasStoreUsed)
	}

	//fmt.Printf("result value %+v\n", result)

	if err != nil {
		//logger.V(2).Error(err, "error execcuting SC", "entrypoint", entrypoint, "scid", scid)
		return
	}

	if err == nil && result.Type == Uint64 && result.ValueUint64 == 0 { // confirm the changes
		for k, v := range tx_store.RawKeys {
			StoreSCValue(data_tree, scid, []byte(k), v)

			//			fmt.Printf("storing %x %x\n", k,v)
		}
		data_tree.Transfere = append(data_tree.Transfere, tx_store.Transfers[scid].TransferE...)
	} else { // discard all changes, since we never write to store immediately, they are purged, however we need to  return any value associated
		err = fmt.Errorf("Discarded knowingly")
		return
	}

	//fmt.Printf("SC execution finished amount value %d\n", tx.Value)
	return

}

// reads SC, balance
func ReadSC(w_sc_tree *Tree_Wrapper, data_tree *Tree_Wrapper, scid crypto.Hash) (balance uint64, sc SmartContract, found bool) {
	var zerohash crypto.Hash
	balance, _ = LoadSCAssetValue(data_tree, scid, zerohash)

	sc_bytes, err := data_tree.Get(SC_Code_Key(scid))
	if err != nil {
		return
	}

	var v Variable
	if err = v.UnmarshalBinary(sc_bytes); err != nil {
		return
	}

	sc, pos, err := ParseSmartContract(v.ValueString)
	if err != nil {
		return
	}

	_ = pos

	found = true
	return
}

func LoadSCValue(data_tree *Tree_Wrapper, scid crypto.Hash, key []byte) (v Variable, found bool) {
	//fmt.Printf("loading fromdb %s %s \n", scid, key)

	object_data, err := data_tree.Get(key[:])
	if err != nil {
		return v, false
	}

	if len(object_data) == 0 {
		return v, false
	}

	if err = v.UnmarshalBinary(object_data); err != nil {
		return v, false
	}

	return v, true
}

func LoadSCAssetValue(data_tree *Tree_Wrapper, scid crypto.Hash, asset crypto.Hash) (v uint64, found bool) {
	//fmt.Printf("loading fromdb %s %s \n", scid, key)

	object_data, err := data_tree.Get(asset[:])
	if err != nil {
		return v, false
	}

	if len(object_data) == 0 { // all assets are by default 0
		return v, true
	}
	if len(object_data) != 8 {
		return v, false
	}

	return binary.BigEndian.Uint64(object_data[:]), true
}

// reads a value from SC, always read balance
func ReadSCValue(data_tree *Tree_Wrapper, scid crypto.Hash, key interface{}) (value interface{}) {
	var keybytes []byte

	if key == nil {
		return
	}
	switch k := key.(type) {
	case uint64:
		keybytes = DataKey{Key: Variable{Type: Uint64, ValueUint64: k}}.MarshalBinaryPanic()
	case string:
		keybytes = DataKey{Key: Variable{Type: String, ValueString: k}}.MarshalBinaryPanic()
	//case int64:
	//	keybytes = dvm.DataKey{Key: dvm.Variable{Type: dvm.String, Value: k}}.MarshalBinaryPanic()
	default:
		return
	}

	value_var, found := LoadSCValue(data_tree, scid, keybytes)
	//fmt.Printf("read value %+v", value_var)
	if found && value_var.Type != Invalid {
		switch value_var.Type {
		case Uint64:
			value = value_var.ValueUint64
		case String:
			value = value_var.ValueString
		default:
			panic("This variable cannot be loaded")
		}
	}
	return
}

// store the value in the chain
func StoreSCValue(data_tree *Tree_Wrapper, scid crypto.Hash, key, value []byte) {
	if bytes.Compare(scid[:], key) == 0 { // an scid can mint its assets infinitely
		return
	}
	data_tree.Put(key, value)
	return
}
