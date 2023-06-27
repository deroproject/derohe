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

package transaction

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
)

type TransactionType uint64

const (
	PREMINE      TransactionType = iota // premine is represented by this
	REGISTRATION                        // registration tx are represented by this
	COINBASE                            // normal coinbase tx  ( if miner address is already registered)
	NORMAL                              // one to one TX with ring members
	BURN_TX                             // if user burns an amount to control inflation
	SC_TX                               // smart contract transaction

)

func (t TransactionType) String() string {
	switch t {
	case PREMINE:
		return "PREMINE"
	case REGISTRATION:
		return "REGISTRATION"
	case COINBASE:
		return "COINBASE"
	case NORMAL:
		return "NORMAL"
	case BURN_TX:
		return "BURN"
	case SC_TX:
		return "SC"

	default:
		return "unknown transaction type"
	}
}

const PAYLOAD_LIMIT = 1 + 144 // entire payload header is mandatorily encrypted
// sender position in ring representation in a byte, uptp 256 ring
// 144 byte payload  ( to implement specific functionality such as delivery of keys etc), user dependent encryption
const PAYLOAD0_LIMIT = 144 // 1 byte has been reserved for sender position in ring representation in a byte, uptp 256 ring

const ENCRYPTED_DEFAULT_PAYLOAD_CBOR = byte(0)

// the core transaction
// in our design, tx cam be sent by 1 wallet, but SC part/gas can be signed by any other user, but this is not implemented
type Transaction_Prefix struct {
	Version uint64 `json:"version"`

	SourceNetwork uint64 `json:"source_network"` // which network originated the transaction acting as source
	DestNetwork   uint64 `json:"dest_network"`   // which network is acting as sink
	// other networks may work with 0 fees and custom block time of < 1 sec
	// above protocol which is majorly complete would be enough for entire EARTH !!
	// thereby representing immense scalability and privacy both at the same time
	// default dero network has id 0

	TransactionType TransactionType `json:"txtype"`

	Value        uint64        `json:"value"`         // represents value for premine, Gas for SC, BURN transactions
	MinerAddress [33]byte      `json:"miner_address"` // miner address  // 33 bytes also used for registration
	C            [32]byte      `json:"c"`             // used for registration
	S            [32]byte      `json:"s"`             // used for registration
	Height       uint64        `json:"height"`        // height at the state, used to cross-check state
	BLID         [32]byte      `json:"blid"`          // which is used to build the tx
	SCDATA       rpc.Arguments `json:"scdata"`        // all SC related data is provided here, an SC tx uses all the fields
}

type AssetPayload struct {
	SCID      crypto.Hash // which asset, it's zero for main asset
	BurnValue uint64      `json:"value"` // represents value for premine, SC, BURN transactions

	RPCType    byte   // its unencrypted  and is by default 0 for almost all txs
	RPCPayload []byte // rpc payload encryption depends on RPCType

	// sender position in ring representation in a byte, uptp 256 ring
	// 144 byte payload  ( to implement specific functionality such as delivery of keys etc), user dependent encryption
	Statement crypto.Statement // note statement containts fees
	Proof     *crypto.Proof
}

// marshal asset
/*
func (a AssetPayload) MarshalHeader() ([]byte, error) {

	return writer.Bytes(), nil
}
*/

func (a AssetPayload) MarshalHeaderStatement() ([]byte, error) {
	var writer bytes.Buffer
	var buffer_backing [binary.MaxVarintLen64]byte
	buf := buffer_backing[:]
	_ = buf

	n := binary.PutUvarint(buf, a.BurnValue)
	writer.Write(buf[:n])

	writer.Write(a.SCID[:])

	writer.WriteByte(a.RPCType) // payload type byte
	writer.Write(a.RPCPayload)  // src Id is always payload limit bytes

	if len(a.RPCPayload) != PAYLOAD_LIMIT {
		return nil, fmt.Errorf("RPCPayload should be %d bytes, but have %d bytes", PAYLOAD_LIMIT, len(a.RPCPayload))
	}

	a.Statement.Serialize(&writer)
	//if  err != nil {
	//	return nil,err
	//}

	return writer.Bytes(), nil
}

func (a *AssetPayload) UnmarshalHeaderStatement(r *bytes.Reader) (err error) {
	if a.BurnValue, err = binary.ReadUvarint(r); err != nil {
		return err
	}
	if _, err = r.Read(a.SCID[:]); err != nil {
		return err
	}
	if a.RPCType, err = r.ReadByte(); err != nil {
		return err
	}

	a.RPCPayload = make([]byte, PAYLOAD_LIMIT, PAYLOAD_LIMIT)
	if _, err = r.Read(a.RPCPayload[:]); err != nil {
		return err
	}

	if err = a.Statement.Deserialize(r); err != nil {
		return err
	}

	return nil
}

func (a AssetPayload) MarshalProofs() ([]byte, error) {
	var writer bytes.Buffer
	a.Proof.Serialize(&writer)

	return writer.Bytes(), nil
}

func (a *AssetPayload) UnmarshalProofs(r *bytes.Reader) (err error) {
	a.Proof = &crypto.Proof{}

	if err = a.Proof.Deserialize(r, crypto.GetPowerof2(len(a.Statement.Publickeylist_pointers)/int(a.Statement.Bytes_per_publickey))); err != nil {
		return err
	}

	return nil

}

type Transaction struct {
	Transaction_Prefix // same as Transaction_Prefix

	Payloads []AssetPayload // each transaction can have a number of payloads
}

// this excludes the proof part, so it can pruned
func (tx *Transaction) GetHash() (result crypto.Hash) {
	switch tx.Version {
	case 1:
		result = crypto.Hash(crypto.Keccak256(tx.SerializeCoreStatement()))
	default:
		panic("Transaction version unknown")

	}
	return
}

// returns whether the tx is coinbase
func (tx *Transaction) IsCoinbase() (result bool) {
	return tx.TransactionType == COINBASE
}

func (tx *Transaction) IsRegistration() (result bool) {
	return tx.TransactionType == REGISTRATION
}

func (tx *Transaction) IsPremine() (result bool) {
	return tx.TransactionType == PREMINE
}

func (tx *Transaction) IsSC() (result bool) {
	return tx.TransactionType == SC_TX
}

// if external proof is required
func (tx *Transaction) IsProofRequired() (result bool) {
	return (tx.IsCoinbase() || tx.IsRegistration() || tx.IsPremine()) == false

}

func (tx *Transaction) Fees() (fees uint64) {
	var zero_scid [32]byte
	for i := range tx.Payloads {
		if zero_scid == tx.Payloads[i].SCID {
			fees += tx.Payloads[i].Statement.Fees
		}
	}
	return fees
}

// tx storage gas
func (tx *Transaction) GasStorage() (fees uint64) {
	return tx.Fees()
	/*
	       if !tx.IsSC(){
	           return 0
	       }
	   	var zero_scid [32]byte
	       count := 0
	   	for i := range tx.Payloads {
	   		if zero_scid == tx.Payloads[i].SCID {
	   			count++
	   		}
	   	}
	       if count == 1 {
	           return tx.Value
	       }
	   	return 0
	*/
}

func (tx *Transaction) IsRegistrationValid() (result bool) {

	var u bn256.G1

	if tx.TransactionType != REGISTRATION {
		return false
	}

	if err := u.DecodeCompressed(tx.MinerAddress[0:33]); err != nil {
		return false
	}

	s := new(big.Int).SetBytes(tx.S[:])
	c := new(big.Int).SetBytes(tx.C[:])

	tmppoint := new(bn256.G1).Add(new(bn256.G1).ScalarMult(crypto.G, s), new(bn256.G1).ScalarMult(&u, new(big.Int).Neg(c)))
	serialize := []byte(fmt.Sprintf("%s%s", u.String(), tmppoint.String()))

	c_calculated := crypto.ReducedHash(serialize)
	if c.String() == c_calculated.String() {
		return true
	}
	//return fmt.Errorf("Registration signature is invalid")
	return false
}

func (tx *Transaction) Deserialize(buf []byte) (err error) {
	var tmp_uint64 uint64
	var r *bytes.Reader
	tx.Clear() // clear existing

	done := 0
	tx.Version, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid Version in Transaction\n")
	}
	buf = buf[done:]

	if tx.Version != 1 {
		return fmt.Errorf("Transaction version not equal to 1 \n")
	}

	tx.SourceNetwork, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid SourceNetwork in Transaction\n")
	}
	buf = buf[done:]

	tx.DestNetwork, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid DestNetwork in Transaction\n")
	}
	buf = buf[done:]

	tmp_uint64, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid TransactionType in Transaction\n")
	}
	buf = buf[done:]
	tx.TransactionType = TransactionType(tmp_uint64)

	switch tx.TransactionType {
	case PREMINE, REGISTRATION, COINBASE, BURN_TX, NORMAL, SC_TX:
	default:
		panic("unknown transaction type")
	}

	if tx.TransactionType == PREMINE || tx.TransactionType == SC_TX { // represents Gas in SC tx
		tx.Value, done = binary.Uvarint(buf)
		if done <= 0 {
			return fmt.Errorf("Invalid Premine value  in Transaction\n")
		}
		buf = buf[done:]
	}

	if tx.TransactionType == PREMINE || tx.TransactionType == COINBASE || tx.TransactionType == REGISTRATION {
		if 33 != copy(tx.MinerAddress[:], buf[:]) {
			return fmt.Errorf("Invalid Miner Address in Transaction\n")
		}
		buf = buf[33:]
	}

	if tx.TransactionType == REGISTRATION {
		if 32 != copy(tx.C[:], buf[:32]) {
			return fmt.Errorf("Invalid C in Transaction\n")
		}
		buf = buf[32:]

		if 32 != copy(tx.S[:], buf[:32]) {
			return fmt.Errorf("Invalid S in Transaction\n")
		}
		buf = buf[32:]
	}

	if tx.TransactionType == BURN_TX || tx.TransactionType == NORMAL || tx.TransactionType == SC_TX {
		// parse height and root hash
		tx.Height, done = binary.Uvarint(buf)
		if done <= 0 {
			return fmt.Errorf("Invalid Height value  in Transaction\n")
		}
		buf = buf[done:]
		if len(buf) < 32 {
			return fmt.Errorf("Invalid BLID value  in Transaction\n")
		}
		copy(tx.BLID[:], buf[:32])
		buf = buf[32:]

		var asset_count uint64
		asset_count, done = binary.Uvarint(buf)
		if done <= 0 || asset_count < 1 {
			return fmt.Errorf("Invalid asset_count  in Transaction\n")
		}
		buf = buf[done:]

		for i := uint64(0); i < asset_count; i++ {
			var a AssetPayload

			r = bytes.NewReader(buf[:])
			if err = a.UnmarshalHeaderStatement(r); err != nil {
				panic(err)
			}
			tx.Payloads = append(tx.Payloads, a)
			buf = buf[len(buf)-r.Len():]
		}

	}

	if tx.TransactionType == SC_TX {
		var sc_len uint64
		sc_len, done = binary.Uvarint(buf)
		if done <= 0 {
			return fmt.Errorf("Invalid sc length  in Transaction\n")
		}
		buf = buf[done:]
		if sc_len > uint64(len(buf)) { // we are are crossing tx_boundary
			return fmt.Errorf("SC len out of possible range")
		}
		if err := tx.SCDATA.UnmarshalBinary(buf[:sc_len]); err != nil {
			return err
		}
		buf = buf[sc_len:]
	}

	if tx.TransactionType == BURN_TX || tx.TransactionType == NORMAL || tx.TransactionType == SC_TX {

		for i := range tx.Payloads {
			tx.Payloads[i].Proof = &crypto.Proof{}

			r = bytes.NewReader(buf[:])
			if err = tx.Payloads[i].UnmarshalProofs(r); err != nil {
				panic(err)
			}
			buf = buf[len(buf)-r.Len():]
		}
	}

	if len(buf) != 0 && !(tx.TransactionType == PREMINE || tx.TransactionType == COINBASE) { // these tx are complete
		return fmt.Errorf("Extra unknown data in Transaction, extrabytes %d\n", len(buf))
	}

	return nil
}

// // clean the transaction everything
func (tx *Transaction) Clear() {
	tx = &Transaction{}
}

func (tx *Transaction) SerializeHeader() []byte {

	var serialised_header bytes.Buffer

	var buffer_backing [binary.MaxVarintLen64]byte

	buf := buffer_backing[:]

	n := binary.PutUvarint(buf, tx.Version)
	serialised_header.Write(buf[:n])

	n = binary.PutUvarint(buf, tx.SourceNetwork)
	serialised_header.Write(buf[:n])

	n = binary.PutUvarint(buf, tx.DestNetwork)
	serialised_header.Write(buf[:n])

	switch tx.TransactionType {
	case PREMINE, REGISTRATION, COINBASE, BURN_TX, NORMAL, SC_TX:
	default:
		panic("unknown transaction type")
	}

	n = binary.PutUvarint(buf, uint64(tx.TransactionType))
	serialised_header.Write(buf[:n])

	if tx.TransactionType == PREMINE || tx.TransactionType == SC_TX {
		n := binary.PutUvarint(buf, tx.Value)
		serialised_header.Write(buf[:n])
	}
	if tx.TransactionType == PREMINE || tx.TransactionType == COINBASE || tx.TransactionType == REGISTRATION {
		serialised_header.Write(tx.MinerAddress[:])
	}
	if tx.TransactionType == REGISTRATION {
		serialised_header.Write(tx.C[:])
		serialised_header.Write(tx.S[:])
	}

	if tx.TransactionType == BURN_TX || tx.TransactionType == NORMAL || tx.TransactionType == SC_TX {
		n = binary.PutUvarint(buf, uint64(tx.Height))
		serialised_header.Write(buf[:n])
		serialised_header.Write(tx.BLID[:])

		n = binary.PutUvarint(buf, uint64(len(tx.Payloads)))
		serialised_header.Write(buf[:n])

		for _, p := range tx.Payloads {
			if pheader_bytes, err := p.MarshalHeaderStatement(); err == nil {
				serialised_header.Write(pheader_bytes)
			} else {
				panic(err)
			}
		}

	}

	if tx.TransactionType == SC_TX {
		if data, err := tx.SCDATA.MarshalBinary(); err != nil {
			panic(err)
		} else {
			n = binary.PutUvarint(buf, uint64(len(data)))
			serialised_header.Write(buf[:n])
			serialised_header.Write(data)
		}
	}

	return serialised_header.Bytes()
}

// serialize entire transaction include signature
func (tx *Transaction) Serialize() []byte {
	var serialised bytes.Buffer
	header_bytes := tx.SerializeHeader()
	serialised.Write(header_bytes)
	for _, p := range tx.Payloads {
		if pheader_bytes, err := p.MarshalProofs(); err == nil {
			serialised.Write(pheader_bytes)
		} else {
			panic(err)
		}
	}
	return serialised.Bytes() //buf
}

// TXID excludes proof, rest everything is included
func (tx *Transaction) SerializeCoreStatement() []byte {
	var serialised bytes.Buffer
	header_bytes := tx.SerializeHeader()
	serialised.Write(header_bytes)

	switch tx.TransactionType {
	case PREMINE, COINBASE:
	case REGISTRATION:
	case NORMAL, BURN_TX, SC_TX:
	default:
		panic("unknown transaction type")
	}

	return serialised.Bytes() //buf
}
