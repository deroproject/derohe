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
import "math"
import "math/big"
import "bytes"
import "strconv"

//import "crypto/rand"
//import "encoding/hex"

import "github.com/deroproject/derohe/cryptography/bn256"

//import "golang.org/x/crypto/sha3"

//import "github.com/kubernetes/klog"

type Proof struct {
	BA *bn256.G1
	BS *bn256.G1
	A  *bn256.G1
	B  *bn256.G1

	CLnG, CRnG, C_0G, DG, y_0G, gG, C_XG, y_XG []*bn256.G1

	u *bn256.G1

	f *FieldVector

	z_A *big.Int

	T_1 *bn256.G1
	T_2 *bn256.G1

	that *big.Int
	mu   *big.Int

	c                     *big.Int
	s_sk, s_r, s_b, s_tau *big.Int

	ip *InnerProduct
}

type IPStatement struct {
	PrimeBase *GeneratorParams
	P         *bn256.G1
}

type IPWitness struct {
	L *FieldVector
	R *FieldVector
}

// this is based on roothash, scid etc and user's secret key
// thus is the basis of protection from a number of double spending attacks
func (p *Proof) Nonce() Hash {
	return Keccak256(p.u.EncodeCompressed())
}

func (p *Proof) Parity() bool {
	zero := big.NewInt(0)
	if zero.Cmp(p.f.Element(0)) == 0 {
		return true
	}
	return false
}

func (p *Proof) Serialize(w *bytes.Buffer) {
	if p == nil {
		return
	}
	w.Write(p.BA.EncodeCompressed())
	w.Write(p.BS.EncodeCompressed())
	w.Write(p.A.EncodeCompressed())
	w.Write(p.B.EncodeCompressed())

	//w.WriteByte(byte(len(p.CLnG)))  // we can skip this byte also, why not skip it

	// fmt.Printf("CLnG byte %d\n",len(p.CLnG))
	for i := 0; i < len(p.CLnG); i++ {
		w.Write(p.CLnG[i].EncodeCompressed())
		w.Write(p.CRnG[i].EncodeCompressed())
		w.Write(p.C_0G[i].EncodeCompressed())
		w.Write(p.DG[i].EncodeCompressed())
		w.Write(p.y_0G[i].EncodeCompressed())
		w.Write(p.gG[i].EncodeCompressed())
		w.Write(p.C_XG[i].EncodeCompressed())
		w.Write(p.y_XG[i].EncodeCompressed())
	}

	w.Write(p.u.EncodeCompressed())

	if len(p.CLnG) != len(p.f.vector) {
		/// panic(fmt.Sprintf("different size %d %d", len(p.CLnG), len(p.f.vector)))
	}

	/*if len(p.f.vector) != 2 {
	          panic(fmt.Sprintf("f could not be serialized length %d", len(p.CLnG), len(p.f.vector)))
	  }
	*/

	//    fmt.Printf("writing %d fvector points\n", len(p.f.vector));
	for i := 0; i < len(p.f.vector); i++ {
		w.Write(ConvertBigIntToByte(p.f.vector[i]))
	}

	w.Write(ConvertBigIntToByte(p.z_A))

	w.Write(p.T_1.EncodeCompressed())
	w.Write(p.T_2.EncodeCompressed())

	w.Write(ConvertBigIntToByte(p.that))
	w.Write(ConvertBigIntToByte(p.mu))

	w.Write(ConvertBigIntToByte(p.c))
	w.Write(ConvertBigIntToByte(p.s_sk))
	w.Write(ConvertBigIntToByte(p.s_r))
	w.Write(ConvertBigIntToByte(p.s_b))
	w.Write(ConvertBigIntToByte(p.s_tau))

	p.ip.Serialize(w)

}

func (proof *Proof) Deserialize(r *bytes.Reader, length int) error {

	var buf [32]byte
	var bufp [33]byte

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		proof.BA = &p
	} else {
		return err
	}

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		proof.BS = &p
	} else {
		return err
	}

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		proof.A = &p
	} else {
		return err
	}

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		proof.B = &p
	} else {
		return err
	}

	proof.CLnG = proof.CLnG[:0]
	proof.CRnG = proof.CRnG[:0]
	proof.C_0G = proof.C_0G[:0]
	proof.DG = proof.DG[:0]
	proof.y_0G = proof.y_0G[:0]
	proof.gG = proof.gG[:0]
	proof.C_XG = proof.C_XG[:0]
	proof.y_XG = proof.y_XG[:0]

	for i := 0; i < length; i++ {

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			proof.CLnG = append(proof.CLnG, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			//fmt.Printf("CRnG point bytes2 %x\n", bufp[:])
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				//fmt.Printf("CRng point bytes1 %x\n", bufp[:])
				return err
			}
			proof.CRnG = append(proof.CRnG, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			proof.C_0G = append(proof.C_0G, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			proof.DG = append(proof.DG, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			proof.y_0G = append(proof.y_0G, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			proof.gG = append(proof.gG, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			proof.C_XG = append(proof.C_XG, &p)
		} else {
			return err
		}

		if n, err := r.Read(bufp[:]); n == 33 && err == nil {
			var p bn256.G1
			if err = p.DecodeCompressed(bufp[:]); err != nil {
				return err
			}
			proof.y_XG = append(proof.y_XG, &p)
		} else {
			return err
		}

	}

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		proof.u = &p
	} else {
		return err
	}

	proof.f = &FieldVector{}

	//fmt.Printf("flen  %d\n", flen )
	for j := 0; j < length*2; j++ {
		if n, err := r.Read(buf[:]); n == 32 && err == nil {
			proof.f.vector = append(proof.f.vector, new(big.Int).SetBytes(buf[:]))
		} else {
			return err
		}

	}

	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.z_A = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		proof.T_1 = &p
	} else {
		return err
	}

	if n, err := r.Read(bufp[:]); n == 33 && err == nil {
		var p bn256.G1
		if err = p.DecodeCompressed(bufp[:]); err != nil {
			return err
		}
		proof.T_2 = &p
	} else {
		return err
	}

	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.that = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}

	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.mu = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}

	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.c = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}
	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.s_sk = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}
	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.s_r = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}
	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.s_b = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}
	if n, err := r.Read(buf[:]); n == 32 && err == nil {
		proof.s_tau = new(big.Int).SetBytes(buf[:])
	} else {
		return err
	}
	proof.ip = &InnerProduct{}

	return proof.ip.Deserialize(r)

}

/*
// statement hash
func (s *Statement) Hash() *big.Int {
	var input []byte
	for i := range s.CLn {
		input = append(input, s.CLn[i].Marshal()...)
	}
	for i := range s.CRn {
		input = append(input, s.CRn[i].Marshal()...)
	}
	for i := range s.C {
		input = append(input, s.C[i].Marshal()...)
	}
	input = append(input, s.D.Marshal()...)
	for i := range s.Publickeylist {
		input = append(input, s.Publickeylist[i].Marshal()...)
	}
	input = append(input, s.Roothash[:]...)

	return reducedhash(input)
}
*/

func (p *Proof) Size() int {
	size := 4*POINT_SIZE + (len(p.CLnG)+len(p.CRnG)+len(p.C_0G)+len(p.DG)+len(p.y_0G)+len(p.gG)+len(p.C_XG)+len(p.y_XG))*POINT_SIZE
	size += POINT_SIZE
	size += len(p.f.vector) * FIELDELEMENT_SIZE
	size += FIELDELEMENT_SIZE
	size += 2 * POINT_SIZE // T_1 ,T_2
	size += 7 * FIELDELEMENT_SIZE
	size += p.ip.Size()
	return size
}

func (proof *Proof) hashmash1(v *big.Int) *big.Int {
	var input []byte
	input = append(input, ConvertBigIntToByte(v)...)
	for i := range proof.CLnG {
		input = append(input, proof.CLnG[i].Marshal()...)
	}
	for i := range proof.CRnG {
		input = append(input, proof.CRnG[i].Marshal()...)
	}
	for i := range proof.C_0G {
		input = append(input, proof.C_0G[i].Marshal()...)
	}
	for i := range proof.DG {
		input = append(input, proof.DG[i].Marshal()...)
	}
	for i := range proof.y_0G {
		input = append(input, proof.y_0G[i].Marshal()...)
	}
	for i := range proof.gG {
		input = append(input, proof.gG[i].Marshal()...)
	}
	for i := range proof.C_XG {
		input = append(input, proof.C_XG[i].Marshal()...)
	}
	for i := range proof.y_XG {
		input = append(input, proof.y_XG[i].Marshal()...)
	}
	return reducedhash(input)
}

// function, which takes a string as
// argument and return the reverse of string.
func reverse(s string) string {
	rns := []rune(s) // convert to rune
	for i, j := 0, len(rns)-1; i < j; i, j = i+1, j-1 {

		// swap the letters of the string,
		// like first with last and so on.
		rns[i], rns[j] = rns[j], rns[i]
	}

	// return the reversed string.
	return string(rns)
}

var params = NewGeneratorParams(128) // these can be pregenerated similarly as in DERO project

func GenerateProof(scid Hash, scid_index int, s *Statement, witness *Witness, u *bn256.G1, txid Hash, burn_value uint64) *Proof {

	var proof Proof
	proof.u = u

	statementhash := reducedhash(txid[:])

	// statement should be constructed from these, however these are being simplified
	var C []*ElGamal
	var Cn ElGamalVector
	for i := range s.C {
		C = append(C, ConstructElGamal(s.C[i], s.D))
		Cn.vector = append(Cn.vector, ConstructElGamal(s.CLn[i], s.CRn[i]))
	}

	btransfer := new(big.Int).SetInt64(int64(witness.TransferAmount)) // this should be reduced
	bdiff := new(big.Int).SetInt64(int64(witness.Balance))            // this should be reduced

	number := btransfer.Add(btransfer, bdiff.Lsh(bdiff, 64)) // we are placing balance and left over balance, and doing a range proof of 128 bits

	number_string := reverse("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" + number.Text(2))
	number_string_left_128bits := string(number_string[0:128])

	var aL, aR FieldVector // convert the amount to make sure it cannot be negative

	//klog.V(2).Infof("reverse %s\n", number_string_left_128bits)
	for _, b := range []byte(number_string_left_128bits) {
		if b == '1' {
			aL.vector = append(aL.vector, new(big.Int).SetInt64(1))
			aR.vector = append(aR.vector, new(big.Int).SetInt64(0))
		} else {
			aL.vector = append(aL.vector, new(big.Int).SetInt64(0))
			aR.vector = append(aR.vector, new(big.Int).Mod(new(big.Int).SetInt64(-1), bn256.Order))
		}
	}

	//klog.V(2).Infof("aRa %+v\n", aRa)

	proof_BA_internal := NewPedersenVectorCommitment().Commit(&aL, &aR)
	proof.BA = proof_BA_internal.Result

	sL := NewFieldVectorRandomFilled(len(aL.vector))
	sR := NewFieldVectorRandomFilled(len(aL.vector))
	proof_BS_internal := NewPedersenVectorCommitment().Commit(sL, sR)
	proof.BS = proof_BS_internal.Result

	//klog.V(2).Infof("Proof BA %s\n", proof.BA.String())
	//klog.V(2).Infof("Proof BS %s\n", proof.BS.String())

	if len(s.Publickeylist) >= 1 && len(s.Publickeylist)&(len(s.Publickeylist)-1) != 0 {
		fmt.Printf("len of Publickeylist %d\n", len(s.Publickeylist))
		panic("we need power of 2")
	}

	N := len(s.Publickeylist)
	m := int(math.Log2(float64(N)))

	if math.Pow(2, float64(m)) != float64(N) {
		panic("log failed")
	}

	var aa, ba, bspecial []*big.Int
	for i := 0; i < 2*m; i++ {
		if i == 0 || i == m {
			aa = append(aa, new(big.Int).SetUint64(0))
		} else {
			aa = append(aa, RandomScalarFixed())
		}
	}

	witness_index := reverse(fmt.Sprintf("%0"+fmt.Sprintf("%db", m)+"%0"+fmt.Sprintf("%db", m), witness.Index[1], witness.Index[0]))

	for _, b := range []byte(witness_index) {
		if b == '1' {
			ba = append(ba, new(big.Int).SetUint64(1))
			bspecial = append(bspecial, new(big.Int).Mod(new(big.Int).SetInt64(-1), bn256.Order))
		} else {
			ba = append(ba, new(big.Int).SetUint64(0))
			bspecial = append(bspecial, new(big.Int).Mod(new(big.Int).SetInt64(1), bn256.Order))
		}
	}

	a := NewFieldVector(aa)
	b := NewFieldVector(ba)

	//	klog.V(1).Infof("witness_index of sender/receiver %s\n", witness_index)

	c := a.Hadamard(NewFieldVector(bspecial))
	d := a.Hadamard(a).Negate()

	//	klog.V(2).Infof("d %s\n", d.vector[0].Text(16))

	e := NewFieldVector([]*big.Int{new(big.Int).Mod(new(big.Int).Mul(a.vector[0], a.vector[m]), bn256.Order),
		new(big.Int).Mod(new(big.Int).Mul(a.vector[0], a.vector[m]), bn256.Order)})

	second := new(big.Int).Set(a.vector[b.vector[m].Uint64()*uint64(m)])
	second.Neg(second)

	f := NewFieldVector([]*big.Int{a.vector[b.vector[0].Uint64()*uint64(m)], new(big.Int).Mod(second, bn256.Order)})

	//	for i := range f.vector {
	//		klog.V(2).Infof("f %d %s\n", i, f.vector[i].Text(16))
	//	}

	proof_A_internal := NewPedersenVectorCommitment().Commit(a, d.Concat(e))
	proof.A = proof_A_internal.Result
	proof_B_internal := NewPedersenVectorCommitment().Commit(b, c.Concat(f))
	proof.B = proof_B_internal.Result

	//	klog.V(2).Infof("Proof A %s\n", proof.A.String())
	//	klog.V(2).Infof("Proof B %s\n", proof.B.String())

	var v *big.Int

	{ // hash mash
		var input []byte
		input = append(input, ConvertBigIntToByte(statementhash)...)
		input = append(input, proof.BA.Marshal()...)
		input = append(input, proof.BS.Marshal()...)
		input = append(input, proof.A.Marshal()...)
		input = append(input, proof.B.Marshal()...)
		v = reducedhash(input)
	}

	var P, Q, Pi, Qi [][]*big.Int
	Pi = RecursivePolynomials(Pi, NewPolynomial(nil), a.SliceRaw(0, m), b.SliceRaw(0, m))
	Qi = RecursivePolynomials(Qi, NewPolynomial(nil), a.SliceRaw(m, 2*m), b.SliceRaw(m, 2*m))

	// transpose the matrices
	for i := 0; i < m; i++ {
		P = append(P, []*big.Int{})
		Q = append(Q, []*big.Int{})
		for j := range Pi {
			P[i] = append(P[i], Pi[j][i])
			Q[i] = append(Q[i], Qi[j][i])
		}
	}

	//	for i := range P {
	//		for j := range P[i] {
	//			klog.V(2).Infof("P%d,%d %s\n", i, j, P[i][j].Text(16))
	//		}
	//	}

	//phi := NewFieldVectorRandomFilled(m)
	//chi := NewFieldVectorRandomFilled(m)
	//psi := NewFieldVectorRandomFilled(m)

	var phi, chi, psi ElGamalVector
	for i := 0; i < m; i++ {
		phi.vector = append(phi.vector, CommitElGamal(s.Publickeylist[witness.Index[0]], new(big.Int).SetUint64(0)))
		chi.vector = append(chi.vector, CommitElGamal(s.Publickeylist[witness.Index[0]], new(big.Int).SetUint64(0)))
		psi.vector = append(psi.vector, CommitElGamal(s.Publickeylist[witness.Index[0]], new(big.Int).SetUint64(0)))
	}

	var CnG, C_0G, y_0G ElGamalVector
	for i := 0; i < m; i++ {

		CnG.vector = append(CnG.vector, Cn.MultiExponentiate(NewFieldVector(P[i])).Add(phi.vector[i]))

		{
			var pvector PointVector
			for j := range C {
				pvector.vector = append(pvector.vector, C[j].Left)
			}
			left := pvector.MultiExponentiate(NewFieldVector(P[i])) //.Add(chi.vector[i].Left)
			left = new(bn256.G1).Add(new(bn256.G1).Set(left), chi.vector[i].Left)
			C_0G.vector = append(C_0G.vector, ConstructElGamal(left, chi.vector[i].Right))
		}

		{

			left := NewPointVector(s.Publickeylist).MultiExponentiate(NewFieldVector(P[i]))
			left = new(bn256.G1).Add(new(bn256.G1).Set(left), psi.vector[i].Left)
			y_0G.vector = append(y_0G.vector, ConstructElGamal(left, psi.vector[i].Right))
		}

		/*
			{ // y_0G
				var rightp, result bn256.G1
				leftp := NewPointVector(s.Publickeylist).Commit(P[i])
				rightp.ScalarMult(s.Publickeylist[witness.Index[0]], psi.vector[i])
				result.Add(leftp, &rightp)
				proof.y_0G = append(proof.y_0G, &result)
				//klog.V(2).Infof("y_0G %d %s\n",i, result.String())
			}

			{ // gG
				var result bn256.G1
				result.ScalarMult(params.G, psi.vector[i])
				proof.gG = append(proof.gG, &result)
				//klog.V(2).Infof("gG %d %s\n",i, result.String())
			}
		*/

		/*
			{ // C_XG
				var result bn256.G1
				result.ScalarMult(s.D, omega.vector[i])
				proof.C_XG = append(proof.C_XG, &result)
				//klog.V(2).Infof("C_XG %d %s\n",i, result.String())
			}

			{ // y_XG
				var result bn256.G1
				result.ScalarMult(params.G, omega.vector[i])
				proof.y_XG = append(proof.y_XG, &result)
				klog.V(2).Infof("y_XG %d %s\n", i, result.String())
			}
		*/

	}

	/*
		for i := range proof.CLnG {
			klog.V(2).Infof("CLnG %d %s\n", i, proof.CLnG[i].String())
		}
		for i := range proof.CRnG {
			klog.V(2).Infof("CRnG %d %s\n", i, proof.CRnG[i].String())
		}
		for i := range proof.C_0G {
			klog.V(2).Infof("C_0G %d %s\n", i, proof.C_0G[i].String())
		}
		for i := range proof.DG {
			klog.V(2).Infof("DG %d %s\n", i, proof.DG[i].String())
		}
		for i := range proof.y_0G {
			klog.V(2).Infof("y_0G %d %s\n", i, proof.y_0G[i].String())
		}
		for i := range proof.gG {
			klog.V(2).Infof("gG %d %s\n", i, proof.gG[i].String())
		}
		for i := range proof.C_XG {
			klog.V(2).Infof("C_XG %d %s\n", i, proof.C_XG[i].String())
		}
		for i := range proof.y_XG {
			klog.V(2).Infof("y_XG %d %s\n", i, proof.y_XG[i].String())
		}
	*/
	var C_XG []*ElGamal
	for i := 0; i < m; i++ {
		C_XG = append(C_XG, CommitElGamal(C[0].Right, new(big.Int).SetUint64(0)))
	}

	vPow := new(big.Int).SetInt64(1) // doesn't need reduction, since it' alredy reduced

	for i := 0; i < N; i++ {

		var poly [][]*big.Int
		if i%2 == 0 {
			poly = P
		} else {
			poly = Q
		}

		//	klog.V(2).Infof("\n\n")
		//	for i := range proof.C_XG {
		//		klog.V(2).Infof("C_XG before %d %s\n", i, proof.C_XG[i].String())
		//	}

		//	klog.V(2).Infof("loop %d pos in poly sender %d receiver %d\n", i, (witness.Index[0]+N-(i-i%2))%N, (witness.Index[1]+N-(i-i%2))%N)

		for j := range C_XG {

			amount := new(big.Int).SetUint64(uint64(witness.TransferAmount))
			amount_neg := new(big.Int).Neg(amount)
			amount_fees := new(big.Int).SetUint64(s.Fees + burn_value)
			left := new(big.Int).Sub(amount_neg, amount_fees)
			left = new(big.Int).Mod(new(big.Int).Mul(new(big.Int).Set(left), poly[j][(witness.Index[0]+N-(i-i%2))%N]), bn256.Order)

			right := new(big.Int).Mod(new(big.Int).Mul(new(big.Int).Set(amount), poly[j][(witness.Index[1]+N-(i-i%2))%N]), bn256.Order)

			joined := new(big.Int).Mod(new(big.Int).Add(left, right), bn256.Order)

			mul := new(big.Int).Mod(new(big.Int).Mul(vPow, joined), bn256.Order)

			C_XG[j] = C_XG[j].Plus(mul)
		}

		if i != 0 {
			vPow.Mul(vPow, v)
			vPow.Mod(vPow, bn256.Order)
		}

		//klog.V(2).Infof("vPow %d %s\n", i, vPow.Text(16)))

	}

	for i := range C_XG {
		proof.C_XG = append(proof.C_XG, C_XG[i].Left)
		proof.y_XG = append(proof.y_XG, C_XG[i].Right)

		proof.CLnG = append(proof.CLnG, CnG.vector[i].Left)
		proof.CRnG = append(proof.CRnG, CnG.vector[i].Right)

		proof.C_0G = append(proof.C_0G, C_0G.vector[i].Left)
		proof.DG = append(proof.DG, C_0G.vector[i].Right)

		proof.y_0G = append(proof.y_0G, y_0G.vector[i].Left)
		proof.gG = append(proof.gG, y_0G.vector[i].Right)
	}

	//klog.V(2).Infof("\n\n")
	//	for i := range proof.C_XG {
	//	klog.V(2).Infof("C_XG after %d %s\n", i, proof.C_XG[i].String())
	//	}

	// calculate w hashmash

	w := proof.hashmash1(v)

	{
		var input []byte

		input = append(input, ConvertBigIntToByte(v)...)
		for i := range proof.CLnG {
			input = append(input, proof.CLnG[i].Marshal()...)
		}
		for i := range proof.CRnG {
			input = append(input, proof.CRnG[i].Marshal()...)
		}

		for i := range proof.C_0G {
			input = append(input, proof.C_0G[i].Marshal()...)
		}
		for i := range proof.DG {
			input = append(input, proof.DG[i].Marshal()...)
		}
		for i := range proof.y_0G {
			input = append(input, proof.y_0G[i].Marshal()...)
		}
		for i := range proof.gG {
			input = append(input, proof.gG[i].Marshal()...)
		}
		for i := range C_XG {
			input = append(input, C_XG[i].Left.Marshal()...)
		}
		for i := range proof.y_XG {
			input = append(input, C_XG[i].Right.Marshal()...)
		}
		//fmt.Printf("whash     %s  %s\n", reducedhash(input).Text(16), w.Text(16))

	}

	proof.f = b.Times(w).Add(a)

	//	for i := range proof.f.vector {
	//		klog.V(2).Infof("proof.f %d %s\n", i, proof.f.vector[i].Text(16))
	//	}

	ttttt := new(big.Int).Mod(new(big.Int).Mul(proof_B_internal.Randomness, w), bn256.Order)
	proof.z_A = new(big.Int).Mod(new(big.Int).Add(ttttt, proof_A_internal.Randomness), bn256.Order)

	//	klog.V(2).Infof("proofz_A  %s\n", proof.z_A.Text(16))

	y := reducedhash(ConvertBigIntToByte(w))

	//	klog.V(2).Infof("yyyyyyyyyy  %s\n", y.Text(16))

	ys_raw := []*big.Int{new(big.Int).SetUint64(1)}
	for i := 1; i < 128; i++ {
		var tt big.Int
		tt.Mul(ys_raw[len(ys_raw)-1], y)
		tt.Mod(&tt, bn256.Order)
		ys_raw = append(ys_raw, &tt)
	}
	ys := NewFieldVector(ys_raw)

	z := reducedhash(ConvertBigIntToByte(y))
	//	klog.V(2).Infof("zzzzzzzzzz  %s \n", z.Text(16))

	zs := []*big.Int{new(big.Int).Exp(z, new(big.Int).SetUint64(2), bn256.Order), new(big.Int).Exp(z, new(big.Int).SetUint64(3), bn256.Order)}
	//	for i := range zs {
	//		klog.V(2).Infof("zs %d %s\n", i, zs[i].Text(16))
	//	}

	twos := []*big.Int{new(big.Int).SetUint64(1)}
	for i := 1; i < 64; i++ {
		var tt big.Int
		tt.Mul(twos[len(twos)-1], new(big.Int).SetUint64(2))
		tt.Mod(&tt, bn256.Order)
		twos = append(twos, &tt)
	}

	twoTimesZs := []*big.Int{}
	for i := 0; i < 2; i++ {
		for j := 0; j < 64; j++ {
			var tt big.Int
			tt.Mul(zs[i], twos[j])
			tt.Mod(&tt, bn256.Order)
			twoTimesZs = append(twoTimesZs, &tt)

			//		klog.V(2).Infof("twoTimesZssss ============= %d %s\n", i*32+j, twoTimesZs[i*32+j].Text(16))

		}
	}

	tmp := aL.AddConstant(new(big.Int).Mod(new(big.Int).Neg(z), bn256.Order))
	lPoly := NewFieldVectorPolynomial(tmp, sL)
	//for i := range lPoly.coefficients {
	//	for j := range lPoly.coefficients[i].vector {
	//		klog.V(2).Infof("lPoly %d,%d %s\n", i, j, lPoly.coefficients[i].vector[j].Text(16))
	//	}
	//}

	rPoly := NewFieldVectorPolynomial(ys.Hadamard(aR.AddConstant(z)).Add(NewFieldVector(twoTimesZs)), sR.Hadamard(ys))
	//for i := range rPoly.coefficients {
	//	for j := range rPoly.coefficients[i].vector {
	//		klog.V(2).Infof("rPoly %d,%d %s\n", i, j, rPoly.coefficients[i].vector[j].Text(16))
	//	}
	//}

	tPolyCoefficients := lPoly.InnerProduct(rPoly) // just an array of BN Reds... should be length 3
	//for j := range tPolyCoefficients {
	//	klog.V(2).Infof("tPolyCoefficients %d,%d %s\n", 0, j, tPolyCoefficients[j].Text(16))
	//}

	proof_T1 := NewPedersenCommitmentNew().Commit(tPolyCoefficients[1])
	proof_T2 := NewPedersenCommitmentNew().Commit(tPolyCoefficients[2])
	proof.T_1 = proof_T1.Result
	proof.T_2 = proof_T2.Result

	//polyCommitment := NewPolyCommitment(params, tPolyCoefficients)
	/*proof.tCommits = NewPointVector(polyCommitment.GetCommitments())

	for j := range proof.tCommits.vector {
		klog.V(2).Infof("tCommits %d %s\n", j, proof.tCommits.vector[j].String())
	}
	*/

	x := new(big.Int)

	{
		var input []byte
		input = append(input, ConvertBigIntToByte(z)...) // tie intermediates/commit
		//for j := range proof.tCommits.vector {
		//	input = append(input, proof.tCommits.vector[j].Marshal()...)
		//}
		input = append(input, proof_T1.Result.Marshal()...)
		input = append(input, proof_T2.Result.Marshal()...)
		x = reducedhash(input)
	}

	//klog.V(2).Infof("x  %s\n", x.Text(16))

	//evalCommit := polyCommitment.Evaluate(x)

	//klog.V(2).Infof("evalCommit.X  %s\n", j, evalCommit.X.Text(16))
	//klog.V(2).Infof("evalCommit.R  %s\n", j, evalCommit.R.Text(16))

	//proof.that = evalCommit.X

	xsquare := new(big.Int).Mod(new(big.Int).Mul(x, x), bn256.Order)

	proof.that = tPolyCoefficients[0]
	proof.that = new(big.Int).Mod(new(big.Int).Add(proof.that, new(big.Int).Mod(new(big.Int).Mul(tPolyCoefficients[1], x), bn256.Order)), bn256.Order)
	proof.that = new(big.Int).Mod(new(big.Int).Add(proof.that, new(big.Int).Mod(new(big.Int).Mul(tPolyCoefficients[2], xsquare), bn256.Order)), bn256.Order)

	/*
		accumulator := new(big.Int).Set(x)
		for i := 1; i < 3; i++ {
			tmp := new(big.Int).Set(accumulator)
			proof.that = proof.that.Add(new(bn256.G1).Set(proof.that), tPolyCoefficients[i].Times(accumulator))
			accumulator.Mod(new(big.Int).Mul(tmp, x), bn256.Order)
		}
	*/

	//klog.V(2).Infof("evalCommit.that  %s\n", proof.that.Text(16))

	//tauX := evalCommit.R
	tauX_left := new(big.Int).Mod(new(big.Int).Mul(proof_T1.Randomness, x), bn256.Order)
	tauX_right := new(big.Int).Mod(new(big.Int).Mul(proof_T2.Randomness, xsquare), bn256.Order)
	tauX := new(big.Int).Mod(new(big.Int).Add(tauX_left, tauX_right), bn256.Order)

	proof.mu = new(big.Int).Mod(new(big.Int).Mul(proof_BS_internal.Randomness, x), bn256.Order)
	proof.mu.Add(proof.mu, proof_BA_internal.Randomness)
	proof.mu.Mod(proof.mu, bn256.Order)

	//klog.V(2).Infof("proof.mu  %s\n", proof.mu.Text(16))

	var CrnR, y_0R, y_XR bn256.G1
	// var gR bn256.G1
	CrnR.ScalarMult(params.G, new(big.Int))
	y_0R.ScalarMult(params.G, new(big.Int))
	y_XR.ScalarMult(params.G, new(big.Int))
	//DR.ScalarMult(params.G, new(big.Int))
	//gR.ScalarMult(params.G, new(big.Int))

	CnR := ConstructElGamal(nil, ElGamal_ZERO) // only right side is needer
	chi_bigint := new(big.Int).SetUint64(0)
	psi_bigint := new(big.Int).SetUint64(0)
	C_XR := ConstructElGamal(nil, ElGamal_ZERO) // only right side is needer

	var p_, q_ []*big.Int
	for i := 0; i < N; i++ {
		p_ = append(p_, new(big.Int))
		q_ = append(q_, new(big.Int))
	}
	p := NewFieldVector(p_)
	q := NewFieldVector(q_)

	wPow := new(big.Int).SetUint64(1) // already reduced

	for i := 0; i < m; i++ {

		{
			CnR = CnR.Add(phi.vector[i].Neg().Mul(wPow))
		}

		{
			mm := new(big.Int).Mod(new(big.Int).Mul(chi.vector[i].Randomness, wPow), bn256.Order)
			chi_bigint = new(big.Int).Mod(new(big.Int).Add(chi_bigint, mm), bn256.Order)
		}

		{
			mm := new(big.Int).Mod(new(big.Int).Mul(psi.vector[i].Randomness, wPow), bn256.Order)
			psi_bigint = new(big.Int).Mod(new(big.Int).Add(psi_bigint, mm), bn256.Order)
		}

		/*	{
				tmp := new(bn256.G1)
				mm := new(big.Int).Mod(new(big.Int).Neg(chi.vector[i]), bn256.Order)
				mm = mm.Mod(new(big.Int).Mul(mm, wPow), bn256.Order)
				tmp.ScalarMult(params.G, mm)
				DR.Add(new(bn256.G1).Set(&DR), tmp)
			}

			{
				tmp := new(bn256.G1)
				mm := new(big.Int).Mod(new(big.Int).Neg(psi.vector[i]), bn256.Order)
				mm = mm.Mod(new(big.Int).Mul(mm, wPow), bn256.Order)
				tmp.ScalarMult(s.Publickeylist[witness.Index[0]], mm)
				y_0R.Add(new(bn256.G1).Set(&y_0R), tmp)
			}
		*/
		/*
			{
				tmp := new(bn256.G1)
				mm := new(big.Int).Mod(new(big.Int).Neg(psi.vector[i]), bn256.Order)
				mm = mm.Mod(new(big.Int).Mul(mm, wPow), bn256.Order)
				tmp.ScalarMult(params.G, mm)
				gR.Add(new(bn256.G1).Set(&gR), tmp)
			}

			{
				tmp := new(bn256.G1)
				tmp.ScalarMult(proof.y_XG[i], new(big.Int).Neg(wPow))
				y_XR.Add(new(bn256.G1).Set(&y_XR), tmp)
			}
		*/

		{
			C_XR = C_XR.Add(C_XG[i].Neg().Mul(wPow))
		}

		//fmt.Printf("y_XG[%d] %s\n",i, proof.y_XG[i].String())
		//fmt.Printf("C_XG[%d] %s\n",i, C_XG[i].Right.String())

		//fmt.Printf("G %s\n",G.String())
		//fmt.Printf("elgamalG %s\n",C_XG[0].G.String())

		p = p.Add(NewFieldVector(P[i]).Times(wPow))
		q = q.Add(NewFieldVector(Q[i]).Times(wPow))
		wPow = new(big.Int).Mod(new(big.Int).Mul(wPow, w), bn256.Order)

		//	klog.V(2).Infof("wPow %s\n", wPow.Text(16))

	}

	CnR = CnR.Add(Cn.vector[witness.Index[0]].Mul(wPow))

	//for i := range CnR{
	//	proof.CLnG = append(proof.CLnG, CnR[i].Left)
	//	proof.CRnG = append(proof.CRnG, CnR[i].Right)
	//}

	//CrnR.Add(new(bn256.G1).Set(&CrnR), new(bn256.G1).ScalarMult(s.CRn[witness.Index[0]], wPow))
	//y_0R.Add(new(bn256.G1).Set(&y_0R), new(bn256.G1).ScalarMult(s.Publickeylist[witness.Index[0]], wPow))
	//DR.Add(new(bn256.G1).Set(&DR), new(bn256.G1).ScalarMult(s.D, wPow))

	DR := new(bn256.G1).ScalarMult(C[0].Right, wPow)
	DR = new(bn256.G1).Add(new(bn256.G1).Set(DR), new(bn256.G1).ScalarMult(global_pedersen_values.G, new(big.Int).Mod(new(big.Int).Neg(chi_bigint), bn256.Order)))

	gR := new(bn256.G1).ScalarMult(global_pedersen_values.G, new(big.Int).Mod(new(big.Int).Sub(wPow, psi_bigint), bn256.Order))

	//gR.Add(new(bn256.G1).Set(&gR), new(bn256.G1).ScalarMult(params.G, wPow))

	var p__, q__ []*big.Int
	for i := 0; i < N; i++ {

		if i == witness.Index[0] {
			p__ = append(p__, new(big.Int).Set(wPow))
		} else {
			p__ = append(p__, new(big.Int))
		}

		if i == witness.Index[1] {
			q__ = append(q__, new(big.Int).Set(wPow))
		} else {
			q__ = append(q__, new(big.Int))
		}
	}
	p = p.Add(NewFieldVector(p__))
	q = q.Add(NewFieldVector(q__))

	//	klog.V(2).Infof("CrnR %s\n", CrnR.String())
	//	klog.V(2).Infof("DR %s\n", DR.String())
	//	klog.V(2).Infof("y_0R %s\n", y_0R.String())
	//	klog.V(2).Infof("gR %s\n", gR.String())
	//	klog.V(2).Infof("y_XR %s\n", y_XR.String())

	//	for i := range p.vector {
	//		klog.V(2).Infof("p %d %s \n", i, p.vector[i].Text(16))
	//	}

	//	for i := range q.vector {
	//		klog.V(2).Infof("q %d %s \n", i, q.vector[i].Text(16))
	//	}

	y_p := Convolution(p, NewPointVector(s.Publickeylist))
	y_q := Convolution(q, NewPointVector(s.Publickeylist))

	//	for i := range y_p.vector {
	//		klog.V(2).Infof("y_p %d %s \n", i, y_p.vector[i].String())
	//	}
	//	for i := range y_q.vector {
	//		klog.V(2).Infof("y_q %d %s \n", i, y_q.vector[i].String())
	//	}

	vPow = new(big.Int).SetUint64(1) // already reduced
	for i := 0; i < N; i++ {

		ypoly := y_p
		if i%2 == 1 {
			ypoly = y_q
		}
		y_XR.Add(new(bn256.G1).Set(&y_XR), new(bn256.G1).ScalarMult(ypoly.vector[i/2], vPow))

		C_XR = C_XR.Add(ConstructElGamal(nil, new(bn256.G1).ScalarMult(ypoly.vector[i/2], vPow)))

		//fmt.Printf("y_XR[%d] %s\n",i, y_XR.String())
		//fmt.Printf("C_XR[%d] %s\n",i, C_XR.Right.String())
		if i > 0 {
			vPow = new(big.Int).Mod(new(big.Int).Mul(vPow, v), bn256.Order)
		}
	}

	//	klog.V(2).Infof("y_XR %s\n", y_XR.String())
	//	klog.V(2).Infof("vPow %s\n", vPow.Text(16))
	//	klog.V(2).Infof("v %s\n", v.Text(16))

	k_sk := RandomScalarFixed()
	k_r := RandomScalarFixed()
	k_b := RandomScalarFixed()
	k_tau := RandomScalarFixed()

	A_y := new(bn256.G1).ScalarMult(gR, k_sk)
	A_D := new(bn256.G1).ScalarMult(params.G, k_r)
	A_b := new(bn256.G1).ScalarMult(params.G, k_b)
	t1 := new(bn256.G1).ScalarMult(CnR.Right, zs[1])
	d1 := new(bn256.G1).ScalarMult(DR, new(big.Int).Mod(new(big.Int).Neg(zs[0]), bn256.Order))
	d1 = new(bn256.G1).Add(d1, t1)
	d1 = new(bn256.G1).ScalarMult(d1, k_sk)
	A_b = new(bn256.G1).Add(A_b, d1)

	A_X := new(bn256.G1).ScalarMult(C_XR.Right, k_r)

	A_t := new(bn256.G1).ScalarMult(params.G, new(big.Int).Mod(new(big.Int).Neg(k_b), bn256.Order))
	A_t = new(bn256.G1).Add(A_t, new(bn256.G1).ScalarMult(params.H, k_tau))

	A_u := new(bn256.G1)

	{
		var input []byte
		input = append(input, []byte(PROTOCOL_CONSTANT)...)
		input = append(input, s.Roothash[:]...)

		scid_index_str := strconv.Itoa(scid_index)
		input = append(input, scid[:]...)
		input = append(input, scid_index_str...)

		point := HashToPoint(HashtoNumber(input))

		A_u = new(bn256.G1).ScalarMult(point, k_sk)
	}

	//	klog.V(2).Infof("A_y %s\n", A_y.String())
	//	klog.V(2).Infof("A_D %s\n", A_D.String())
	//	klog.V(2).Infof("A_b %s\n", A_b.String())
	//	klog.V(2).Infof("A_X %s\n", A_X.String())
	//	klog.V(2).Infof("A_t %s\n", A_t.String())
	//	klog.V(2).Infof("A_u %s\n", A_u.String())

	{
		var input []byte
		input = append(input, ConvertBigIntToByte(x)...)
		input = append(input, A_y.Marshal()...)
		input = append(input, A_D.Marshal()...)
		input = append(input, A_b.Marshal()...)
		input = append(input, A_X.Marshal()...)
		input = append(input, A_t.Marshal()...)
		input = append(input, A_u.Marshal()...)
		proof.c = reducedhash(input)
	}

	proof.s_sk = new(big.Int).Mod(new(big.Int).Mul(proof.c, witness.SecretKey), bn256.Order)
	proof.s_sk = new(big.Int).Mod(new(big.Int).Add(proof.s_sk, k_sk), bn256.Order)

	proof.s_r = new(big.Int).Mod(new(big.Int).Mul(proof.c, witness.R), bn256.Order)
	proof.s_r = new(big.Int).Mod(new(big.Int).Add(proof.s_r, k_r), bn256.Order)

	//	proof_c_neg := new(big.Int).Mod(new(big.Int).Neg(proof.c), bn256.Order)
	//	dummyA_X := new(bn256.G1).ScalarMult(&y_XR, proof.s_r) //, new(bn256.G1).ScalarMult(anonsupport.C_XR, proof_c_neg) )

	//	klog.V(2).Infof("dummyA_X %s\n", dummyA_X.String())
	//	klog.V(2).Infof("s_r %s\n", proof.s_r.Text(16))
	//	klog.V(2).Infof("C %s\n", proof.c.Text(16))
	//	klog.V(2).Infof("C_neg %s\n", proof_c_neg.Text(16))

	w_transfer := new(big.Int).Mod(new(big.Int).Mul(new(big.Int).SetUint64(uint64(witness.TransferAmount)), zs[0]), bn256.Order)
	w_balance := new(big.Int).Mod(new(big.Int).Mul(new(big.Int).SetUint64(uint64(witness.Balance)), zs[1]), bn256.Order)
	w_tmp := new(big.Int).Mod(new(big.Int).Add(w_transfer, w_balance), bn256.Order)
	w_tmp = new(big.Int).Mod(new(big.Int).Mul(w_tmp, wPow), bn256.Order)
	w_tmp = new(big.Int).Mod(new(big.Int).Mul(w_tmp, proof.c), bn256.Order)
	proof.s_b = new(big.Int).Mod(new(big.Int).Add(w_tmp, k_b), bn256.Order)

	proof.s_tau = new(big.Int).Mod(new(big.Int).Mul(tauX, wPow), bn256.Order)
	proof.s_tau = new(big.Int).Mod(new(big.Int).Mul(proof.s_tau, proof.c), bn256.Order)
	proof.s_tau = new(big.Int).Mod(new(big.Int).Add(proof.s_tau, k_tau), bn256.Order)

	//	klog.V(2).Infof("proof.c %s\n", proof.c.Text(16))
	//	klog.V(2).Infof("proof.s_sk %s\n", proof.s_sk.Text(16))
	//	klog.V(2).Infof("proof.s_r %s\n", proof.s_r.Text(16))
	//	klog.V(2).Infof("proof.s_b %s\n", proof.s_b.Text(16))
	//	klog.V(2).Infof("proof.s_tau %s\n", proof.s_tau.Text(16))

	o := reducedhash(ConvertBigIntToByte(proof.c))

	pvector := NewPedersenVectorCommitment()
	pvector.H = new(bn256.G1).ScalarMult(pvector.H, o)
	pvector.Hs = pvector.Hs.Hadamard(ys.Invert().vector)
	pvector.gvalues = lPoly.Evaluate(x)
	pvector.hvalues = rPoly.Evaluate(x)
	proof.ip = NewInnerProductProofNew(pvector, o)
	/*

		u_x := new(bn256.G1).ScalarMult(params.G, o)
		P1 = new(bn256.G1).Add(P1, new(bn256.G1).ScalarMult(u_x, proof.that))
		klog.V(2).Infof("o %s\n", o.Text(16))
		klog.V(2).Infof("x %s\n", x.Text(16))
		klog.V(2).Infof("u_x %s\n", u_x.String())
		klog.V(2).Infof("p %s\n", P1.String())
		klog.V(2).Infof("hPrimes length %d\n", len(hPrimes.vector))

		primebase := NewGeneratorParams3(u_x, params.Gs, hPrimes) // trigger sigma protocol
		ipstatement := &IPStatement{PrimeBase: primebase, P: P1}
		ipwitness := &IPWitness{L: lPoly.Evaluate(x), R: rPoly.Evaluate(x)}

		for i := range ipwitness.L.vector {
			klog.V(2).Infof("L %d %s \n", i, ipwitness.L.vector[i].Text(16))
		}

		for i := range ipwitness.R.vector {
			klog.V(2).Infof("R %d %s \n", i, ipwitness.R.vector[i].Text(16))
		}

		proof.ip = NewInnerProductProof(ipstatement, ipwitness, o)
	*/

	return &proof

}
