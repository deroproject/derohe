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

// this file implements necessary structure to  SC handling

import "fmt"
import "runtime/debug"
import "encoding/binary"
import "github.com/deroproject/derohe/cryptography/crypto"

import "github.com/deroproject/derohe/dvm"

//import "github.com/deroproject/graviton"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/transaction"

import "github.com/romana/rlog"

// currently we 2 contract types
// 1 OPEN
// 2 PRIVATE
type SC_META_DATA struct {
	Type     byte // 0  Open, 1 Private
	Balance  uint64
	DataHash crypto.Hash // hash of SC data tree is here, so as the meta tree verifies all  SC DATA
}

// serialize the structure
func (meta SC_META_DATA) MarshalBinary() (buf []byte) {
	buf = make([]byte, 41, 41)
	buf[0] = meta.Type
	binary.LittleEndian.PutUint64(buf[1:], meta.Balance)
	copy(buf[1+8+len(meta.DataHash):], meta.DataHash[:])
	return
}

func (meta *SC_META_DATA) UnmarshalBinary(buf []byte) (err error) {
	if len(buf) != 1+8+32 {
		return fmt.Errorf("input buffer should be of 41 bytes in length")
	}
	meta.Type = buf[0]
	meta.Balance = binary.LittleEndian.Uint64(buf[1:])
	copy(meta.DataHash[:], buf[1+8+len(meta.DataHash):])
	return nil
}

func SC_Meta_Key(scid crypto.Hash) []byte {
	return scid[:]
}
func SC_Code_Key(scid crypto.Hash) []byte {
	return dvm.Variable{Type: dvm.String, Value: "C"}.MarshalBinaryPanic()
}

// this will process the SC transaction
// the tx should only be processed , if it has been processed

func (chain *Blockchain) execute_sc_function(w_sc_tree *Tree_Wrapper, data_tree *Tree_Wrapper, scid crypto.Hash, bl_height, bl_topoheight uint64, bl_hash crypto.Hash, tx transaction.Transaction, entrypoint string, hard_fork_version_current int64) (gas uint64, err error) {
	defer func() {
		// safety so if anything wrong happens, verification fails
		if r := recover(); r != nil {
			logger.Warnf("Recovered while rewinding chain, Stack trace below block_hash ")
			logger.Warnf("Stack trace  \n%s", debug.Stack())
		}
	}()

	//if !tx.Verify_SC_Signature() { // if tx is not SC TX, or Signature could not be verified skip it
	//	return
	//}

	tx_hash := tx.GetHash()
	tx_store := dvm.Initialize_TX_store()

	// used as value loader from disk
	// this function is used to load any data required by the SC

	diskloader := func(key dvm.DataKey, found *uint64) (result dvm.Variable) {
		var exists bool
		if result, exists = chain.LoadSCValue(data_tree, key.SCID, key.MarshalBinaryPanic()); exists {
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

	balance, sc_parsed, found := chain.ReadSC(w_sc_tree, data_tree, scid)
	if !found {
		fmt.Printf("SC not found\n")
		return
	}
	// if we found the SC in parsed form, check whether entrypoint is found
	function, ok := sc_parsed.Functions[entrypoint]
	if !ok {
		rlog.Warnf("stored SC  does not contain entrypoint '%s' scid %s \n", entrypoint, scid)
		return
	}
	_ = function

	//fmt.Printf("entrypoint found '%s' scid %s\n", entrypoint, scid)
	//if len(sc_tx.Params) == 0 { // initialize params if not initialized earlier
	//	sc_tx.Params = map[string]string{}
	//}
	//sc_tx.Params["value"] = fmt.Sprintf("%d", sc_tx.Value) // overide value

	tx_store.DiskLoader = diskloader // hook up loading from chain
	tx_store.DiskLoaderRaw = diskloader_raw
	tx_store.BalanceAtStart = balance
	tx_store.SCID = scid

	//fmt.Printf("tx store %v\n", tx_store)

	// setup block hash, height, topoheight correctly
	state := &dvm.Shared_State{
		Store: tx_store,
		Chain_inputs: &dvm.Blockchain_Input{
			BL_HEIGHT:     bl_height,
			BL_TOPOHEIGHT: uint64(bl_topoheight),
			SCID:          scid,
			BLID:          bl_hash,
			TXID:          tx_hash,
			Signer:        string(tx.MinerAddress[:]),
		},
	}

	for p := range tx.Payloads {
		if tx.Payloads[p].SCID.IsZero() {
			state.DERO_Received += tx.Payloads[p].BurnValue
		}
		if tx.Payloads[p].SCID == scid {
			state.Token_Received += tx.Payloads[p].BurnValue
		}
	}

	// setup balance correctly
	tx_store.ReceiveInternal(scid, state.DERO_Received)

	// we have an entrypoint, now we must setup parameters and dvm
	// all parameters are in string form to bypass translation issues in middle layers
	params := map[string]interface{}{}

	for _, p := range function.Params {
		switch {
		case p.Type == dvm.Uint64 && p.Name == "value":
			params[p.Name] = fmt.Sprintf("%d", state.DERO_Received) // overide value
		case p.Type == dvm.Uint64 && tx.SCDATA.Has(p.Name, rpc.DataUint64):
			params[p.Name] = fmt.Sprintf("%d", tx.SCDATA.Value(p.Name, rpc.DataUint64).(uint64))
		case p.Type == dvm.String && tx.SCDATA.Has(p.Name, rpc.DataString):
			params[p.Name] = tx.SCDATA.Value(p.Name, rpc.DataString).(string)

		default:
			err = fmt.Errorf("entrypoint '%s' parameter type missing or not yet supported (%+v)", entrypoint, p)
			return
		}
	}

	result, err := dvm.RunSmartContract(&sc_parsed, entrypoint, state, params)

	//fmt.Printf("result value %+v\n", result)

	if err != nil {
		rlog.Warnf("entrypoint '%s' scid %s  err execution '%s' \n", entrypoint, scid, err)
		return
	}

	if err == nil && result.Type == dvm.Uint64 && result.Value.(uint64) == 0 { // confirm the changes
		for k, v := range tx_store.Keys {
			chain.StoreSCValue(data_tree, scid, k.MarshalBinaryPanic(), v.MarshalBinaryPanic())
		}
		for k, v := range tx_store.RawKeys {
			chain.StoreSCValue(data_tree, scid, []byte(k), v)
		}

		data_tree.leftover_balance = tx_store.Balance(scid)
		data_tree.transfere = append(data_tree.transfere, tx_store.Transfers[scid].TransferE...)

	} else { // discard all changes, since we never write to store immediately, they are purged, however we need to  return any value associated
		err = fmt.Errorf("Discarded knowingly")
		return
	}

	//fmt.Printf("SC execution finished amount value %d\n", tx.Value)
	return

}

// reads SC, balance
func (chain *Blockchain) ReadSC(w_sc_tree *Tree_Wrapper, data_tree *Tree_Wrapper, scid crypto.Hash) (balance uint64, sc dvm.SmartContract, found bool) {
	meta_bytes, err := w_sc_tree.Get(SC_Meta_Key(scid))
	if err != nil {
		return
	}

	var meta SC_META_DATA // the meta contains the link to the SC bytes
	if err := meta.UnmarshalBinary(meta_bytes); err != nil {
		return
	}
	balance = meta.Balance

	sc_bytes, err := data_tree.Get(SC_Code_Key(scid))
	if err != nil {
		return
	}

	var v dvm.Variable

	if err = v.UnmarshalBinary(sc_bytes); err != nil {
		return
	}

	sc, pos, err := dvm.ParseSmartContract(v.Value.(string))
	if err != nil {
		return
	}

	_ = pos

	found = true
	return
}

func (chain *Blockchain) LoadSCValue(data_tree *Tree_Wrapper, scid crypto.Hash, key []byte) (v dvm.Variable, found bool) {
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

// reads a value from SC, always read balance
func (chain *Blockchain) ReadSCValue(data_tree *Tree_Wrapper, scid crypto.Hash, key interface{}) (value interface{}) {
	var keybytes []byte

	if key == nil {
		return
	}
	switch k := key.(type) {
	case uint64:
		keybytes = dvm.DataKey{Key: dvm.Variable{Type: dvm.Uint64, Value: k}}.MarshalBinaryPanic()
	case string:
		keybytes = dvm.DataKey{Key: dvm.Variable{Type: dvm.String, Value: k}}.MarshalBinaryPanic()
	case int64:
		keybytes = dvm.DataKey{Key: dvm.Variable{Type: dvm.String, Value: k}}.MarshalBinaryPanic()
	default:
		return
	}

	value_var, found := chain.LoadSCValue(data_tree, scid, keybytes)
	//fmt.Printf("read value %+v", value_var)
	if found && value_var.Type != dvm.Invalid {
		value = value_var.Value
	}
	return
}

// store the value in the chain
func (chain *Blockchain) StoreSCValue(data_tree *Tree_Wrapper, scid crypto.Hash, key, value []byte) {
	data_tree.Put(key, value)
	return
}
