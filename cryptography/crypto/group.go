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

//import "fmt"
import (
	"encoding/hex"
	"math/big"

	"github.com/stratumfarm/derohe/cryptography/bn256"
) //import "crypto/rand"

// this file implements Big Number Reduced form with bn256's Order

type Point bn256.G1

var GPoint Point

// ScalarMult with chainable API
func (p *Point) ScalarMult(r *BNRed) (result *Point) {
	result = new(Point)
	((*bn256.G1)(result)).ScalarMult(((*bn256.G1)(p)), ((*big.Int)(r)))
	return result
}

func (p *Point) EncodeCompressed() []byte {
	return ((*bn256.G1)(p)).EncodeCompressed()
}

func (p *Point) DecodeCompressed(i []byte) error {
	return ((*bn256.G1)(p)).DecodeCompressed(i)
}

func (p *Point) G1() *bn256.G1 {
	return ((*bn256.G1)(p))
}
func (p *Point) Set(x *Point) *Point {
	return ((*Point)(((*bn256.G1)(p)).Set(((*bn256.G1)(x)))))
}

func (p *Point) String() string {
	return string(((*bn256.G1)(p)).EncodeCompressed())
}

func (p *Point) StringHex() string {
	return string(hex.EncodeToString(((*bn256.G1)(p)).EncodeCompressed()))
}

func (p *Point) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(((*bn256.G1)(p)).EncodeCompressed())), nil
}

func (p *Point) UnmarshalText(text []byte) error {

	tmp, err := hex.DecodeString(string(text))
	if err != nil {
		return err
	}
	return p.DecodeCompressed(tmp)
}
