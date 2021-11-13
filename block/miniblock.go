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
import "strings"
import "encoding/binary"

import "golang.org/x/crypto/sha3"

import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/pow"

const MINIBLOCK_SIZE = 68

// it should be exactly 68 bytes after serialization
// structure size 1 + 6 + 4 + 4 + 16 +32 + 5 bytes
type MiniBlock struct {
	// all the below 3 fields are serialized into single byte
	Version   uint8 // 1 byte    // lower 5 bits (0,1,2,3,4)
	Genesis   bool  // 1 bit flag // bits
	PastCount uint8 // previous  count  // bits 6,7

	Timestamp uint64 //  6 bytes  millisecond precision, serialized in 6 bytes,
	// can represent time till 2121-04-11 11:53:25
	Past [2]uint32 // 4 bytes used to build DAG of miniblocks and prevent number of attacks

	KeyHash crypto.Hash //  16 bytes, remaining bytes are trimmed miniblock miner keyhash
	Check   crypto.Hash // in non genesis,32 bytes this is  XOR of hash of TXhashes and block header hash
	// in genesis, this represents 8 bytes height, 12 bytes of first tip, 12 bytes of second tip
	Nonce [5]byte // 5 nonce byte represents 2^40 variations, 2^40 work every ms

	// below fields are never serialized and are placed here for easier processing/documentation
	Distance       uint32      // distance to tip block
	PastMiniBlocks []MiniBlock // pointers to past
	Height         int64       // which height

}

func (mbl MiniBlock) String() string {
	r := new(strings.Builder)

	fmt.Fprintf(r, "%08x %d ", mbl.GetMiniID(), mbl.Version)
	if mbl.Genesis {
		fmt.Fprintf(r, "GENESIS height %d", int64(binary.BigEndian.Uint64(mbl.Check[:])))
	} else {
		fmt.Fprintf(r, "NORMAL ")
	}

	if mbl.PastCount == 1 {
		fmt.Fprintf(r, " Past [%08x]", mbl.Past[0])
	} else {
		fmt.Fprintf(r, " Past [%08x %08x]", mbl.Past[0], mbl.Past[1])
	}
	fmt.Fprintf(r, " time %d", mbl.Timestamp)

	return r.String()
}

func (mbl *MiniBlock) GetTimestamp() time.Time {
	return time.Unix(0, int64(mbl.Timestamp*uint64(time.Millisecond)))
}

//func (mbl *MiniBlock) SetTimestamp(t time.Time) {
//	mbl.Timestamp = uint64(t.UTC().UnixMilli())
//}

func (mbl *MiniBlock) GetMiniID() uint32 {
	h := mbl.GetHash()
	return binary.BigEndian.Uint32(h[:])
}

// this function gets the block identifier hash, this is only used to deduplicate mini blocks
func (mbl *MiniBlock) GetHash() (hash crypto.Hash) {
	ser := mbl.Serialize()
	return sha3.Sum256(ser[:])
}

// Get PoW hash , this is very slow function
func (mbl *MiniBlock) GetPoWHash() (hash crypto.Hash) {
	return pow.Pow(mbl.Serialize())
}

func (mbl *MiniBlock) HasPid(pid uint32) bool {

	switch mbl.PastCount {
	case 0:
		return false
	case 1:
		if mbl.Past[0] == pid {
			return true
		} else {
			return false
		}

	case 2:
		if mbl.Past[0] == pid || mbl.Past[1] == pid {
			return true
		} else {
			return false
		}

	default:
		panic("not supported")
	}

}

func (mbl *MiniBlock) SanityCheck() error {
	if mbl.Version >= 32 {
		return fmt.Errorf("version not supported")
	}
	if mbl.PastCount > 2 {
		return fmt.Errorf("tips cannot be more than 2")
	}
	if mbl.PastCount == 0 {
		return fmt.Errorf("miniblock must have tips")
	}
	return nil
}

// serialize entire block ( 64 bytes )
func (mbl *MiniBlock) Serialize() (result []byte) {
	result = make([]byte, MINIBLOCK_SIZE, MINIBLOCK_SIZE)
	var scratch [8]byte
	if err := mbl.SanityCheck(); err != nil {
		panic(err)
	}

	result[0] = mbl.Version | mbl.PastCount<<6

	if mbl.Genesis {
		result[0] |= 1 << 5
	}

	binary.BigEndian.PutUint64(scratch[:], mbl.Timestamp)
	copy(result[1:], scratch[2:]) // 1 + 6

	for i, v := range mbl.Past {
		binary.BigEndian.PutUint32(result[7+i*4:], v)
	}

	copy(result[1+6+4+4:], mbl.KeyHash[:16])   // 1 + 6 + 4 + 4 + 16
	copy(result[1+6+4+4+16:], mbl.Check[:])    // 1 + 6 + 4 + 4 + 16 + 32
	copy(result[1+6+4+4+16+32:], mbl.Nonce[:]) // 1 + 6 + 4 + 4 + 16 + 32 + 5 = 68 bytes

	return result
}

//parse entire block completely
func (mbl *MiniBlock) Deserialize(buf []byte) (err error) {
	var scratch [8]byte

	if len(buf) < MINIBLOCK_SIZE {
		return fmt.Errorf("Expected %d bytes. Actual %d", MINIBLOCK_SIZE, len(buf))
	}

	if mbl.Version = buf[0] & 0x1f; mbl.Version != 1 {
		return fmt.Errorf("unknown version '%d'", mbl.Version)
	}

	mbl.PastCount = buf[0] >> 6
	if buf[0]&0x20 > 0 {
		mbl.Genesis = true
	}

	if err = mbl.SanityCheck(); err != nil {
		return err
	}

	if len(buf) != MINIBLOCK_SIZE {
		return fmt.Errorf("Expecting %d bytes", MINIBLOCK_SIZE)
	}

	copy(scratch[2:], buf[1:])
	mbl.Timestamp = binary.BigEndian.Uint64(scratch[:])

	for i := range mbl.Past {
		mbl.Past[i] = binary.BigEndian.Uint32(buf[7+i*4:])
	}

	copy(mbl.KeyHash[:], buf[15:15+16])
	copy(mbl.Check[:], buf[15+16:])
	copy(mbl.Nonce[:], buf[15+16+32:])
	mbl.Height = int64(binary.BigEndian.Uint64(mbl.Check[:]))

	if mbl.GetMiniID() == mbl.Past[0] {
		return fmt.Errorf("Self Collision")
	}
	if mbl.PastCount == 2 && mbl.GetMiniID() == mbl.Past[1] {
		return fmt.Errorf("Self Collision")
	}

	return
}
