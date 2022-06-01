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

package crypto

// this file implements serialization/deserialization of balances from the chain

//import "fmt"
import "encoding/binary"

//import "github.com/romana/rlog"

// import "github.com/stratumfarm/derohe/cryptography/bn256"

// this structure is used to store balance in the balance tree
type NonceBalance struct {
	NonceHeight uint64   // stores nonce height
	Balance     *ElGamal // store homomorphic balance
}

func (x *NonceBalance) checknil() {
	if x == nil {
		panic("Cannot be NIL")
	}
}
func (x *NonceBalance) Marshal() (output []byte) {
	x.checknil()
	var tmp [binary.MaxVarintLen64]byte
	size := binary.PutUvarint(tmp[:], x.NonceHeight)
	output = append(output, tmp[:size]...)
	output = append(output, x.Balance.Serialize()...)

	//	fmt.Printf("serialized to %x nonceheight %d\n", output,x.NonceHeight)
	//	fmt.Printf("serialized bo %x nonceheight %d\n", x.Balance.Serialize(), x.NonceHeight)
	return
}

func (x *NonceBalance) Serialize() []byte {
	return x.Marshal()
}

// any error should panic, because we can no longer do anything
func (x *NonceBalance) Unmarshal(buf []byte) *NonceBalance {
	x.checknil()
	var size int
	x.NonceHeight, size = binary.Uvarint(buf[:])
	if size <= 0 {
		panic("buf too small or value larger than 64 bits")
	}
	//	fmt.Printf("buf %x nonceheight %d, size %d bufsize %d\n",buf,x.NonceHeight, size, len(buf)-size)
	x.Balance = new(ElGamal).Deserialize(buf[size:])
	return x
}

func (x *NonceBalance) Deserialize(buf []byte) *NonceBalance {
	return x.Unmarshal(buf)
}

func (x *NonceBalance) Nonce() uint64 {
	x.checknil()
	return x.NonceHeight
}

// used to avoid some processing costs during verification
func (x *NonceBalance) UnmarshalNonce(buf []byte) {
	x.checknil()
	var size int
	x.NonceHeight, size = binary.Uvarint(buf[:])
	if size <= 0 {
		panic("buf too small or value larger than 64 bits")
	}
}
