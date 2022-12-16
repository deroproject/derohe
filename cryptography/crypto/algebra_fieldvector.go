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
import "math/big"

//import "crypto/rand"
//import "encoding/hex"

import "github.com/deroproject/derohe/cryptography/bn256"

//import "golang.org/x/crypto/sha3"

type FieldVector struct {
	vector []*big.Int
}

func NewFieldVector(input []*big.Int) *FieldVector {
	return &FieldVector{vector: input}
}

func NewFieldVectorRandomFilled(capacity int) *FieldVector {
	fv := &FieldVector{vector: make([]*big.Int, capacity, capacity)}
	for i := range fv.vector {
		fv.vector[i] = RandomScalarFixed()
	}
	return fv
}

func (fv *FieldVector) Length() int {
	return len(fv.vector)
}

// slice and return
func (fv *FieldVector) Slice(start, end int) *FieldVector {
	var result FieldVector
	for i := start; i < end; i++ {
		result.vector = append(result.vector, new(big.Int).Set(fv.vector[i]))
	}
	return &result
}

// copy and return
func (fv *FieldVector) Clone() *FieldVector {
	return fv.Slice(0, len(fv.vector))
}

func (fv *FieldVector) Element(index int) *big.Int {
	return fv.vector[index]
}

func (fv *FieldVector) SliceRaw(start, end int) []*big.Int {
	var result FieldVector
	for i := start; i < end; i++ {
		result.vector = append(result.vector, new(big.Int).Set(fv.vector[i]))
	}
	return result.vector
}

func (fv *FieldVector) Flip() *FieldVector {
	var result FieldVector
	for i := range fv.vector {
		result.vector = append(result.vector, new(big.Int).Set(fv.vector[(len(fv.vector)-i)%len(fv.vector)]))
	}
	return &result
}

func (fv *FieldVector) Sum() *big.Int {
	var accumulator big.Int

	for i := range fv.vector {
		var accopy big.Int

		accopy.Add(&accumulator, fv.vector[i])
		accumulator.Mod(&accopy, bn256.Order)
	}

	return &accumulator
}

func (fv *FieldVector) Add(addendum *FieldVector) *FieldVector {
	var result FieldVector

	if len(fv.vector) != len(addendum.vector) {
		panic("mismatched number of elements")
	}

	for i := range fv.vector {
		var ri big.Int
		ri.Mod(new(big.Int).Add(fv.vector[i], addendum.vector[i]), bn256.Order)
		result.vector = append(result.vector, &ri)
	}

	return &result
}

func (gv *FieldVector) AddConstant(c *big.Int) *FieldVector {
	var result FieldVector

	for i := range gv.vector {
		var ri big.Int
		ri.Mod(new(big.Int).Add(gv.vector[i], c), bn256.Order)
		result.vector = append(result.vector, &ri)
	}

	return &result
}

func (fv *FieldVector) Hadamard(exponent *FieldVector) *FieldVector {
	var result FieldVector

	if len(fv.vector) != len(exponent.vector) {
		panic("mismatched number of elements")
	}
	for i := range fv.vector {
		result.vector = append(result.vector, new(big.Int).Mod(new(big.Int).Mul(fv.vector[i], exponent.vector[i]), bn256.Order))
	}

	return &result
}

func (fv *FieldVector) InnerProduct(exponent *FieldVector) *big.Int {
	if len(fv.vector) != len(exponent.vector) {
		panic("mismatched number of elements")
	}

	accumulator := new(big.Int)
	for i := range fv.vector {
		tmp := new(big.Int).Mod(new(big.Int).Mul(fv.vector[i], exponent.vector[i]), bn256.Order)
		accumulator.Add(accumulator, tmp)
		accumulator.Mod(accumulator, bn256.Order)
	}

	return accumulator
}

func (fv *FieldVector) Negate() *FieldVector {
	var result FieldVector
	for i := range fv.vector {
		result.vector = append(result.vector, new(big.Int).Mod(new(big.Int).Neg(fv.vector[i]), bn256.Order))
	}
	return &result
}

func (fv *FieldVector) Times(multiplier *big.Int) *FieldVector {
	var result FieldVector
	for i := range fv.vector {
		res := new(big.Int).Mul(fv.vector[i], multiplier)
		res.Mod(res, bn256.Order)
		result.vector = append(result.vector, res)
	}
	return &result
}

func (fv *FieldVector) Invert() *FieldVector {
	var result FieldVector
	for i := range fv.vector {
		result.vector = append(result.vector, new(big.Int).ModInverse(fv.vector[i], bn256.Order))
	}
	return &result
}

func (fv *FieldVector) Concat(addendum *FieldVector) *FieldVector {
	var result FieldVector
	for i := range fv.vector {
		result.vector = append(result.vector, new(big.Int).Set(fv.vector[i]))
	}

	for i := range addendum.vector {
		result.vector = append(result.vector, new(big.Int).Set(addendum.vector[i]))
	}

	return &result
}

func (fv *FieldVector) Extract(parity bool) *FieldVector {
	var result FieldVector

	remainder := 0
	if parity {
		remainder = 1
	}
	for i := range fv.vector {
		if i%2 == remainder {

			result.vector = append(result.vector, new(big.Int).Set(fv.vector[i]))
		}
	}
	return &result
}
