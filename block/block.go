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

package block

import "fmt"

import "time"
import "bytes"
import "strings"
import "runtime/debug"
import "encoding/hex"
import "encoding/binary"

import "golang.org/x/crypto/sha3"

import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derosuite/config"
import "github.com/deroproject/derohe/transaction"

type Block struct {
	Major_Version uint64                  `json:"major_version"`
	Minor_Version uint64                  `json:"minor_version"`
	Timestamp     uint64                  `json:"timestamp"` // time stamp is now in milli seconds
	Height        uint64                  `json:"height"`
	Miner_TX      transaction.Transaction `json:"miner_tx"`

	Proof      [32]byte      `json:"-"` // proof is being used to record balance root hash
	Tips       []crypto.Hash `json:"tips"`
	MiniBlocks []MiniBlock   `json:"miniblocks"`
	Tx_hashes  []crypto.Hash `json:"tx_hashes"`
}

// we process incoming blocks in this format
type Complete_Block struct {
	Bl  *Block
	Txs []*transaction.Transaction
}

// this function gets the block identifier hash
// this has been simplified and varint length has been removed
// keccak hash of entire block including miniblocks, gives the block id
func (bl *Block) GetHash() (hash crypto.Hash) {
	return sha3.Sum256(bl.serialize(false))
}

func (bl *Block) GetHashSkipLastMiniBlock() (hash crypto.Hash) {
	return sha3.Sum256(bl.SerializeWithoutLastMiniBlock())
}

// serialize entire block ( block_header + miner_tx + tx_list )
func (bl *Block) Serialize() []byte {
	return bl.serialize(false) // include mini blocks
}

func (bl *Block) SerializeWithoutLastMiniBlock() []byte {
	return bl.serialize(true) //skip last mini block
}

// get timestamp, it has millisecond granularity
func (bl *Block) GetTimestamp() time.Time {
	return time.Unix(0, int64(bl.Timestamp*uint64(time.Millisecond)))
}

// stringifier
func (bl Block) String() string {
	r := new(strings.Builder)
	fmt.Fprintf(r, "BLID:%s\n", bl.GetHash())
	fmt.Fprintf(r, "Major version:%d Minor version: %d ", bl.Major_Version, bl.Minor_Version)
	fmt.Fprintf(r, "Height:%d ", bl.Height)
	fmt.Fprintf(r, "Timestamp:%d  (%s)\n", bl.Timestamp, bl.GetTimestamp())
	for i := range bl.Tips {
		fmt.Fprintf(r, "Past %d:%s\n", i, bl.Tips[i])
	}
	for i, mbl := range bl.MiniBlocks {
		fmt.Fprintf(r, "Mini %d:%s\n", i, mbl)
	}
	for i, txid := range bl.Tx_hashes {
		fmt.Fprintf(r, "tx %d:%s\n", i, txid)
	}
	return r.String()
}

// this function serializes a block and skips miniblocks is requested
func (bl *Block) serialize(skiplastminiblock bool) []byte {

	var serialized bytes.Buffer

	buf := make([]byte, binary.MaxVarintLen64)

	n := binary.PutUvarint(buf, uint64(bl.Major_Version))
	serialized.Write(buf[:n])

	n = binary.PutUvarint(buf, uint64(bl.Minor_Version))
	serialized.Write(buf[:n])

	binary.BigEndian.PutUint64(buf, bl.Timestamp)
	serialized.Write(buf[:8])

	n = binary.PutUvarint(buf, bl.Height)
	serialized.Write(buf[:n])

	// write miner address
	serialized.Write(bl.Miner_TX.Serialize())

	serialized.Write(bl.Proof[:])

	n = binary.PutUvarint(buf, uint64(len(bl.Tips)))
	serialized.Write(buf[:n])

	for _, hash := range bl.Tips {
		serialized.Write(hash[:])
	}

	if len(bl.MiniBlocks) == 0 {
		serialized.WriteByte(0)
	} else {
		if skiplastminiblock == false {
			n = binary.PutUvarint(buf, uint64(len(bl.MiniBlocks)))
			serialized.Write(buf[:n])

			for _, mblock := range bl.MiniBlocks {
				s := mblock.Serialize()
				serialized.Write(s[:])
			}
		} else {
			length := len(bl.MiniBlocks) - 1
			n = binary.PutUvarint(buf, uint64(length))
			serialized.Write(buf[:n])

			for i := 0; i < length; i++ {
				s := bl.MiniBlocks[i].Serialize()
				serialized.Write(s[:])
			}

		}
	}

	n = binary.PutUvarint(buf, uint64(len(bl.Tx_hashes)))
	serialized.Write(buf[:n])

	for _, hash := range bl.Tx_hashes {
		serialized.Write(hash[:])
	}

	return serialized.Bytes()

}

// get block transactions tree hash
func (bl *Block) GetTipsHash() (result crypto.Hash) {
	h := sha3.New256() // add all the remaining hashes
	for i := range bl.Tips {
		h.Write(bl.Tips[i][:])
	}
	r := h.Sum(nil)
	copy(result[:], r)
	return
}

// get block transactions
// we have discarded the merkle tree and have shifted to a plain version
func (bl *Block) GetTXSHash() (result crypto.Hash) {
	h := sha3.New256()
	for i := range bl.Tx_hashes {
		h.Write(bl.Tx_hashes[i][:])
	}
	r := h.Sum(nil)
	copy(result[:], r)

	return
}

//parse entire block completely
func (bl *Block) Deserialize(buf []byte) (err error) {
	done := 0

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Invalid Block cannot deserialize '%s' stack %s", hex.EncodeToString(buf), string(debug.Stack()))
			return
		}
	}()

	bl.Major_Version, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid Major Version in Block\n")
	}
	buf = buf[done:]

	bl.Minor_Version, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid Minor Version in Block\n")
	}
	buf = buf[done:]

	if len(buf) < 8 {
		return fmt.Errorf("Incomplete timestamp in Block\n")
	}

	bl.Timestamp = binary.BigEndian.Uint64(buf) // we have read 8 bytes
	buf = buf[8:]

	bl.Height, done = binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid Height in Block\n")
	}
	buf = buf[done:]

	// parse miner tx
	err = bl.Miner_TX.Deserialize(buf)
	if err != nil {
		return err
	}

	buf = buf[len(bl.Miner_TX.Serialize()):] // skup number of bytes processed

	// read 32 byte proof
	copy(bl.Proof[:], buf[0:32])
	buf = buf[32:]

	// header finished here

	// read and parse transaction
	/*err = bl.Miner_tx.DeserializeHeader(buf)

	if err != nil {
		return fmt.Errorf("Cannot parse miner TX  %x", buf)
	}

	// if tx was parse, make sure it's coin base
	if len(bl.Miner_tx.Vin) != 1 || bl.Miner_tx.Vin[0].(transaction.Txin_gen).Height > config.MAX_CHAIN_HEIGHT {
		// serialize transaction again to get the tx size, so as parsing could continue
		return fmt.Errorf("Invalid Miner TX")
	}

	miner_tx_serialized_size := bl.Miner_tx.Serialize()
	buf = buf[len(miner_tx_serialized_size):]
	*/

	tips_count, done := binary.Uvarint(buf)
	if done <= 0 || done > 1 {
		return fmt.Errorf("Invalid Tips count in Block\n")
	}
	buf = buf[done:]

	// remember first tx is merkle root

	for i := uint64(0); i < tips_count; i++ {
		//fmt.Printf("Parsing transaction hash %d  tx_count %d\n", i, tx_count)
		var h crypto.Hash
		copy(h[:], buf[:32])
		buf = buf[32:]

		bl.Tips = append(bl.Tips, h)

	}

	miniblocks_count, done := binary.Uvarint(buf)
	if done <= 0 || done > 2 {
		return fmt.Errorf("Invalid Mini blocks count in Block, done %d", done)
	}
	buf = buf[done:]

	for i := uint64(0); i < miniblocks_count; i++ {
		var mbl MiniBlock

		if err = mbl.Deserialize(buf[:MINIBLOCK_SIZE]); err != nil {
			return err
		}
		buf = buf[MINIBLOCK_SIZE:]

		bl.MiniBlocks = append(bl.MiniBlocks, mbl)

	}

	//fmt.Printf("miner tx %x\n", miner_tx_serialized_size)
	// read number of transactions
	tx_count, done := binary.Uvarint(buf)
	if done <= 0 {
		return fmt.Errorf("Invalid Tx count in Block\n")
	}
	buf = buf[done:]

	// remember first tx is merkle root

	for i := uint64(0); i < tx_count; i++ {
		//fmt.Printf("Parsing transaction hash %d  tx_count %d\n", i, tx_count)
		var h crypto.Hash
		copy(h[:], buf[:32])
		buf = buf[32:]

		bl.Tx_hashes = append(bl.Tx_hashes, h)

	}

	//fmt.Printf("%d member in tx hashes \n",len(bl.Tx_hashes))

	return

}
