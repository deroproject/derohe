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

import "fmt"
import "math/big"

//import "crypto/rand"
//import "encoding/hex"

import "github.com/deroproject/derohe/cryptography/bn256"

//import "golang.org/x/crypto/sha3"

var G *bn256.G1
var global_pedersen_values PedersenVectorCommitment

func init() {
	var zeroes [64]byte
	var gs, hs []*bn256.G1

	global_pedersen_values.G = HashToPoint(HashtoNumber([]byte(PROTOCOL_CONSTANT + "G"))) // this is same as mybase or vice-versa
	global_pedersen_values.H = HashToPoint(HashtoNumber([]byte(PROTOCOL_CONSTANT + "H")))

	global_pedersen_values.GSUM = new(bn256.G1)
	global_pedersen_values.GSUM.Unmarshal(zeroes[:])

	for i := 0; i < 128; i++ {
		gs = append(gs, HashToPoint(HashtoNumber(append([]byte(PROTOCOL_CONSTANT+"G"), hextobytes(makestring64(fmt.Sprintf("%x", i)))...))))
		hs = append(hs, HashToPoint(HashtoNumber(append([]byte(PROTOCOL_CONSTANT+"H"), hextobytes(makestring64(fmt.Sprintf("%x", i)))...))))

		global_pedersen_values.GSUM = new(bn256.G1).Add(global_pedersen_values.GSUM, gs[i])
	}
	global_pedersen_values.Gs = NewPointVector(gs)
	global_pedersen_values.Hs = NewPointVector(hs)

	// also initialize elgamal_zero
	ElGamal_ZERO = new(bn256.G1).ScalarMult(global_pedersen_values.G, new(big.Int).SetUint64(0))
	ElGamal_ZERO_string = ElGamal_ZERO.String()
	ElGamal_BASE_G = global_pedersen_values.G
	G = global_pedersen_values.G
	((*bn256.G1)(&GPoint)).Set(G) // setup base point

	//   fmt.Printf("basepoint %s on %x\n", G.String(), G.Marshal())
}

type PedersenCommitmentNew struct {
	G          *bn256.G1
	H          *bn256.G1
	Randomness *big.Int
	Result     *bn256.G1
}

func NewPedersenCommitmentNew() (p *PedersenCommitmentNew) {
	return &PedersenCommitmentNew{G: global_pedersen_values.G, H: global_pedersen_values.H}
}

// commit a specific value to specific bases
func (p *PedersenCommitmentNew) Commit(value *big.Int) *PedersenCommitmentNew {
	p.Randomness = RandomScalarFixed()
	point := new(bn256.G1).Add(new(bn256.G1).ScalarMult(p.G, value), new(bn256.G1).ScalarMult(p.H, p.Randomness))
	p.Result = new(bn256.G1).Set(point)
	return p
}

type PedersenVectorCommitment struct {
	G    *bn256.G1
	H    *bn256.G1
	GSUM *bn256.G1

	Gs         *PointVector
	Hs         *PointVector
	Randomness *big.Int
	Result     *bn256.G1

	gvalues *FieldVector
	hvalues *FieldVector
}

func NewPedersenVectorCommitment() (p *PedersenVectorCommitment) {
	p = &PedersenVectorCommitment{}
	*p = global_pedersen_values
	return
}

// commit a specific value to specific bases
func (p *PedersenVectorCommitment) Commit(gvalues, hvalues *FieldVector) *PedersenVectorCommitment {

	p.Randomness = RandomScalarFixed()
	point := new(bn256.G1).ScalarMult(p.H, p.Randomness)
	point = new(bn256.G1).Add(point, p.Gs.MultiExponentiate(gvalues))
	point = new(bn256.G1).Add(point, p.Hs.MultiExponentiate(hvalues))

	p.Result = new(bn256.G1).Set(point)
	return p
}
