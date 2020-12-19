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

import "github.com/deroproject/derohe/crypto/bn256"

type FieldVectorPolynomial struct {
	coefficients []*FieldVector
}

func NewFieldVectorPolynomial(inputs ...*FieldVector) *FieldVectorPolynomial {
	fv := &FieldVectorPolynomial{}
	for _, input := range inputs {
		fv.coefficients = append(fv.coefficients, input.Clone())
	}
	return fv
}

func (fv *FieldVectorPolynomial) Length() int {
	return len(fv.coefficients)
}

func (fv *FieldVectorPolynomial) Evaluate(x *big.Int) *FieldVector {

	result := fv.coefficients[0].Clone()

	accumulator := new(big.Int).Set(x)

	for i := 1; i < len(fv.coefficients); i++ {
		result = result.Add(fv.coefficients[i].Times(accumulator))
		accumulator.Mul(accumulator, x)
		accumulator.Mod(accumulator, bn256.Order)

	}
	return result
}

func (fv *FieldVectorPolynomial) InnerProduct(other *FieldVectorPolynomial) []*big.Int {

	var result []*big.Int

	result_length := fv.Length() + other.Length() - 1
	for i := 0; i < result_length; i++ {
		result = append(result, new(big.Int)) // 0 value fill
	}

	for i := range fv.coefficients {
		for j := range other.coefficients {
			tmp := new(big.Int).Set(result[i+j])
			result[i+j].Add(tmp, fv.coefficients[i].InnerProduct(other.coefficients[j]))
			result[i+j].Mod(result[i+j], bn256.Order)
		}
	}
	return result
}

/*

type PedersenCommitment struct {
	X      *big.Int
	R      *big.Int
	Params *GeneratorParams
}

func NewPedersenCommitment(params *GeneratorParams, x, r *big.Int) *PedersenCommitment {
	pc := &PedersenCommitment{Params: params, X: new(big.Int).Set(x), R: new(big.Int).Set(r)}
	return pc
}
func (pc *PedersenCommitment) Commit() *bn256.G1 {
	var left, right, result bn256.G1
	left.ScalarMult(pc.Params.G, pc.X)
	right.ScalarMult(pc.Params.H, pc.R)
	result.Add(&left, &right)
	return &result
}
func (pc *PedersenCommitment) Add(other *PedersenCommitment) *PedersenCommitment {
	var x, r big.Int
	x.Mod(new(big.Int).Add(pc.X, other.X), bn256.Order)
	r.Mod(new(big.Int).Add(pc.R, other.R), bn256.Order)
	return NewPedersenCommitment(pc.Params, &x, &r)
}
func (pc *PedersenCommitment) Times(constant *big.Int) *PedersenCommitment {
	var x, r big.Int
	x.Mod(new(big.Int).Mul(pc.X, constant), bn256.Order)
	r.Mod(new(big.Int).Mul(pc.R, constant), bn256.Order)
	return NewPedersenCommitment(pc.Params, &x, &r)
}

type PolyCommitment struct {
	coefficient_commitments []*PedersenCommitment
	Params                  *GeneratorParams
}

func NewPolyCommitment(params *GeneratorParams, coefficients []*big.Int) *PolyCommitment {
	pc := &PolyCommitment{Params: params}
	pc.coefficient_commitments = append(pc.coefficient_commitments, NewPedersenCommitment(params, coefficients[0], new(big.Int).SetUint64(0)))

	for i := 1; i < len(coefficients); i++ {
		pc.coefficient_commitments = append(pc.coefficient_commitments, NewPedersenCommitment(params, coefficients[i], RandomScalarFixed()))

	}
	return pc
}

func (pc *PolyCommitment) GetCommitments() []*bn256.G1 {
	var result []*bn256.G1
	for i := 1; i < len(pc.coefficient_commitments); i++ {
		result = append(result, pc.coefficient_commitments[i].Commit())
	}
	return result
}

func (pc *PolyCommitment) Evaluate(constant *big.Int) *PedersenCommitment {
	result := pc.coefficient_commitments[0]

	accumulator := new(big.Int).Set(constant)

	for i := 1; i < len(pc.coefficient_commitments); i++ {

		tmp := new(big.Int).Set(accumulator)
		result = result.Add(pc.coefficient_commitments[i].Times(accumulator))
		accumulator.Mod(new(big.Int).Mul(tmp, constant), bn256.Order)
	}

	return result
}
*/

/*
// bother FieldVector and GeneratorVector satisfy this
type Vector interface{
	Length() int
	Extract(parity bool) Vector
	Add(other Vector)Vector
	Hadamard( []*big.Int) Vector
	Times (*big.Int) Vector
	Negate() Vector
}
*/

// check this https://pdfs.semanticscholar.org/d38d/e48ee4127205a0f25d61980c8f241718b66e.pdf
// https://arxiv.org/pdf/1802.03932.pdf

var unity *big.Int

func init() {
	// primitive 2^28th root of unity modulo q
	unity, _ = new(big.Int).SetString("14a3074b02521e3b1ed9852e5028452693e87be4e910500c7ba9bbddb2f46edd", 16)

}

func fft_FieldVector(input *FieldVector, inverse bool) *FieldVector {
	length := input.Length()
	if length == 1 {
		return input
	}

	// lngth must be multiple of 2 ToDO
	if length%2 != 0 {
		panic("length must be multiple of 2")
	}

	//unity,_ := new(big.Int).SetString("14a3074b02521e3b1ed9852e5028452693e87be4e910500c7ba9bbddb2f46edd",16)

	omega := new(big.Int).Exp(unity, new(big.Int).SetUint64((1<<28)/uint64(length)), bn256.Order)
	if inverse {
		omega = new(big.Int).ModInverse(omega, bn256.Order)
	}

	even := fft_FieldVector(input.Extract(false), inverse)
	odd := fft_FieldVector(input.Extract(true), inverse)

	omegas := []*big.Int{new(big.Int).SetUint64(1)}

	for i := 1; i < length/2; i++ {
		omegas = append(omegas, new(big.Int).Mod(new(big.Int).Mul(omegas[i-1], omega), bn256.Order))
	}

	omegasv := NewFieldVector(omegas)
	result := even.Add(odd.Hadamard(omegasv)).Concat(even.Add(odd.Hadamard(omegasv).Negate()))
	if inverse {
		result = result.Times(new(big.Int).ModInverse(new(big.Int).SetUint64(2), bn256.Order))
	}

	return result

}

// this is exactly same as fft_FieldVector, alternate implementation
func fftints(input []*big.Int) (result []*big.Int) {
	size := len(input)
	if size == 1 {
		return input
	}
	//require(size % 2 == 0, "Input size is not a power of 2!");

	unity, _ := new(big.Int).SetString("14a3074b02521e3b1ed9852e5028452693e87be4e910500c7ba9bbddb2f46edd", 16)

	omega := new(big.Int).Exp(unity, new(big.Int).SetUint64((1<<28)/uint64(size)), bn256.Order)

	even := fftints(extractbits(input, 0))
	odd := fftints(extractbits(input, 1))
	omega_run := new(big.Int).SetUint64(1)
	result = make([]*big.Int, len(input), len(input))
	for i := 0; i < len(input)/2; i++ {
		temp := new(big.Int).Mod(new(big.Int).Mul(odd[i], omega_run), bn256.Order)
		result[i] = new(big.Int).Mod(new(big.Int).Add(even[i], temp), bn256.Order)
		result[i+size/2] = new(big.Int).Mod(new(big.Int).Sub(even[i], temp), bn256.Order)
		omega_run = new(big.Int).Mod(new(big.Int).Mul(omega, omega_run), bn256.Order)
	}
	return result
}

func extractbits(input []*big.Int, parity int) (result []*big.Int) {
	result = make([]*big.Int, len(input)/2, len(input)/2)
	for i := 0; i < len(input)/2; i++ {
		result[i] = new(big.Int).Set(input[2*i+parity])
	}
	return
}

func fft_GeneratorVector(input *PointVector, inverse bool) *PointVector {
	length := input.Length()
	if length == 1 {
		return input
	}

	// lngth must be multiple of 2 ToDO
	if length%2 != 0 {
		panic("length must be multiple of 2")
	}

	// unity,_ := new(big.Int).SetString("14a3074b02521e3b1ed9852e5028452693e87be4e910500c7ba9bbddb2f46edd",16)

	omega := new(big.Int).Exp(unity, new(big.Int).SetUint64((1<<28)/uint64(length)), bn256.Order)
	if inverse {
		omega = new(big.Int).ModInverse(omega, bn256.Order)
	}

	even := fft_GeneratorVector(input.Extract(false), inverse)

	//fmt.Printf("exponent_fft %d %s \n",i, exponent_fft.vector[i].Text(16))

	odd := fft_GeneratorVector(input.Extract(true), inverse)

	omegas := []*big.Int{new(big.Int).SetUint64(1)}

	for i := 1; i < length/2; i++ {
		omegas = append(omegas, new(big.Int).Mod(new(big.Int).Mul(omegas[i-1], omega), bn256.Order))
	}

	omegasv := omegas
	result := even.Add(odd.Hadamard(omegasv)).Concat(even.Add(odd.Hadamard(omegasv).Negate()))
	if inverse {
		result = result.Times(new(big.Int).ModInverse(new(big.Int).SetUint64(2), bn256.Order))
	}

	return result

}

func Convolution(exponent *FieldVector, base *PointVector) *PointVector {
	size := base.Length()

	exponent_fft := fft_FieldVector(exponent.Flip(), false)

	/*exponent_fft2 := fftints( exponent.Flip().vector) // aternate implementation proof checking
	for i := range exponent_fft.vector{
				fmt.Printf("exponent_fft %d %s \n",i, exponent_fft.vector[i].Text(16))
				fmt.Printf("exponent_ff2 %d %s \n",i, exponent_fft2[i].Text(16))
			}
	*/

	temp := fft_GeneratorVector(base, false).Hadamard(exponent_fft.vector)
	return fft_GeneratorVector(temp.Slice(0, size/2).Add(temp.Slice(size/2, size)).Times(new(big.Int).ModInverse(new(big.Int).SetUint64(2), bn256.Order)), true)
	// using the optimization described here https://dsp.stackexchange.com/a/30699
}
