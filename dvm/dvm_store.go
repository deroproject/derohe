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
import "github.com/holiman/uint256"

// this package exports an interface which is used by blockchain to persist/query data

type DataKey struct {
	SCID    crypto.Hash // tx which created the the contract or contract ID
	Asset   crypto.Hash // used only if it repesebts a balane
	Balance bool        // whether this represents a balance
	Key     Variable
}

type TransferInternal struct {
	Asset  crypto.Hash `cbor:"Asset,omitempty" json:"Asset,omitempty"` //  transfer this asset
	SCID   string      `cbor:"A,omitempty" json:"A,omitempty"`         //  transfer to this SCID
	Amount uint64      `cbor:"V,omitempty" json:"V,omitempty"`         // Amount in Atomic units
}

// any external tranfers
type TransferExternal struct {
	Asset   crypto.Hash `cbor:"Asset,omitempty" json:"Asset,omitempty"` //  transfer this asset
	Address string      `cbor:"A,omitempty" json:"A,omitempty"`         //  transfer to this address 33 bytes
	Amount  uint64      `cbor:"V,omitempty" json:"V,omitempty"`         // Amount in Atomic units
}

type SC_Transfers struct {
	BalanceAtStart uint64             // value at start
	TransferI      []TransferInternal // all internal transfers, SC to other SC
	TransferE      []TransferExternal // all external transfers, SC to external wallets
}

// all SC load and store operations will go though this
type TX_Storage struct {
	DiskLoader     func(DataKey, *uint64) Variable // used to load variabled
	BalanceLoader  func(DataKey) uint64            // used to load balance
	DiskLoaderRaw  func([]byte) ([]byte, bool)
	SCID           crypto.Hash
	BalanceAtStart uint64            // at runtime this will be fed balance
	RawKeys        map[string][]byte // this keeps the in-transit DB updates, just in case we have to discard instantly

	Transfers map[crypto.Hash]SC_Transfers // all transfers ( internal/external )

	State *Shared_State // only for book keeping of storage gas
}

// initialize tx store
func Initialize_TX_store() (tx_store *TX_Storage) {
	tx_store = &TX_Storage{RawKeys: map[string][]byte{}, Transfers: map[crypto.Hash]SC_Transfers{}}
	return
}

func (tx_store *TX_Storage) RawLoad(key []byte) (value []byte, found bool) {
	if value, found = tx_store.RawKeys[string(key)]; !found {
		if tx_store.DiskLoaderRaw == nil {
			return
		}
		value, found = tx_store.DiskLoaderRaw(key)
	}
	return
}

func (tx_store *TX_Storage) Delete(dkey DataKey) {
	tx_store.RawKeys[string(dkey.MarshalBinaryPanic())] = []byte{}
	return
}

// this will load the variable, and if the key is found
// loads are cheaper
func (tx_store *TX_Storage) Load(dkey DataKey, found_value *uint64) (value Variable) {

	//fmt.Printf("Loading %+v   \n", dkey)

	*found_value = 0
	if result, ok := tx_store.RawKeys[string(dkey.MarshalBinaryPanic())]; ok { // if it was modified in current TX, use it
		*found_value = 1

		if err := value.UnmarshalBinary(result); err != nil {
			panic(err)
		}

		if tx_store.State != nil {
			if value.Length() > 10 {
				tx_store.State.ConsumeStorageGas(value.Length() / 10)
			} else {
				tx_store.State.ConsumeStorageGas(1)
			}
		}

		return value
	}

	if tx_store.DiskLoader == nil {
		panic("DVM_STORAGE_BACKEND is not ready")
	}

	value = tx_store.DiskLoader(dkey, found_value)
	if tx_store.State != nil {
		if value.Length() > 10 {
			tx_store.State.ConsumeStorageGas(value.Length() / 10)
		} else {
			tx_store.State.ConsumeStorageGas(1)
		}
	}

	return
}

// store variable
func (tx_store *TX_Storage) Store(dkey DataKey, v Variable) {
	//fmt.Printf("Storing request %+v   : %+v\n", dkey, v)

	kbytes := dkey.MarshalBinaryPanic()
	vbytes := v.MarshalBinaryPanic()
	tx_store.State.ConsumeStorageGas(int64(len(vbytes)) * 1)
	tx_store.RawKeys[string(kbytes)] = vbytes
}

// store variable
func (tx_store *TX_Storage) SendExternal(sender_scid, asset crypto.Hash, addr_str string, amount uint64) {
	//fmt.Printf("Transfer to  external address   : %+v\n", addr_str)
	transfer := tx_store.Transfers[sender_scid]
	transfer.TransferE = append(transfer.TransferE, TransferExternal{Address: addr_str, Asset: asset, Amount: amount})
	tx_store.Transfers[sender_scid] = transfer

}

func GetBalanceKey(scid, asset crypto.Hash) (x DataKey) {
	x.SCID = scid
	x.Balance = true
	x.Key = Variable{Type: String, ValueString: string(asset[:])}
	return x
}

/*
func GetNormalKey(scid crypto.Key,  v Variable) (x DataKey) {
    x.SCID = scid
    x.Key = Variable {Type:v.Type, Value: v.Value}
    return x
}
*/

// why should we not hash the return value to return a hash value
// using entire key could be useful, if DB can somehow link between  them in the form of buckets and all
func (dkey DataKey) MarshalBinary() (ser []byte, err error) {
	ser, err = dkey.Key.MarshalBinary()
	return
}

func (dkey DataKey) MarshalBinaryPanic() (ser []byte) {
	var err error

	if dkey.Balance {
		switch dkey.Key.Type {
		case String:
			ser = append(ser, ([]byte(dkey.Key.ValueString))...) // string
			return
		default:

			panic("balance keys can only be string")
		}
	}
	if ser, err = dkey.Key.MarshalBinary(); err != nil {
		panic(err)
	}
	return
}

func (v Variable) Length() (length int64) {
	switch v.Type {
	case Invalid, None:
		return
	case Uint64:
		var buf [binary.MaxVarintLen64]byte
		done := binary.PutUvarint(buf[:], v.ValueUint64) // uint64 data type
		length += int64(done) + 1
	case String:
		length = int64(len([]byte(v.ValueString)) + 1)
	case Uint256:
		length = int64(32) + 1
	default:
		panic("unknown variable type not implemented")
	}
	return
}

// these are used by lowest layers
func (v Variable) MarshalBinary() (data []byte, err error) {
	switch v.Type {
	case Invalid, None:
		return
	case Uint64:
		var buf [binary.MaxVarintLen64]byte
		done := binary.PutUvarint(buf[:], v.ValueUint64) // uint64 data type
		data = append(data, buf[:done]...)
	case String:
		data = append(data, ([]byte(v.ValueString))...) // string
	case Uint256:
		data = (&v.ValueUint256).Bytes()

	default:
		panic("unknown variable type not implemented2")
	}
	data = append(data, byte(v.Type)) // add object type
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
	if len(buf) < 1 {
		return fmt.Errorf("invalid, probably corruption")
	}

	switch Vtype(buf[len(buf)-1]) {
	case Invalid, None:
		return fmt.Errorf("Invalid cannot be deserialized")
	case Uint64:
		v.Type = Uint64
		var n int
		v.ValueUint64, n = binary.Uvarint(buf[:len(buf)-1]) // uint64 data type
		if n <= 0 {
			panic("corruption in DB")
			return fmt.Errorf("corruption in DB")
		}
	case String:
		v.Type = String
		v.ValueString = string(buf[:len(buf)-1])
		return nil
	case Uint256:
		v.Type = Uint256
		v.ValueUint256 = *uint256.NewInt(0).SetBytes(buf[:len(buf)-1])
		return nil

	default:
		panic("unknown variable type not implemented3")

	}
	return
}
