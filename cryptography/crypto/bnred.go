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

import (
	"fmt"
	"math/big"
)

//import "encoding/binary"

//import "crypto/rand"
//import "github.com/stratumfarm/derohe/crypto/bn256"

// this file implements Big Number Reduced form with bn256's Order

type BNRed big.Int

func RandomScalarBNRed() *BNRed {
	return (*BNRed)(RandomScalar())
}

// converts  big.Int to BNRed
func GetBNRed(x *big.Int) *BNRed {
	result := new(BNRed)

	((*big.Int)(result)).Set(x)
	return result
}

// convert BNRed to BigInt
func (x *BNRed) BigInt() *big.Int {
	return new(big.Int).Set(((*big.Int)(x)))
}

func (x *BNRed) SetBytes(buf []byte) *BNRed {
	((*big.Int)(x)).SetBytes(buf)
	return x
}

func (x *BNRed) String() string {
	return ((*big.Int)(x)).Text(16)
}

func (x *BNRed) Text(base int) string {
	return ((*big.Int)(x)).Text(base)
}

func (x *BNRed) MarshalText() ([]byte, error) {
	return []byte(((*big.Int)(x)).Text(16)), nil
}

func (x *BNRed) UnmarshalText(text []byte) error {
	_, err := fmt.Sscan("0x"+string(text), ((*big.Int)(x)))
	return err
}

func FillBytes(x *big.Int, xbytes []byte) {
	// FillBytes not available pre 1.15
	bb := x.Bytes()

	if len(bb) > 32 {
		panic(fmt.Sprintf("number not representable in 32 bytes %d  %x", len(bb), bb))
	}

	for i := range xbytes { // optimized to memclr
		xbytes[i] = 0
	}

	j := 32
	for i := len(bb) - 1; i >= 0; i-- {
		j--
		xbytes[j] = bb[i]
	}
}

/*
// this will return fixed random scalar
func RandomScalarFixed() *big.Int {
	//return new(big.Int).Set(fixed)

	return RandomScalar()
}


type KeyPair struct {
	x *big.Int // secret key
	y *bn256.G1 // public key
}
*/
