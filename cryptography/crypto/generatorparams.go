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

func NewGeneratorParams(count int) *GeneratorParams {
	GP := &GeneratorParams{}
	var zeroes [64]byte

	GP.G = HashToPoint(HashtoNumber([]byte(PROTOCOL_CONSTANT + "G"))) // this is same as mybase or vice-versa
	GP.H = HashToPoint(HashtoNumber([]byte(PROTOCOL_CONSTANT + "H")))

	var gs, hs []*bn256.G1

	GP.GSUM = new(bn256.G1)
	GP.GSUM.Unmarshal(zeroes[:])

	for i := 0; i < count; i++ {
		gs = append(gs, HashToPoint(HashtoNumber(append([]byte(PROTOCOL_CONSTANT+"G"), hextobytes(makestring64(fmt.Sprintf("%x", i)))...))))
		hs = append(hs, HashToPoint(HashtoNumber(append([]byte(PROTOCOL_CONSTANT+"H"), hextobytes(makestring64(fmt.Sprintf("%x", i)))...))))

		GP.GSUM = new(bn256.G1).Add(GP.GSUM, gs[i])
	}
	GP.Gs = NewPointVector(gs)
	GP.Hs = NewPointVector(hs)

	return GP
}

func NewGeneratorParams3(h *bn256.G1, gs, hs *PointVector) *GeneratorParams {
	GP := &GeneratorParams{}

	GP.G = HashToPoint(HashtoNumber([]byte(PROTOCOL_CONSTANT + "G"))) // this is same as mybase or vice-versa
	GP.H = h
	GP.Gs = gs
	GP.Hs = hs
	return GP
}

func (gp *GeneratorParams) Commit(blind *big.Int, gexps, hexps *FieldVector) *bn256.G1 {
	result := new(bn256.G1).ScalarMult(gp.H, blind)
	for i := range gexps.vector {
		result = new(bn256.G1).Add(result, new(bn256.G1).ScalarMult(gp.Gs.vector[i], gexps.vector[i]))
	}
	if hexps != nil {
		for i := range hexps.vector {
			result = new(bn256.G1).Add(result, new(bn256.G1).ScalarMult(gp.Hs.vector[i], hexps.vector[i]))
		}
	}
	return result
}
