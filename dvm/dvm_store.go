// Copyright 2017-2018 DERO Project. All rights reserved.
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

import "fmt"
import "encoding/binary"
import "github.com/deroproject/derohe/cryptography/crypto"

const DVAL = "DERO_BALANCE" // DERO Values are stored in this variable
const CHANGELOG = "CHANGELOG"

// this package exports an interface which is used by blockchain to persist/query data

type DataKey struct {
	SCID crypto.Hash // tx which created the the contract or contract ID
	Key  Variable
}

type DataAtom struct {
	Key DataKey

	Prev_Value Variable // previous Value if any
	Value      Variable // current value if any
}

type TransferInternal struct {
	Received []uint64
	Sent     []uint64
}

// any external tranfers
type TransferExternal struct {
	Address string `cbor:"A,omitempty" json:"A,omitempty"` //  transfer to this blob
	Amount  uint64 `cbor:"V,omitempty" json:"V,omitempty"` // Amount in Atomic units
}

type SC_Transfers struct {
	BalanceAtStart uint64             // value at start
	TransferI      TransferInternal   // all internal transfers, SC to other SC
	TransferE      []TransferExternal // all external transfers, SC to external wallets
}

// all SC load and store operations will go though this
type TX_Storage struct {
	DiskLoader     func(DataKey, *uint64) Variable
	DiskLoaderRaw  func([]byte) ([]byte, bool)
	SCID           crypto.Hash
	BalanceAtStart uint64               // at runtime this will be fed balance
	Keys           map[DataKey]Variable // this keeps the in-transit DB updates, just in case we have to discard instantly
	RawKeys        map[string][]byte

	Transfers map[crypto.Hash]SC_Transfers // all transfers ( internal/external )
}

var DVM_STORAGE_BACKEND DVM_Storage_Loader // this variable can be hijacked at runtime to offer different stores such as RAM/file/DB etc

type DVM_Storage_Loader interface {
	Load(DataKey, *uint64) Variable
	Store(DataKey, Variable)
	RawLoad([]byte) ([]byte, bool)
	RawStore([]byte, []byte)
}

// initialize tx store
func Initialize_TX_store() (tx_store *TX_Storage) {
	tx_store = &TX_Storage{Keys: map[DataKey]Variable{}, RawKeys: map[string][]byte{}, Transfers: map[crypto.Hash]SC_Transfers{}}
	return
}

func (tx_store *TX_Storage) RawLoad(key []byte) (value []byte, found bool) {
	value, found = tx_store.RawKeys[string(key)]

	if !found {
		if tx_store.DiskLoaderRaw == nil {
			return
		}
		value, found = tx_store.DiskLoaderRaw(key)
	}
	return
}

func (tx_store *TX_Storage) RawStore(key []byte, value []byte) {
	tx_store.RawKeys[string(key)] = value
	return
}

// this will load the variable, and if the key is found
func (tx_store *TX_Storage) Load(dkey DataKey, found_value *uint64) (value Variable) {

	//fmt.Printf("Loading %+v   \n", dkey)

	*found_value = 0
	if result, ok := tx_store.Keys[dkey]; ok { // if it was modified in current TX, use it
		*found_value = 1
		return result
	}

	if tx_store.DiskLoader == nil {
		panic("DVM_STORAGE_BACKEND is not ready")
	}

	value = tx_store.DiskLoader(dkey, found_value)

	return
}

// store variable
func (tx_store *TX_Storage) Store(dkey DataKey, v Variable) {
	//fmt.Printf("Storing request %+v   : %+v\n", dkey, v)

	tx_store.Keys[dkey] = v
}

// store variable
func (tx_store *TX_Storage) SendExternal(sender_scid crypto.Hash, addr_str string, amount uint64) {

	//fmt.Printf("Transfer to  external address   : %+v\n", addr_str)

	tx_store.Balance(sender_scid) // load from disk if required
	transfer := tx_store.Transfers[sender_scid]
	transfer.TransferE = append(transfer.TransferE, TransferExternal{Address: addr_str, Amount: amount})
	tx_store.Transfers[sender_scid] = transfer
	tx_store.Balance(sender_scid) //  recalculate balance panic if any issues

}

// if TXID is not already loaded, load it
func (tx_store *TX_Storage) ReceiveInternal(scid crypto.Hash, amount uint64) {

	tx_store.Balance(scid) // load from disk if required
	transfer := tx_store.Transfers[scid]
	transfer.TransferI.Received = append(transfer.TransferI.Received, amount)
	tx_store.Transfers[scid] = transfer
	tx_store.Balance(scid) //  recalculate balance panic if any issues
}

func (tx_store *TX_Storage) SendInternal(sender_scid crypto.Hash, receiver_scid crypto.Hash, amount uint64) {

	//sender side
	{
		tx_store.Balance(sender_scid) // load from disk if required
		transfer := tx_store.Transfers[sender_scid]
		transfer.TransferI.Sent = append(transfer.TransferI.Sent, amount)
		tx_store.Transfers[sender_scid] = transfer
		tx_store.Balance(sender_scid) //  recalculate balance panic if any issues
	}

	{
		tx_store.Balance(receiver_scid) // load from disk if required
		transfer := tx_store.Transfers[receiver_scid]
		transfer.TransferI.Received = append(transfer.TransferI.Received, amount)
		tx_store.Transfers[receiver_scid] = transfer
		tx_store.Balance(receiver_scid) //  recalculate balance panic if any issues
	}

}

func GetBalanceKey(scid crypto.Hash) (x DataKey) {
	x.SCID = scid
	x.Key = Variable{Type: String, Value: DVAL}
	return x
}

/*
func GetNormalKey(scid crypto.Key,  v Variable) (x DataKey) {
    x.SCID = scid
    x.Key = Variable {Type:v.Type, Value: v.Value}
    return x
}
*/

// this will give the balance, will load the balance from disk
func (tx_store *TX_Storage) Balance(scid crypto.Hash) uint64 {

	if scid != tx_store.SCID {
		fmt.Printf("scid %s  SCID %s\n", scid, tx_store.SCID)
		fmt.Printf("tx_store internal %+v\n", tx_store)
		panic("cross SC balance calls are not supported")
	}
	if _, ok := tx_store.Transfers[scid]; !ok {

		var transfer SC_Transfers
		/*
			        found_value := uint64(0)
					value := tx_store.Load(GetBalanceKey(scid), &found_value)

					if found_value == 0 {
						panic(fmt.Sprintf("SCID %s is not loaded", scid)) // we must load  it from disk
					}

					if value.Type != Uint64 {
						panic(fmt.Sprintf("SCID %s balance is not uint64, HOW ??", scid)) // we must load  it from disk
					}
		*/

		transfer.BalanceAtStart = tx_store.BalanceAtStart
		tx_store.Transfers[scid] = transfer
	}

	transfers := tx_store.Transfers[scid]
	balance := transfers.BalanceAtStart

	// replay all receives/sends

	//  handle all internal receives
	for _, amt_received := range transfers.TransferI.Received {
		c := balance + amt_received

		if c >= balance {
			balance = c
		} else {
			panic("uint64 overflow wraparound attack")
		}
	}

	// handle all internal sends
	for _, amt_sent := range transfers.TransferI.Sent {
		if amt_sent >= balance {
			panic("uint64 underflow wraparound attack")
		}
		balance = balance - amt_sent
	}

	// handle all external sends
	for _, trans := range transfers.TransferE {
		if trans.Amount >= balance {
			panic("uint64 underflow wraparound attack")
		}
		balance = balance - trans.Amount
	}

	return balance
}

// whether the scid has enough balance
func (tx_store *TX_Storage) HasBalance(scid crypto.Key, amount uint64) {

}

// why should we not hash the return value to return a hash value
// using entire key could be useful, if DB can somehow link between  them in the form of buckets and all
func (dkey DataKey) MarshalBinary() (ser []byte, err error) {
	ser, err = dkey.Key.MarshalBinary()
	return
}

func (dkey DataKey) MarshalBinaryPanic() (ser []byte) {
	var err error
	if ser, err = dkey.Key.MarshalBinary(); err != nil {
		panic(err)
	}
	return
}

// these are used by lowest layers
func (v Variable) MarshalBinary() (data []byte, err error) {
	data = append(data, byte(v.Type)) // add object type
	switch v.Type {
	case Invalid:
		return
	case Uint64:
		var buf [binary.MaxVarintLen64]byte
		done := binary.PutUvarint(buf[:], v.Value.(uint64)) // uint64 data type
		data = append(data, buf[:done]...)
	case Blob:
		panic("not implemented")
	case String:
		data = append(data, ([]byte(v.Value.(string)))...) // string
	default:
		panic("unknown variable type not implemented")
	}
	return
}
func (v Variable) MarshalBinaryPanic() (ser []byte) {
	var err error
	if ser, err = v.MarshalBinary(); err != nil {
		panic(err)
	}
	return
}

func (v *Variable) UnmarshalBinary(buf []byte) (err error) {
	if len(buf) < 1 || Vtype(buf[0]) == Invalid {
		return fmt.Errorf("invalid, probably corruption")
	}

	switch Vtype(buf[0]) {
	case Invalid:
		return fmt.Errorf("Invalid cannot be deserialized")
	case Uint64:
		v.Type = Uint64
		var n int
		v.Value, n = binary.Uvarint(buf[1:]) // uint64 data type
		if n <= 0 {
			panic("corruption in DB")
			return fmt.Errorf("corruption in DB")
		}
	case String:
		v.Type = String
		v.Value = string(buf[1:])
		return nil
	case Blob:
		panic("blob not implemented") // an encrypted blob, used to add data to blockchain without knowing address

	default:
		panic("unknown variable type not implemented")

	}
	return
}
