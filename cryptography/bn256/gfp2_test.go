package bn256

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests that the exponentiation in gfp2 works correctly
// SageMath test vector:
/*
p = 21888242871839275222246405745257275088696311157297823662689037894645226208583
Fp = GF(p)
Fpx.<j> = PolynomialRing(Fp, 'j')
// The modulus is in the form `j^2 - non-residue-in-Fp`
// Fp2.<i> = GF(p^2, modulus=j^2 - 3) // 3 is a quadratic non-residue in Fp
// See: https://github.com/scipr-lab/libff/blob/master/libff/algebra/curves/alt_bn128/alt_bn128_init.cpp#L95
// The quadratic non-residue used in -1, so the modulus is
Fp2.<i> = GF(p^2, modulus=j^2 + 1)

// Quad. Non. Resid. test (Euler's criterion)
eulerExp = (Fp(p-1)/Fp(2))
Fp(-1)^eulerExp + Fp(1) == p // Should return true, then we see that -1 (ie: p-1) is a nqr (non quadratic residue mod p)

// Element taken randomly in the field Fp2
// we denote an element of Fp2 as: e = x*i + y
baseElement = 8192512702373747571754527085437364828369119615795326562285198594140975111129*i + 14719814144181528030533020377409648968040866053797156997322427920899698335369

// baseElementXHex = hex(8192512702373747571754527085437364828369119615795326562285198594140975111129)
baseElementXHex             = 121ccc410d6339f7 bbc9a8f5b577c5c5 96c9dfdd6233cbac 34a8ddeafedf9bd9
baseElementXHexLittleEndian = 34a8ddeafedf9bd9 96c9dfdd6233cbac bbc9a8f5b577c5c5 121ccc410d6339f7

// baseElementYHex = hex(14719814144181528030533020377409648968040866053797156997322427920899698335369)
baseElementYHex             = 208b1e9b9b11a98c 30a84b2641e87244 9a54780d0e482cfb 146adf9eb7641e89
baseElementYHexLittleEndian = 146adf9eb7641e89 9a54780d0e482cfb 30a84b2641e87244 208b1e9b9b11a98c

// We run in Sage, resExponentiation = baseElement ^ 5, and we get
baseElementTo5 = baseElement ^ 5
baseElementTo5 = 1919494216989370714282264091499504460829540920627494019318177103740489354093*i + 3944397571509712892671395330294281468555185483198747137614153093242854529958
baseElementTo5XHex             = 043e652d8f044857 c4cbfe9636928309 44288a2a00432390 7fa7e33a3e5acb6d
baseElementTo5XHexLittleEndian = 7fa7e33a3e5acb6d 44288a2a00432390 c4cbfe9636928309 043e652d8f044857

baseElementTo5YHex             = 08b8732d547b1cda b5c82ff0bfaa42c1 54e7b24b65223fc2 88b3e8a6de535ba6
baseElementTo5XHexLittleEndian = 88b3e8a6de535ba6 54e7b24b65223fc2 b5c82ff0bfaa42c1 08b8732d547b1cda
*/
func TestExp(t *testing.T) {
	// Case 1: Exponent = 5 (= 0x05)
	baseElementX := &gfP{0x34a8ddeafedf9bd9, 0x96c9dfdd6233cbac, 0xbbc9a8f5b577c5c5, 0x121ccc410d6339f7}
	baseElementY := &gfP{0x146adf9eb7641e89, 0x9a54780d0e482cfb, 0x30a84b2641e87244, 0x208b1e9b9b11a98c}
	// montEncode each Fp element
	// Important to do since the field arithmetic uses montgomery encoding in the library
	montEncode(baseElementX, baseElementX)
	montEncode(baseElementY, baseElementY)
	baseElement := &gfP2{*baseElementX, *baseElementY}

	// We keep the expected result non encoded
	// Will need to decode the obtained result to be able to assert it with this
	baseElementTo5X := &gfP{0x7fa7e33a3e5acb6d, 0x44288a2a00432390, 0xc4cbfe9636928309, 0x043e652d8f044857}
	baseElementTo5Y := &gfP{0x88b3e8a6de535ba6, 0x54e7b24b65223fc2, 0xb5c82ff0bfaa42c1, 0x08b8732d547b1cda}
	baseElementTo5 := &gfP2{*baseElementTo5X, *baseElementTo5Y}

	// Manual multiplication, to make sure the results are all coherent with each other
	manual := &gfP2{}
	manual = manual.Set(baseElement)
	manual = manual.Mul(manual, manual)      // manual ^ 2
	manual = manual.Mul(manual, manual)      // manual ^ 4
	manual = manual.Mul(manual, baseElement) // manual ^ 5
	manualDecoded := gfP2Decode(manual)

	// Expected result (obtained with sagemath, after some type conversions)
	w := &gfP2{}
	w = w.Set(baseElementTo5)

	// Result returned by the Exp function
	exponent5 := bigFromBase10("5")
	h := &gfP2{}
	h = h.Set(baseElement)
	h = h.Exp(exponent5)

	// We decode the result of the exponentiation to be able to compare with the
	// non-encoded/sagemath generated expected result
	hDecoded := gfP2Decode(h)

	assert.Equal(t, *w, *hDecoded, "The result of the exponentiation is not coherent with the Sagemath test vector")
	assert.Equal(t, *manualDecoded, *hDecoded, "The result of the exponentiation is not coherent with the manual repeated multiplication")

	// Case 2: Exponent = bigExponent = 39028236692093846773374607431768211455 + 2^128 - 2^64 = 379310603613032310218302470789826871295 = 0x11d5c90a20486bd1c40686b493777ffff
	// This exponent can be encoded on 3 words/uint64 => 0x1 0x1d5c90a20486bd1c 0x40686b493777ffff if 64bit machine or
	// on 5 words/uint32 => 0x1 0x1d5c90a2 0x0486bd1c 0x40686b49 0x3777ffff if 32bit machine
	baseElementX = &gfP{0x34a8ddeafedf9bd9, 0x96c9dfdd6233cbac, 0xbbc9a8f5b577c5c5, 0x121ccc410d6339f7}
	baseElementY = &gfP{0x146adf9eb7641e89, 0x9a54780d0e482cfb, 0x30a84b2641e87244, 0x208b1e9b9b11a98c}
	// montEncode each Fp element
	// Important to do since the field arithmetic uses montgomery encoding in the library
	montEncode(baseElementX, baseElementX)
	montEncode(baseElementY, baseElementY)
	baseElement = &gfP2{*baseElementX, *baseElementY}

	// We keep the expected result non encoded
	// Will need to decode the obtained result to be able to assert it with this
	// Sagemath:
	// baseElementToBigExp = baseElement ^ bigExponent
	// baseElementToBigExp => 7379142427977467878031119988604583496475317621776403696479934226513132928021*i + 17154720713365092794088637301427106756251681045968150072197181728711103784706
	// baseElementToBigExpXHex = 10507254ce787236 62cf3f84eb21adee 30ec827a799a519a 1464fc2ec9263c15
	// baseElementToBigExpYHex = 25ed3a53d558db9a 07da01cc9d10c5d5 ff7b1e4f41b874d7 debbc13409c8a702
	baseElementToBigExpX := &gfP{0x1464fc2ec9263c15, 0x30ec827a799a519a, 0x62cf3f84eb21adee, 0x10507254ce787236}
	baseElementToBigExpY := &gfP{0xdebbc13409c8a702, 0xff7b1e4f41b874d7, 0x07da01cc9d10c5d5, 0x25ed3a53d558db9a}
	baseElementToBigExp := &gfP2{*baseElementToBigExpX, *baseElementToBigExpY}

	// Expected result (obtained with sagemath, after some type conversions)
	w = &gfP2{}
	w = w.Set(baseElementToBigExp)

	// Result returned by the Exp function
	bigExp := bigFromBase10("379310603613032310218302470789826871295")
	h = &gfP2{}
	h = h.Set(baseElement)
	h = h.Exp(bigExp)

	// We decode the result of the exponentiation to be able to compare with the
	// non-encoded/sagemath generated expected result
	hDecoded = gfP2Decode(h)

	assert.Equal(t, *w, *hDecoded, "The result of the exponentiation is not coherent with the Sagemath test vector")
}

func TestSqrt(t *testing.T) {
	// Case 1: Valid QR
	// qr = 8192512702373747571754527085437364828369119615795326562285198594140975111129*i + 14719814144181528030533020377409648968040866053797156997322427920899698335369
	// This is a QR in Fp2
	qrXBig := bigFromBase10("8192512702373747571754527085437364828369119615795326562285198594140975111129")
	qrYBig := bigFromBase10("14719814144181528030533020377409648968040866053797156997322427920899698335369")
	qr := &gfP2{*newMontEncodedGFpFromBigInt(qrXBig), *newMontEncodedGFpFromBigInt(qrYBig)}
	res, err := qr.Sqrt()
	assert.NoError(t, err, "An error shouldn't be returned as we try to get the sqrt of a QR")
	// We decode the result of the squaring to compare the result with the Sagemath test vector
	// To get the sqrt of `r` in Sage, we run: `r.sqrt()`, and we get:
	// 838738240039331261565244756819667559640832302782323121523807597830118111128*i + 701115843855913009657260259360827182296091347204618857804078039211229345012
	resDecoded := gfP2Decode(res)
	expectedXBig := bigFromBase10("838738240039331261565244756819667559640832302782323121523807597830118111128")
	expectedYBig := bigFromBase10("701115843855913009657260259360827182296091347204618857804078039211229345012")
	expected := &gfP2{*newGFpFromBigInt(expectedXBig), *newGFpFromBigInt(expectedYBig)}

	assert.Equal(t, *expected, *resDecoded, "The result of the sqrt is not coherent with the Sagemath test vector")

	// Case 2: Valid QR
	// qr = -1 = 0 * i + 21888242871839275222246405745257275088696311157297823662689037894645226208582
	// The sqrt of qr is: sqrt = 21888242871839275222246405745257275088696311157297823662689037894645226208582 * i + 0
	qr = &gfP2{*newGFp(0), *newMontEncodedGFpFromBigInt(bigFromBase10("21888242871839275222246405745257275088696311157297823662689037894645226208582"))}
	res, err = qr.Sqrt()
	assert.NoError(t, err, "An error shouldn't be returned as we try to get the sqrt of a QR")

	resDecoded = gfP2Decode(res)
	expected = &gfP2{*newGFpFromBigInt(bigFromBase10("21888242871839275222246405745257275088696311157297823662689037894645226208582")), *newGFp(0)}
	assert.Equal(t, *expected, *resDecoded, "The result of the sqrt is not coherent with the Sagemath test vector")

	// Case 3: Get the sqrt of a QNR
	// qnr = 10142231111593789910248975994434553601587001629804098271704323146176084338608*i + 13558357083504759335548106329923635779485621365040524539176938811542516618464
	qnrXBig := bigFromBase10("10142231111593789910248975994434553601587001629804098271704323146176084338608")
	qnrYBig := bigFromBase10("13558357083504759335548106329923635779485621365040524539176938811542516618464")
	qnr := &gfP2{*newMontEncodedGFpFromBigInt(qnrXBig), *newMontEncodedGFpFromBigInt(qnrYBig)}
	res, err = qnr.Sqrt()
	assert.Error(t, err, "An error should have been returned as we try to get the sqrt of a QNR")
	assert.Nil(t, res, "The result of sqrt should be nil as we try to get the sqrt of a QNR")
}
