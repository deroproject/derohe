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

import "fmt"
import "bytes"
import "math/big"
import "encoding/binary"

//import "github.com/romana/rlog"

import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/crypto/bn256"

type TransactionType uint64

const (
	PREMINE      TransactionType = iota // premine is represented by this
	REGISTRATION                        // registration tx are represented by this
	COINBASE                            // normal coinbase tx  ( if miner address is already registered)
	NORMAL                              // one to one TX with ring members
	BURN_TX                             // if user burns an amount to control inflation
	MULTIUSER_TX                        // multi-user transaction
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
	case MULTIUSER_TX:
		return "MULTIUSER_TX"
	case SC_TX:
		return "SMARTCONTRACT_TX"

	default:
		return "unknown transaction type"
	}
}

// the core transaction
type Transaction_Prefix struct {
	Version         uint64          `json:"version"`
	TransactionType TransactionType `json:"version"`
	Value           uint64          `json:"value"` // represnets premine, value for SC, BURN amount
	Amounts         []uint64        // open amounts for multi user tx
	MinerAddress    [33]byte        `json:"miner_address"` // miner address  // 33 bytes also used for registration
	C               [32]byte        `json:"c"`             // used for registration
	S               [32]byte        `json:"s"`             // used for registration
	Height          uint64          `json:"height"`        // height at the state, used to cross-check state
	PaymentID       [8]byte         `json:"paymentid"`     // hardcoded 8 bytes
}

type Transaction struct {
	Transaction_Prefix // same as Transaction_Prefix
	Statement          crypto.Statement
	Proof              *crypto.Proof
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

func (tx *Transaction) IsRegistrationValid() (result bool) {

	var u bn256.G1

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

func (tx *Transaction) DeserializeHeader(buf []byte) (err error) {
	var tmp_uint64 uint64
	var r *bytes.Reader
	tx.Clear() // clear existing

	done := 0
	tx.Version, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid Version in Transaction\n")
	}

	if tx.Version != 1 {
		return fmt.Errorf("Transaction version not equal to 1 \n")
	}

	buf = buf[done:]
	tmp_uint64, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid TransactionType in Transaction\n")
	}
	buf = buf[done:]
	tx.TransactionType = TransactionType(tmp_uint64)

	switch tx.TransactionType {
	case PREMINE:
		tx.Value, done = binary.Uvarint(buf)
		if done <= 0 {
			return fmt.Errorf("Invalid Premine value  in Transaction\n")
		}
		buf = buf[done:]
		if 33 != copy(tx.MinerAddress[:], buf[:]) {
			return fmt.Errorf("Invalid Miner Address in Transaction\n")
		}
		buf = buf[33:]
		goto done

	case REGISTRATION:
		if 33 != copy(tx.MinerAddress[:], buf[:33]) {
			return fmt.Errorf("Invalid Miner Address in Transaction\n")
		}
		buf = buf[33:]

		if 32 != copy(tx.C[:], buf[:32]) {
			return fmt.Errorf("Invalid C in Transaction\n")
		}
		buf = buf[32:]

		if 32 != copy(tx.S[:], buf[:32]) {
			return fmt.Errorf("Invalid S in Transaction\n")
		}
		buf = buf[32:]

		goto done

	case COINBASE:
		if 33 != copy(tx.MinerAddress[:], buf[:]) {
			return fmt.Errorf("Invalid Miner Address in Transaction\n")
		}
		buf = buf[33:]
		goto done

	case NORMAL: // parse height and root hash
		tx.Height, done = binary.Uvarint(buf)
		if done <= 0 {
			return fmt.Errorf("Invalid Height value  in Transaction\n")
		}
		buf = buf[done:]

		if len(buf) < 8 {
			return fmt.Errorf("Invalid payment id value  in Transaction\n")
		}
		copy(tx.PaymentID[:], buf[:8])
		buf = buf[8:]

		tx.Proof = &crypto.Proof{}

		r = bytes.NewReader(buf[:])

		tx.Statement.Deserialize(r)

		statement_size := len(buf) - r.Len()
		//	fmt.Printf("tx Statement size  deserialing %d\n", statement_size)

		//	fmt.Printf("tx Proof size  %d\n", len(buf) - statement_size)

		buf = buf[statement_size:]
		r = bytes.NewReader(buf[:])

		if err := tx.Proof.Deserialize(r, crypto.GetPowerof2(len(tx.Statement.Publickeylist_compressed))); err != nil {
			fmt.Printf("error deserialing proof err %s", err)
			return err
		}

		//	fmt.Printf("tx Proof size deserialed %d  bytes remaining %d \n", len(buf) - r.Len(), r.Len())

		if r.Len() != 0 {
			return fmt.Errorf("Extra unknown data in Transaction, extrabytes %d\n", r.Len())
		}

	case BURN_TX:
		panic("TODO")
	case MULTIUSER_TX:
		panic("TODO")
	case SC_TX:
		panic("TODO")

	default:
		panic("unknown transaction type")
	}

done:
	if len(buf) != 0 {
		//return fmt.Errorf("Extra unknown data in Transaction\n")
	}

	//rlog.Tracef(8, "TX deserialized %+v\n", tx)

	return nil //fmt.Errorf("Done Transaction\n")

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

	n = binary.PutUvarint(buf, uint64(tx.TransactionType))
	serialised_header.Write(buf[:n])

	switch tx.TransactionType {
	case PREMINE:
		n := binary.PutUvarint(buf, tx.Value)
		serialised_header.Write(buf[:n])
		serialised_header.Write(tx.MinerAddress[:])
		return serialised_header.Bytes()

	case REGISTRATION:
		serialised_header.Write(tx.MinerAddress[:])
		serialised_header.Write(tx.C[:])
		serialised_header.Write(tx.S[:])
		return serialised_header.Bytes()

	case COINBASE:
		serialised_header.Write(tx.MinerAddress[:])
		return serialised_header.Bytes()

	case NORMAL:
		n = binary.PutUvarint(buf, uint64(tx.Height))
		serialised_header.Write(buf[:n])
		serialised_header.Write(tx.PaymentID[:8]) // payment Id is always 8 bytes
		return serialised_header.Bytes()

	case BURN_TX:
		panic("TODO")
	case MULTIUSER_TX:
		panic("TODO")
	case SC_TX:
		panic("TODO")

	default:
		panic("unknown transaction type")
	}

	return serialised_header.Bytes()
}

// serialize entire transaction include signature
func (tx *Transaction) Serialize() []byte {

	var serialised bytes.Buffer

	header_bytes := tx.SerializeHeader()
	//base_bytes := tx.RctSignature.SerializeBase()
	//prunable := tx.RctSignature.SerializePrunable()
	serialised.Write(header_bytes)
	if tx.Proof != nil {

		//	done_bytes := serialised.Len()

		tx.Statement.Serialize(&serialised)
		//	statement_size := serialised.Len() - done_bytes
		//	fmt.Printf("tx statement_size serializing  %d\n", statement_size)

		//done_bytes =serialised.Len()
		tx.Proof.Serialize(&serialised)

		//	fmt.Printf("tx Proof serialised size %d\n", serialised.Len() - done_bytes)
	}

	return serialised.Bytes() //buf

}

// TXID excludes proof, rest everything is included
func (tx *Transaction) SerializeCoreStatement() []byte {
	var serialised bytes.Buffer
	header_bytes := tx.SerializeHeader()
	serialised.Write(header_bytes)

	switch tx.TransactionType {
	case PREMINE, REGISTRATION, COINBASE:
	case NORMAL, BURN_TX, MULTIUSER_TX, SC_TX:
		tx.Statement.Serialize(&serialised)

	default:
		panic("unknown transaction type")
	}

	return serialised.Bytes() //buf
}
