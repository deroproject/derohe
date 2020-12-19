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
import "math"
import "math/big"
import "bytes"

//import "crypto/rand"
//import "encoding/hex"

import "github.com/deroproject/derohe/crypto/bn256"

//import "golang.org/x/crypto/sha3"

// basically the Σ-protocol
type InnerProduct struct {
	a, b   *big.Int
	ls, rs []*bn256.G1
}

func (ip *InnerProduct) Size() int {
	return FIELDELEMENT_SIZE + FIELDELEMENT_SIZE + 1 + len(ip.ls)*POINT_SIZE + len(ip.rs)*POINT_SIZE
}

// since our bulletproofs are 128 bits, we can get away hard coded 7 entries
func (ip *InnerProduct) Serialize(w *bytes.Buffer) {
	w.Write(ConvertBigIntToByte(ip.a))
	w.Write(ConvertBigIntToByte(ip.b))
	// w.WriteByte(byte(len(ip.ls)))  // we can skip this byte also, why not skip it

	//  fmt.Printf("inner proof length byte %d\n",len(ip.ls))
	for i := range ip.ls {
		w.Write(ip.ls[i].EncodeCompressed())
		w.Write(ip.rs[i].EncodeCompressed())
	}
}

func (ip *InnerProduct) Deserialize(r *bytes.Reader) (err error) {

	var buf [32]byte
	var bufp [33]byte

	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		ip.a = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}

	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		ip.b = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}

	length := 7

	ip.ls = ip.ls[:0]
	ip.rs = ip.rs[:0]
	for i := 0; i < length; i++ {
		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}

			ip.ls = append(ip.ls, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}

			ip.rs = append(ip.rs, &p)
		} else {
			return err
		}

	}

	return err
}

func NewInnerProductProof(ips *IPStatement, witness *IPWitness, salt *big.Int) *InnerProduct {
	var ip InnerProduct

	ip.generateInnerProductProof(ips.PrimeBase, ips.P, witness.L, witness.R, salt)
	return &ip
}

func (ip *InnerProduct) generateInnerProductProof(base *GeneratorParams, P *bn256.G1, as, bs *FieldVector, prev_challenge *big.Int) {
	n := as.Length()

	if n == 1 { // the proof is done, ls,rs are already in place
		ip.a = as.vector[0]
		ip.b = bs.vector[0]
		return
	}

	nPrime := n / 2
	asLeft := as.Slice(0, nPrime)
	asRight := as.Slice(nPrime, n)
	bsLeft := bs.Slice(0, nPrime)
	bsRight := bs.Slice(nPrime, n)
	gLeft := base.Gs.Slice(0, nPrime)
	gRight := base.Gs.Slice(nPrime, n)
	hLeft := base.Hs.Slice(0, nPrime)
	hRight := base.Hs.Slice(nPrime, n)

	cL := asLeft.InnerProduct(bsRight)
	cR := asRight.InnerProduct(bsLeft)

	u := base.H
	L := new(bn256.G1).Add(gRight.Commit(asLeft.vector), hLeft.Commit(bsRight.vector))
	L = new(bn256.G1).Add(L, new(bn256.G1).ScalarMult(u, cL))

	R := new(bn256.G1).Add(gLeft.Commit(asRight.vector), hRight.Commit(bsLeft.vector))
	R = new(bn256.G1).Add(R, new(bn256.G1).ScalarMult(u, cR))

	ip.ls = append(ip.ls, L)
	ip.rs = append(ip.rs, R)

	var input []byte
	input = append(input, ConvertBigIntToByte(prev_challenge)...)
	input = append(input, L.Marshal()...)
	input = append(input, R.Marshal()...)
	x := reducedhash(input)

	xinv := new(big.Int).ModInverse(x, bn256.Order)

	gPrime := gLeft.Times(xinv).Add(gRight.Times(x))
	hPrime := hLeft.Times(x).Add(hRight.Times(xinv))
	aPrime := asLeft.Times(x).Add(asRight.Times(xinv))
	bPrime := bsLeft.Times(xinv).Add(bsRight.Times(x))

	basePrime := NewGeneratorParams3(u, gPrime, hPrime)

	PPrimeL := new(bn256.G1).ScalarMult(L, new(big.Int).Mod(new(big.Int).Mul(x, x), bn256.Order))       //L * (x*x)
	PPrimeR := new(bn256.G1).ScalarMult(R, new(big.Int).Mod(new(big.Int).Mul(xinv, xinv), bn256.Order)) //R * (xinv*xinv)

	PPrime := new(bn256.G1).Add(PPrimeL, PPrimeR)
	PPrime = new(bn256.G1).Add(PPrime, P)

	ip.generateInnerProductProof(basePrime, PPrime, aPrime, bPrime, x)

	return

}

func NewInnerProductProofNew(p *PedersenVectorCommitment, salt *big.Int) *InnerProduct {
	var ip InnerProduct

	ip.generateInnerProductProofNew(p, p.gvalues, p.hvalues, salt)
	return &ip
}

func (ip *InnerProduct) generateInnerProductProofNew(p *PedersenVectorCommitment, as, bs *FieldVector, prev_challenge *big.Int) {
	n := as.Length()

	if n == 1 { // the proof is done, ls,rs are already in place
		ip.a = as.vector[0]
		ip.b = bs.vector[0]
		return
	}

	nPrime := n / 2
	asLeft := as.Slice(0, nPrime)
	asRight := as.Slice(nPrime, n)
	bsLeft := bs.Slice(0, nPrime)
	bsRight := bs.Slice(nPrime, n)

	gsLeft := p.Gs.Slice(0, nPrime)
	gsRight := p.Gs.Slice(nPrime, n)
	hsLeft := p.Hs.Slice(0, nPrime)
	hsRight := p.Hs.Slice(nPrime, n)

	cL := asLeft.InnerProduct(bsRight)
	cR := asRight.InnerProduct(bsLeft)

	/*u := base.H
	L := new(bn256.G1).Add(gRight.Commit(asLeft.vector), hLeft.Commit(bsRight.vector))
	L = new(bn256.G1).Add(L, new(bn256.G1).ScalarMult(u, cL))

	R := new(bn256.G1).Add(gLeft.Commit(asRight.vector), hRight.Commit(bsLeft.vector))
	R = new(bn256.G1).Add(R, new(bn256.G1).ScalarMult(u, cR))
	*/

	Lpart := new(bn256.G1).Add(gsRight.MultiExponentiate(asLeft), hsLeft.MultiExponentiate(bsRight))
	L := new(bn256.G1).Add(Lpart, new(bn256.G1).ScalarMult(p.H, cL))

	Rpart := new(bn256.G1).Add(gsLeft.MultiExponentiate(asRight), hsRight.MultiExponentiate(bsLeft))
	R := new(bn256.G1).Add(Rpart, new(bn256.G1).ScalarMult(p.H, cR))

	ip.ls = append(ip.ls, L)
	ip.rs = append(ip.rs, R)

	var input []byte
	input = append(input, ConvertBigIntToByte(prev_challenge)...)
	input = append(input, L.Marshal()...)
	input = append(input, R.Marshal()...)
	x := reducedhash(input)

	xInv := new(big.Int).ModInverse(x, bn256.Order)

	p.Gs = gsLeft.Times(xInv).Add(gsRight.Times(x))
	p.Hs = hsLeft.Times(x).Add(hsRight.Times(xInv))
	asPrime := asLeft.Times(x).Add(asRight.Times(xInv))
	bsPrime := bsLeft.Times(xInv).Add(bsRight.Times(x))

	ip.generateInnerProductProofNew(p, asPrime, bsPrime, x)

	return

}

func (ip *InnerProduct) Verify(hs []*bn256.G1, u, P *bn256.G1, salt *big.Int, gp *GeneratorParams) bool {
	log_n := uint(len(ip.ls))

	if len(ip.ls) != len(ip.rs) { // length must be same
		return false
	}
	n := uint(math.Pow(2, float64(log_n)))

	o := salt
	var challenges []*big.Int
	for i := uint(0); i < log_n; i++ {

		var input []byte
		input = append(input, ConvertBigIntToByte(o)...)
		input = append(input, ip.ls[i].Marshal()...)
		input = append(input, ip.rs[i].Marshal()...)
		o = reducedhash(input)
		challenges = append(challenges, o)

		o_inv := new(big.Int).ModInverse(o, bn256.Order)

		PPrimeL := new(bn256.G1).ScalarMult(ip.ls[i], new(big.Int).Mod(new(big.Int).Mul(o, o), bn256.Order))         //L * (x*x)
		PPrimeR := new(bn256.G1).ScalarMult(ip.rs[i], new(big.Int).Mod(new(big.Int).Mul(o_inv, o_inv), bn256.Order)) //L * (x*x)

		PPrime := new(bn256.G1).Add(PPrimeL, PPrimeR)
		P = new(bn256.G1).Add(PPrime, P)
	}

	exp := new(big.Int).SetUint64(1)
	for i := uint(0); i < log_n; i++ {
		exp = new(big.Int).Mod(new(big.Int).Mul(exp, challenges[i]), bn256.Order)
	}

	exp_inv := new(big.Int).ModInverse(exp, bn256.Order)

	exponents := make([]*big.Int, n, n)

	exponents[0] = exp_inv // initializefirst element

	bits := make([]bool, n, n)
	for i := uint(0); i < n/2; i++ {
		for j := uint(0); (1<<j)+i < n; j++ {
			i1 := (1 << j) + i
			if !bits[i1] {
				temp := new(big.Int).Mod(new(big.Int).Mul(challenges[log_n-1-j], challenges[log_n-1-j]), bn256.Order)
				exponents[i1] = new(big.Int).Mod(new(big.Int).Mul(exponents[i], temp), bn256.Order)
				bits[i1] = true
			}
		}
	}

	var zeroes [64]byte
	gtemp := new(bn256.G1) // obtain zero element, this should be static and
	htemp := new(bn256.G1) // obtain zero element, this should be static and

	gtemp.Unmarshal(zeroes[:])
	htemp.Unmarshal(zeroes[:])

	for i := uint(0); i < n; i++ {
		gtemp = new(bn256.G1).Add(gtemp, new(bn256.G1).ScalarMult(gp.Gs.vector[i], exponents[i]))
		htemp = new(bn256.G1).Add(htemp, new(bn256.G1).ScalarMult(hs[i], exponents[n-1-i]))
	}
	gtemp = new(bn256.G1).ScalarMult(gtemp, ip.a)
	htemp = new(bn256.G1).ScalarMult(htemp, ip.b)
	utemp := new(bn256.G1).ScalarMult(u, new(big.Int).Mod(new(big.Int).Mul(ip.a, ip.b), bn256.Order))

	P_calculated := new(bn256.G1).Add(gtemp, htemp)
	P_calculated = new(bn256.G1).Add(P_calculated, utemp)

	// fmt.Printf("P %s\n",P.String())
	// fmt.Printf("P_calculated %s\n",P_calculated.String())

	if P_calculated.String() != P.String() { // need something better here
		panic("Faulty or invalid proof")
		return false
	}

	return true
}
