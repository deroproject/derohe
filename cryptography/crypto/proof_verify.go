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
import "strconv"

//import "crypto/rand"
import "encoding/hex"

import "github.com/deroproject/derohe/cryptography/bn256"

//import "golang.org/x/crypto/sha3"

// below 2 structures form bulletproofs and many to many proofs
type AnonSupport struct {
	v    *big.Int
	w    *big.Int
	vPow *big.Int
	wPow *big.Int
	f    [][2]*big.Int
	r    [][2]*big.Int
	temp *bn256.G1
	CLnR *bn256.G1
	CRnR *bn256.G1
	CR   [][2]*bn256.G1
	yR   [][2]*bn256.G1
	C_XR *bn256.G1
	y_XR *bn256.G1
	gR   *bn256.G1
	DR   *bn256.G1
}

type ProtocolSupport struct {
	y                *big.Int
	ys               []*big.Int
	z                *big.Int
	zs               []*big.Int // [z^2, z^3] // only max 2
	twoTimesZSquared [128]*big.Int
	zSum             *big.Int
	x                *big.Int
	t                *big.Int
	k                *big.Int
	tEval            *bn256.G1
}

// sigma protocol
type SigmaSupport struct {
	c                            *big.Int
	A_y, A_D, A_b, A_X, A_t, A_u *bn256.G1
}

// support structures are those which
type InnerProductSupport struct {
	P         bn256.G1
	u_x       bn256.G1
	hPrimes   []*bn256.G1
	hPrimeSum bn256.G1
	o         *big.Int
}

func unmarshalpoint(input string) *bn256.G1 {
	d, err := hex.DecodeString(input)
	if err != nil {
		panic(err)
	}

	if len(d) != 64 {
		panic("wrong length")
	}

	x := new(bn256.G1)
	x.Unmarshal(d)
	return x

}

var gparams = NewGeneratorParams(128) // these can be pregenerated similarly as in DERO project

// verify proof
// first generate supporting structures
func (proof *Proof) Verify(scid Hash, scid_index int, s *Statement, txid Hash, extra_value uint64) bool {

	var anonsupport AnonSupport
	var protsupport ProtocolSupport
	var sigmasupport SigmaSupport

	if len(s.C) != len(s.Publickeylist) {
		return false
	}

	total_open_value := s.Fees + extra_value
	if total_open_value < s.Fees || total_open_value < extra_value { // stop over flowing attacks
		return false
	}

	statementhash := reducedhash(txid[:])
	var input []byte
	input = append(input, ConvertBigIntToByte(statementhash)...)
	input = append(input, proof.BA.Marshal()...)
	input = append(input, proof.BS.Marshal()...)
	input = append(input, proof.A.Marshal()...)
	input = append(input, proof.B.Marshal()...)
	anonsupport.v = reducedhash(input)

	anonsupport.w = proof.hashmash1(anonsupport.v)

	m := proof.f.Length() / 2
	N := int(math.Pow(2, float64(m)))

	anonsupport.f = make([][2]*big.Int, 2*m, 2*m)

	// the secret parity is checked cryptographically
	for k := 0; k < 2*m; k++ {
		anonsupport.f[k][1] = new(big.Int).Set(proof.f.vector[k])
		anonsupport.f[k][0] = new(big.Int).Mod(new(big.Int).Sub(anonsupport.w, proof.f.vector[k]), bn256.Order)
	}

	// check parity condition
	if anonsupport.w.Cmp(proof.f.vector[0]) == 0 || anonsupport.w.Cmp(proof.f.vector[m]) == 0 {
		//	fmt.Printf("parity is well formed\n")
	} else { // test failed, reject the tx
		Logger.V(1).Info("Parity check failed")
		return false
	}

	anonsupport.temp = new(bn256.G1)
	var zeroes [64]byte
	anonsupport.temp.Unmarshal(zeroes[:])

	for k := 0; k < 2*m; k++ {
		anonsupport.temp = new(bn256.G1).Add(anonsupport.temp, new(bn256.G1).ScalarMult(gparams.Gs.vector[k], anonsupport.f[k][1]))

		t := new(big.Int).Mod(new(big.Int).Mul(anonsupport.f[k][1], anonsupport.f[k][0]), bn256.Order)

		anonsupport.temp = new(bn256.G1).Add(anonsupport.temp, new(bn256.G1).ScalarMult(gparams.Hs.vector[k], t))
	}

	t0 := new(bn256.G1).ScalarMult(gparams.Hs.vector[0+2*m], new(big.Int).Mod(new(big.Int).Mul(anonsupport.f[0][1], anonsupport.f[m][1]), bn256.Order))
	t1 := new(bn256.G1).ScalarMult(gparams.Hs.vector[1+2*m], new(big.Int).Mod(new(big.Int).Mul(anonsupport.f[0][0], anonsupport.f[m][0]), bn256.Order))

	anonsupport.temp = new(bn256.G1).Add(anonsupport.temp, t0)
	anonsupport.temp = new(bn256.G1).Add(anonsupport.temp, t1)

	// check whether we successfuly recover B^w * A
	stored := new(bn256.G1).Add(new(bn256.G1).ScalarMult(proof.B, anonsupport.w), proof.A)
	computed := new(bn256.G1).Add(anonsupport.temp, new(bn256.G1).ScalarMult(gparams.H, proof.z_A))

	//	for i := range proof.f.vector {
	//		klog.V(2).Infof("proof.f %d %s\n", i, proof.f.vector[i].Text(16))
	//	}
	//	klog.V(2).Infof("anonsupport.w %s\n", anonsupport.w.Text(16))
	//	klog.V(2).Infof("proof.z_A %s\n", proof.z_A.Text(16))
	//	klog.V(2).Infof("proof.B %s\n", proof.B.String())
	//	klog.V(2).Infof("proof.A %s\n", proof.A.String())
	//	klog.V(2).Infof("gparams.H %s\n", gparams.H.String())

	//	klog.V(2).Infof("stored %s\n", stored.String())
	//	klog.V(2).Infof("computed %s\n", computed.String())

	if stored.String() != computed.String() { // if failed bail out
		Logger.V(1).Info("Recover key failed B^w * A")
		return false
	}

	anonsupport.r = assemblepolynomials(anonsupport.f)

	//	for i := 0; i < len(anonsupport.r); i++ {
	//		klog.V(2).Infof("proof.r %d %s\n", i, anonsupport.r[i][0].Text(16))
	//	}
	//	for i := 0; i < len(anonsupport.r); i++ {
	//		klog.V(2).Infof("proof.q %d %s\n", i, anonsupport.r[i][1].Text(16))
	//	}

	anonsupport.CLnR = new(bn256.G1)
	anonsupport.CRnR = new(bn256.G1)
	anonsupport.CLnR.Unmarshal(zeroes[:])
	anonsupport.CRnR.Unmarshal(zeroes[:])
	for i := 0; i < N; i++ {
		anonsupport.CLnR = new(bn256.G1).Add(anonsupport.CLnR, new(bn256.G1).ScalarMult(s.CLn[i], anonsupport.r[i][0]))
		anonsupport.CRnR = new(bn256.G1).Add(anonsupport.CRnR, new(bn256.G1).ScalarMult(s.CRn[i], anonsupport.r[i][0]))
	}

	//	klog.V(2).Infof("qCrnR %s\n", anonsupport.CRnR.String())

	var p, q []*big.Int
	for i := 0; i < len(anonsupport.r); i++ {
		p = append(p, anonsupport.r[i][0])
		q = append(q, anonsupport.r[i][1])
	}

	//	for i := range s.C {
	//		klog.V(2).Infof("S.c %d %s \n", i, s.C[i].String())
	//	}
	// share code with proof generator for better testing
	C_p := Convolution(NewFieldVector(p), NewPointVector(s.C))
	C_q := Convolution(NewFieldVector(q), NewPointVector(s.C))
	y_p := Convolution(NewFieldVector(p), NewPointVector(s.Publickeylist))
	y_q := Convolution(NewFieldVector(q), NewPointVector(s.Publickeylist))

	//	for i := range s.C {
	//		klog.V(2).Infof("S.c %d %s \n", i, s.C[i].String())
	//	}

	//	for i := range y_p.vector {
	//		klog.V(2).Infof("y_p %d %s \n", i, y_p.vector[i].String())
	//	}
	//	for i := range y_q.vector {
	//		klog.V(2).Infof("y_q %d %s \n", i, y_q.vector[i].String())
	//	}

	for i := range C_p.vector { // assemble back
		anonsupport.CR = append(anonsupport.CR, [2]*bn256.G1{C_p.vector[i], C_q.vector[i]})
		anonsupport.yR = append(anonsupport.yR, [2]*bn256.G1{y_p.vector[i], y_q.vector[i]})
	}

	anonsupport.vPow = new(big.Int).SetUint64(1)

	anonsupport.C_XR = new(bn256.G1)
	anonsupport.y_XR = new(bn256.G1)
	anonsupport.C_XR.Unmarshal(zeroes[:])
	anonsupport.y_XR.Unmarshal(zeroes[:])
	for i := 0; i < N; i++ {
		anonsupport.C_XR.Add(new(bn256.G1).Set(anonsupport.C_XR), new(bn256.G1).ScalarMult(anonsupport.CR[i/2][i%2], anonsupport.vPow))
		anonsupport.y_XR.Add(new(bn256.G1).Set(anonsupport.y_XR), new(bn256.G1).ScalarMult(anonsupport.yR[i/2][i%2], anonsupport.vPow))

		if i > 0 {
			anonsupport.vPow = new(big.Int).Mod(new(big.Int).Mul(anonsupport.vPow, anonsupport.v), bn256.Order)
			//			klog.V(2).Infof("vPow %s\n", anonsupport.vPow.Text(16))
		}
	}

	//	klog.V(2).Infof("vPow %s\n", anonsupport.vPow.Text(16))
	//	klog.V(2).Infof("v %s\n", anonsupport.v.Text(16))

	anonsupport.wPow = new(big.Int).SetUint64(1)
	anonsupport.gR = new(bn256.G1)
	anonsupport.gR.Unmarshal(zeroes[:])
	anonsupport.DR = new(bn256.G1)
	anonsupport.DR.Unmarshal(zeroes[:])

	for i := 0; i < m; i++ {
		wPow_neg := new(big.Int).Mod(new(big.Int).Neg(anonsupport.wPow), bn256.Order)
		anonsupport.CLnR.Add(new(bn256.G1).Set(anonsupport.CLnR), new(bn256.G1).ScalarMult(proof.CLnG[i], wPow_neg))
		anonsupport.CRnR.Add(new(bn256.G1).Set(anonsupport.CRnR), new(bn256.G1).ScalarMult(proof.CRnG[i], wPow_neg))

		anonsupport.CR[0][0].Add(new(bn256.G1).Set(anonsupport.CR[0][0]), new(bn256.G1).ScalarMult(proof.C_0G[i], wPow_neg))
		anonsupport.DR.Add(new(bn256.G1).Set(anonsupport.DR), new(bn256.G1).ScalarMult(proof.DG[i], wPow_neg))
		anonsupport.yR[0][0].Add(new(bn256.G1).Set(anonsupport.yR[0][0]), new(bn256.G1).ScalarMult(proof.y_0G[i], wPow_neg))
		anonsupport.gR.Add(new(bn256.G1).Set(anonsupport.gR), new(bn256.G1).ScalarMult(proof.gG[i], wPow_neg))

		anonsupport.C_XR.Add(new(bn256.G1).Set(anonsupport.C_XR), new(bn256.G1).ScalarMult(proof.C_XG[i], wPow_neg))
		anonsupport.y_XR.Add(new(bn256.G1).Set(anonsupport.y_XR), new(bn256.G1).ScalarMult(proof.y_XG[i], wPow_neg))

		anonsupport.wPow = new(big.Int).Mod(new(big.Int).Mul(anonsupport.wPow, anonsupport.w), bn256.Order)

	}
	//	klog.V(2).Infof("qCrnR %s\n", anonsupport.CRnR.String())

	anonsupport.DR.Add(new(bn256.G1).Set(anonsupport.DR), new(bn256.G1).ScalarMult(s.D, anonsupport.wPow))
	anonsupport.gR.Add(new(bn256.G1).Set(anonsupport.gR), new(bn256.G1).ScalarMult(gparams.G, anonsupport.wPow))
	anonsupport.C_XR.Add(new(bn256.G1).Set(anonsupport.C_XR), new(bn256.G1).ScalarMult(gparams.G, new(big.Int).Mod(new(big.Int).Mul(new(big.Int).SetUint64(total_open_value), anonsupport.wPow), bn256.Order)))

	//anonAuxiliaries.C_XR = anonAuxiliaries.C_XR.add(Utils.g().mul(Utils.fee().mul(anonAuxiliaries.wPow)));  // this line is new

	// at this point, these parameters are comparable with proof generator
	//	klog.V(2).Infof("CLnR %s\n", anonsupport.CLnR.String())
	//	klog.V(2).Infof("qCrnR %s\n", anonsupport.CRnR.String())
	//	klog.V(2).Infof("DR %s\n", anonsupport.DR.String())
	//	klog.V(2).Infof("gR %s\n", anonsupport.gR.String())
	//	klog.V(2).Infof("C_XR %s\n", anonsupport.C_XR.String())
	//	klog.V(2).Infof("y_XR %s\n", anonsupport.y_XR.String())

	protsupport.y = reducedhash(ConvertBigIntToByte(anonsupport.w))
	protsupport.ys = append(protsupport.ys, new(big.Int).SetUint64(1))
	protsupport.k = new(big.Int).SetUint64(1)
	for i := 1; i < 128; i++ {
		protsupport.ys = append(protsupport.ys, new(big.Int).Mod(new(big.Int).Mul(protsupport.ys[i-1], protsupport.y), bn256.Order))
		protsupport.k = new(big.Int).Mod(new(big.Int).Add(protsupport.k, protsupport.ys[i]), bn256.Order)
	}

	protsupport.z = reducedhash(ConvertBigIntToByte(protsupport.y))
	protsupport.zs = []*big.Int{new(big.Int).Exp(protsupport.z, new(big.Int).SetUint64(2), bn256.Order), new(big.Int).Exp(protsupport.z, new(big.Int).SetUint64(3), bn256.Order)}

	protsupport.zSum = new(big.Int).Mod(new(big.Int).Add(protsupport.zs[0], protsupport.zs[1]), bn256.Order)
	protsupport.zSum = new(big.Int).Mod(new(big.Int).Mul(new(big.Int).Set(protsupport.zSum), protsupport.z), bn256.Order)

	//	klog.V(2).Infof("zsum %s\n ", protsupport.zSum.Text(16))

	z_z0 := new(big.Int).Mod(new(big.Int).Sub(protsupport.z, protsupport.zs[0]), bn256.Order)
	protsupport.k = new(big.Int).Mod(new(big.Int).Mul(protsupport.k, z_z0), bn256.Order)

	proof_2_64, _ := new(big.Int).SetString("18446744073709551616", 10)
	zsum_pow := new(big.Int).Mod(new(big.Int).Mul(protsupport.zSum, proof_2_64), bn256.Order)
	zsum_pow = new(big.Int).Mod(new(big.Int).Sub(zsum_pow, protsupport.zSum), bn256.Order)
	protsupport.k = new(big.Int).Mod(new(big.Int).Sub(protsupport.k, zsum_pow), bn256.Order)

	protsupport.t = new(big.Int).Mod(new(big.Int).Sub(proof.that, protsupport.k), bn256.Order) // t = tHat - delta(y, z)

	//	klog.V(2).Infof("that %s\n ", proof.that.Text(16))
	//	klog.V(2).Infof("zk %s\n ", protsupport.k.Text(16))

	for i := 0; i < 64; i++ {
		protsupport.twoTimesZSquared[i] = new(big.Int).Mod(new(big.Int).Mul(protsupport.zs[0], new(big.Int).SetUint64(uint64(math.Pow(2, float64(i))))), bn256.Order)
		protsupport.twoTimesZSquared[64+i] = new(big.Int).Mod(new(big.Int).Mul(protsupport.zs[1], new(big.Int).SetUint64(uint64(math.Pow(2, float64(i))))), bn256.Order)
	}

	//	for i := 0; i < 128; i++ {
	//		klog.V(2).Infof("zsq %d %s", i, protsupport.twoTimesZSquared[i].Text(16))
	//	}

	x := new(big.Int)

	{
		var input []byte
		input = append(input, ConvertBigIntToByte(protsupport.z)...) // tie intermediates/commit
		input = append(input, proof.T_1.Marshal()...)
		input = append(input, proof.T_2.Marshal()...)
		x = reducedhash(input)
	}

	xsq := new(big.Int).Mod(new(big.Int).Mul(x, x), bn256.Order)
	protsupport.tEval = new(bn256.G1).ScalarMult(proof.T_1, x)
	protsupport.tEval.Add(new(bn256.G1).Set(protsupport.tEval), new(bn256.G1).ScalarMult(proof.T_2, xsq))

	//fmt.Printf("protsupport.tEval %s\n", protsupport.tEval.String())

	proof_c_neg := new(big.Int).Mod(new(big.Int).Neg(proof.c), bn256.Order)

	sigmasupport.A_y = new(bn256.G1).Add(new(bn256.G1).ScalarMult(anonsupport.gR, proof.s_sk), new(bn256.G1).ScalarMult(anonsupport.yR[0][0], proof_c_neg))
	sigmasupport.A_D = new(bn256.G1).Add(new(bn256.G1).ScalarMult(gparams.G, proof.s_r), new(bn256.G1).ScalarMult(s.D, proof_c_neg))

	zs0_neg := new(big.Int).Mod(new(big.Int).Neg(protsupport.zs[0]), bn256.Order)

	left := new(bn256.G1).ScalarMult(anonsupport.DR, zs0_neg)
	left.Add(new(bn256.G1).Set(left), new(bn256.G1).ScalarMult(anonsupport.CRnR, protsupport.zs[1]))
	left = new(bn256.G1).ScalarMult(new(bn256.G1).Set(left), proof.s_sk)

	// TODO mid seems wrong
	amount_fees := new(big.Int).SetUint64(total_open_value)
	mid := new(bn256.G1).ScalarMult(G, new(big.Int).Mod(new(big.Int).Mul(amount_fees, anonsupport.wPow), bn256.Order))
	mid.Add(new(bn256.G1).Set(mid), new(bn256.G1).Set(anonsupport.CR[0][0]))

	right := new(bn256.G1).ScalarMult(mid, zs0_neg)
	right.Add(new(bn256.G1).Set(right), new(bn256.G1).ScalarMult(anonsupport.CLnR, protsupport.zs[1]))
	right = new(bn256.G1).ScalarMult(new(bn256.G1).Set(right), proof_c_neg)

	sigmasupport.A_b = new(bn256.G1).ScalarMult(gparams.G, proof.s_b)

	temp := new(bn256.G1).Add(left, right)
	sigmasupport.A_b.Add(new(bn256.G1).Set(sigmasupport.A_b), temp)

	//-        sigmaAuxiliaries.A_b = Utils.g().mul(proof.s_b).add(anonAuxiliaries.DR.mul(zetherAuxiliaries.zs[0].neg()).add(anonAuxiliaries.CRnR.mul(zetherAuxiliaries.zs[1])).mul(proof.s_sk).add(anonAuxiliaries.CR[0][0]                                                         .mul(zetherAuxiliaries.zs[0].neg()).add(anonAuxiliaries.CLnR.mul(zetherAuxiliaries.zs[1])).mul(proof.c.neg())));
	//+        sigmaAuxiliaries.A_b = Utils.g().mul(proof.s_b).add(anonAuxiliaries.DR.mul(zetherAuxiliaries.zs[0].neg()).add(anonAuxiliaries.CRnR.mul(zetherAuxiliaries.zs[1])).mul(proof.s_sk).add(anonAuxiliaries.CR[0][0].add(Utils.g().mul(Utils.fee().mul(anonAuxiliaries.wPow))).mul(zetherAuxiliaries.zs[0].neg()).add(anonAuxiliaries.CLnR.mul(zetherAuxiliaries.zs[1])).mul(proof.c.neg())));

	//var fees bn256.G1
	//fees.ScalarMult(G, new(big.Int).SetInt64(int64( -1 )))
	//anonsupport.C_XR.Add( new(bn256.G1).Set(anonsupport.C_XR), &fees)

	sigmasupport.A_X = new(bn256.G1).Add(new(bn256.G1).ScalarMult(anonsupport.y_XR, proof.s_r), new(bn256.G1).ScalarMult(anonsupport.C_XR, proof_c_neg))

	proof_s_b_neg := new(big.Int).Mod(new(big.Int).Neg(proof.s_b), bn256.Order)

	sigmasupport.A_t = new(bn256.G1).ScalarMult(gparams.G, protsupport.t)
	sigmasupport.A_t.Add(new(bn256.G1).Set(sigmasupport.A_t), new(bn256.G1).Neg(protsupport.tEval))
	sigmasupport.A_t = new(bn256.G1).ScalarMult(sigmasupport.A_t, new(big.Int).Mod(new(big.Int).Mul(proof.c, anonsupport.wPow), bn256.Order))
	sigmasupport.A_t.Add(new(bn256.G1).Set(sigmasupport.A_t), new(bn256.G1).ScalarMult(gparams.H, proof.s_tau))
	sigmasupport.A_t.Add(new(bn256.G1).Set(sigmasupport.A_t), new(bn256.G1).ScalarMult(gparams.G, proof_s_b_neg))
	//	klog.V(2).Infof("t %s\n ", protsupport.t.Text(16))
	//	klog.V(2).Infof("protsupport.tEval %s\n", protsupport.tEval.String())

	{
		var input []byte
		input = append(input, []byte(PROTOCOL_CONSTANT)...)
		input = append(input, s.Roothash[:]...)

		scid_index_str := strconv.Itoa(scid_index)
		input = append(input, scid[:]...)
		input = append(input, scid_index_str...)

		point := HashToPoint(HashtoNumber(input))

		sigmasupport.A_u = new(bn256.G1).ScalarMult(point, proof.s_sk)
		sigmasupport.A_u.Add(new(bn256.G1).Set(sigmasupport.A_u), new(bn256.G1).ScalarMult(proof.u, proof_c_neg))
	}

	//fmt.Printf("A_y %s\n", sigmasupport.A_y.String())
	//fmt.Printf("A_D %s\n", sigmasupport.A_D.String())
	//fmt.Printf("A_b %s\n", sigmasupport.A_b.String())
	//fmt.Printf("A_X %s\n", sigmasupport.A_X.String())
	//fmt.Printf("A_t %s\n", sigmasupport.A_t.String())
	//fmt.Printf("A_u %s\n", sigmasupport.A_u.String())

	{
		var input []byte
		input = append(input, ConvertBigIntToByte(x)...)
		input = append(input, sigmasupport.A_y.Marshal()...)
		input = append(input, sigmasupport.A_D.Marshal()...)
		input = append(input, sigmasupport.A_b.Marshal()...)
		input = append(input, sigmasupport.A_X.Marshal()...)
		input = append(input, sigmasupport.A_t.Marshal()...)
		input = append(input, sigmasupport.A_u.Marshal()...)

		//fmt.Printf("C calculation expected %s actual %s\n",proof.c.Text(16), reducedhash(input).Text(16) )

		if reducedhash(input).Text(16) != proof.c.Text(16) { // we must fail here
			Logger.V(1).Info("C calculation failed")
			return false
		}
	}

	o := reducedhash(ConvertBigIntToByte(proof.c))

	u_x := new(bn256.G1).ScalarMult(gparams.H, o)

	var hPrimes []*bn256.G1
	hPrimeSum := new(bn256.G1)

	hPrimeSum.Unmarshal(zeroes[:])
	for i := 0; i < 128; i++ {
		hPrimes = append(hPrimes, new(bn256.G1).ScalarMult(gparams.Hs.vector[i], new(big.Int).ModInverse(protsupport.ys[i], bn256.Order)))

		//		klog.V(2).Infof("hPrimes %d %s\n", i, hPrimes[i].String())

		tmp := new(big.Int).Mod(new(big.Int).Mul(protsupport.ys[i], protsupport.z), bn256.Order)
		tmp = new(big.Int).Mod(new(big.Int).Add(tmp, protsupport.twoTimesZSquared[i]), bn256.Order)

		hPrimeSum = new(bn256.G1).Add(hPrimeSum, new(bn256.G1).ScalarMult(hPrimes[i], tmp))

	}

	P := new(bn256.G1).Add(proof.BA, new(bn256.G1).ScalarMult(proof.BS, x))
	P = new(bn256.G1).Add(P, new(bn256.G1).ScalarMult(gparams.GSUM, new(big.Int).Mod(new(big.Int).Neg(protsupport.z), bn256.Order)))
	P = new(bn256.G1).Add(P, hPrimeSum)

	P = new(bn256.G1).Add(P, new(bn256.G1).ScalarMult(gparams.H, new(big.Int).Mod(new(big.Int).Neg(proof.mu), bn256.Order)))
	P = new(bn256.G1).Add(P, new(bn256.G1).ScalarMult(u_x, new(big.Int).Mod(new(big.Int).Set(proof.that), bn256.Order)))

	//	klog.V(2).Infof("P  %s\n", P.String())

	if !proof.ip.Verify(hPrimes, u_x, P, o, gparams) {
		Logger.V(1).Info("inner proof failed")
		return false
	}

	// klog.V(2).Infof("proof %s\n", proof.String())
	// panic("proof  successful")

	//	klog.V(2).Infof("Proof successful verified\n")

	return true

}

/*
	func (proof *Proof) String() string {
		klog.V(1).Infof("proof BA %s\n", proof.BA.String())
		klog.V(1).Infof("proof BS %s\n", proof.BS.String())
		klog.V(1).Infof("proof A %s\n", proof.A.String())
		klog.V(1).Infof("proof B %s\n", proof.B.String())

		for i := range proof.CLnG {
			klog.V(1).Infof("CLnG %d %s \n", i, proof.CLnG[i].String())
		}
		for i := range proof.CRnG {
			klog.V(1).Infof("CRnG %d %s \n", i, proof.CRnG[i].String())
		}

		for i := range proof.C_0G {
			klog.V(1).Infof("C_0G %d %s \n", i, proof.C_0G[i].String())
		}
		for i := range proof.DG {
			klog.V(1).Infof("DG %d %s \n", i, proof.DG[i].String())
		}
		for i := range proof.y_0G {
			klog.V(1).Infof("y_0G %d %s \n", i, proof.y_0G[i].String())
		}
		for i := range proof.gG {
			klog.V(1).Infof("gG %d %s \n", i, proof.gG[i].String())
		}

		for i := range proof.C_XG {
			klog.V(1).Infof("C_XG %d %s \n", i, proof.C_XG[i].String())
		}
		for i := range proof.y_XG {
			klog.V(1).Infof("y_XG %d %s \n", i, proof.y_XG[i].String())
		}

		//for i := range proof.tCommits.vector {
		//	klog.V(1).Infof("tCommits %d %s \n", i, proof.tCommits.vector[i].String())
		//}

		klog.V(1).Infof("proof z_A %s\n", proof.z_A.Text(16))
		klog.V(1).Infof("proof that %s\n", proof.that.Text(16))
		klog.V(1).Infof("proof mu %s\n", proof.mu.Text(16))
		klog.V(1).Infof("proof C %s\n", proof.c.Text(16))
		klog.V(1).Infof("proof s_sk %s\n", proof.s_sk.Text(16))
		klog.V(1).Infof("proof s_r %s\n", proof.s_r.Text(16))
		klog.V(1).Infof("proof s_b %s\n", proof.s_b.Text(16))
		klog.V(1).Infof("proof s_tau %s\n", proof.s_tau.Text(16))

		return ""

}
*/
func assemblepolynomials(f [][2]*big.Int) [][2]*big.Int {
	m := len(f) / 2
	N := int(math.Pow(2, float64(m)))
	result := make([][2]*big.Int, N, N)

	for i := 0; i < 2; i++ {
		half := recursivepolynomials(i*m, (i+1)*m, new(big.Int).SetInt64(1), f)
		for j := 0; j < N; j++ {
			result[j][i] = half[j]
		}
	}
	return result
}

func recursivepolynomials(baseline, current int, accum *big.Int, f [][2]*big.Int) []*big.Int {
	size := int(math.Pow(2, float64(current-baseline)))

	result := make([]*big.Int, size, size)
	if current == baseline {
		result[0] = accum
		return result
	}
	current--

	left := recursivepolynomials(baseline, current, new(big.Int).Mod(new(big.Int).Mul(accum, f[current][0]), bn256.Order), f)
	right := recursivepolynomials(baseline, current, new(big.Int).Mod(new(big.Int).Mul(accum, f[current][1]), bn256.Order), f)
	for i := 0; i < size/2; i++ {
		result[i] = left[i]
		result[i+size/2] = right[i]
	}

	return result
}
