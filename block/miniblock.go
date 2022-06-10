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
import "hash"
import "sync"
import "bytes"
import "strings"
import "encoding/binary"

import "golang.org/x/crypto/sha3"

import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/astrobwt"
import "github.com/deroproject/derohe/astrobwt/astrobwtv3"
import "github.com/deroproject/derohe/globals"

const MINIBLOCK_SIZE = 48

var hasherPool = sync.Pool{
	New: func() interface{} { return sha3.New256() },
}

// it should be exactly 48 bytes after serialization
// structure size 1 + 2 + 5 + 8 + 16 + 16 bytes
type MiniBlock struct {
	//  below 3 fields are serialized into single byte
	Version   uint8 // 1 byte    // lower 5 bits (0,1,2,3,4)
	HighDiff  bool  // bit 4  // triggers high diff
	Final     bool  // bit 5
	PastCount uint8 // previous  count  // bits 6,7

	Timestamp uint16 // represents rolling time
	Height    uint64 //  5 bytes  serialized in 5 bytes,

	Past [2]uint32 // 8 bytes used to build DAG of miniblocks and prevent number of attacks

	KeyHash crypto.Hash //  16 bytes, remaining bytes are trimmed miniblock miner keyhash

	Flags uint32    // can be used as flags by special miners to represent something, also used as nonce
	Nonce [3]uint32 // 12 nonce byte represents 2^96 variations, 2^96 work every ms
}

type MiniBlockKey struct {
	Height uint64
	Past0  uint32
	Past1  uint32
}

func (mbl *MiniBlock) GetKey() (key MiniBlockKey) {
	key.Height = mbl.Height
	key.Past0 = mbl.Past[0]
	key.Past1 = mbl.Past[1]
	return
}

func (mbl MiniBlock) String() string {
	r := new(strings.Builder)

	fmt.Fprintf(r, "%d ", mbl.Version)
	fmt.Fprintf(r, "height %d", mbl.Height)

	if mbl.Final {
		fmt.Fprintf(r, " Final ")
	}

	if mbl.HighDiff {
		fmt.Fprintf(r, " HighDiff ")
	}

	if mbl.PastCount == 1 {
		fmt.Fprintf(r, " Past [%08x]", mbl.Past[0])
	} else {
		fmt.Fprintf(r, " Past [%08x %08x]", mbl.Past[0], mbl.Past[1])
	}
	fmt.Fprintf(r, " time %d", mbl.Timestamp)
	fmt.Fprintf(r, " flags %d", mbl.Flags)
	fmt.Fprintf(r, " Nonce [%08x %08x %08x]", mbl.Nonce[0], mbl.Nonce[1], mbl.Nonce[2])

	return r.String()
}

// this function gets the block identifier hash, this is only used to deduplicate mini blocks
func (mbl *MiniBlock) GetHash() (result crypto.Hash) {
	ser := mbl.Serialize()
	sha := hasherPool.Get().(hash.Hash)
	sha.Reset()
	sha.Write(ser[:])
	x := sha.Sum(nil)
	copy(result[:], x[:])
	hasherPool.Put(sha)
	return result

	//	return sha3.Sum256(ser[:])
}

// Get PoW hash , this is very slow function
func (mbl *MiniBlock) GetPoWHash() (hash crypto.Hash) {
	if mbl.Height < uint64(globals.Config.MAJOR_HF2_HEIGHT) {
		return astrobwt.POW16(mbl.Serialize())
	}
	return astrobwtv3.AstroBWTv3(mbl.Serialize())
}

func (mbl *MiniBlock) SanityCheck() error {
	if mbl.Version >= 16 {
		return fmt.Errorf("version not supported")
	}
	if mbl.PastCount > 2 {
		return fmt.Errorf("tips cannot be more than 2")
	}
	if mbl.PastCount == 0 {
		return fmt.Errorf("miniblock must have tips")
	}
	if mbl.Height >= 0xffffffffff {
		return fmt.Errorf("miniblock height not possible")
	}
	if mbl.PastCount == 2 && mbl.Past[0] == mbl.Past[1] {
		return fmt.Errorf("tips cannot collide")
	}
	return nil
}

// serialize entire block ( 64 bytes )
func (mbl *MiniBlock) Serialize() (result []byte) {
	if err := mbl.SanityCheck(); err != nil {
		panic(err)
	}

	var b bytes.Buffer

	versionbyte := mbl.Version | mbl.PastCount<<6
	if mbl.HighDiff {
		versionbyte |= 0x10
	}
	if mbl.Final {
		versionbyte |= 0x20
	}
	b.WriteByte(versionbyte)

	binary.Write(&b, binary.BigEndian, mbl.Timestamp)

	var scratch [8]byte
	binary.BigEndian.PutUint64(scratch[:], mbl.Height)
	b.Write(scratch[3:8]) // 1 + 5

	for _, v := range mbl.Past {
		binary.Write(&b, binary.BigEndian, v)
	}

	b.Write(mbl.KeyHash[:16])
	binary.Write(&b, binary.BigEndian, mbl.Flags)
	for _, v := range mbl.Nonce {
		binary.Write(&b, binary.BigEndian, v)
	}

	return b.Bytes()
}

//parse entire block completely
func (mbl *MiniBlock) Deserialize(buf []byte) (err error) {
	if len(buf) < MINIBLOCK_SIZE {
		return fmt.Errorf("Expected %d bytes. Actual %d", MINIBLOCK_SIZE, len(buf))
	}

	if mbl.Version = buf[0] & 0xf; mbl.Version != 1 {
		return fmt.Errorf("unknown version '%d'", mbl.Version)
	}

	mbl.PastCount = buf[0] >> 6
	if buf[0]&0x10 > 0 {
		mbl.HighDiff = true
	}
	if buf[0]&0x20 > 0 {
		mbl.Final = true
	}

	mbl.Timestamp = binary.BigEndian.Uint16(buf[1:])
	mbl.Height = binary.BigEndian.Uint64(buf[0:]) & 0x000000ffffffffff

	var b bytes.Buffer
	b.Write(buf[8:])

	for i := range mbl.Past {
		if err = binary.Read(&b, binary.BigEndian, &mbl.Past[i]); err != nil {
			return
		}
	}

	if err = mbl.SanityCheck(); err != nil {
		return err
	}

	b.Read(mbl.KeyHash[:16])
	if err = binary.Read(&b, binary.BigEndian, &mbl.Flags); err != nil {
		return
	}

	for i := range mbl.Nonce {
		if err = binary.Read(&b, binary.BigEndian, &mbl.Nonce[i]); err != nil {
			return
		}
	}

	return
}
