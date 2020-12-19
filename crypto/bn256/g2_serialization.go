// Package bn256 implements a particular bilinear group at the 128-bit security
// level.
//
// Bilinear groups are the basis of many of the new cryptographic protocols that
// have been proposed over the past decade. They consist of a triplet of groups
// (G₁, G₂ and GT) such that there exists a function e(g₁ˣ,g₂ʸ)=gTˣʸ (where gₓ
// is a generator of the respective group). That function is called a pairing
// function.
//
// This package specifically implements the Optimal Ate pairing over a 256-bit
// Barreto-Naehrig curve as described in
// http://cryptojedi.org/papers/dclxvi-20100714.pdf. Its output is compatible
// with the implementation described in that paper.
package bn256

import (
	"errors"
)

// This file implement some util functions for the MPC
// especially the serialization and deserialization functions for points in G1

// Constants related to the bn256 pairing friendly curve
const (
	Fq2ElementSize     = 2 * FqElementSize
	G2CompressedSize   = Fq2ElementSize + 1   // + 1 accounts for the additional byte used for masking
	G2UncompressedSize = 2*Fq2ElementSize + 1 // + 1 accounts for the additional byte used for masking
)

// EncodeUncompressed converts the compressed point e into bytes
// Take a point P in Jacobian form (where each coordinate is MontEncoded)
// and encodes it by going back to affine coordinates and montDecode all coordinates
// This function does not modify the point e
// (the variable `temp` is introduced to avoid to modify e)
func (e *G2) EncodeUncompressed() []byte {
	// Check nil pointers
	if e.p == nil {
		e.p = &twistPoint{}
	}

	// Set the right flags
	ret := make([]byte, G2UncompressedSize)
	if e.p.IsInfinity() {
		// Flag the encoding with the infinity flag
		ret[0] |= serializationInfinity
		return ret
	}

	// Marshal
	marshal := e.Marshal()
	// The encoding = flags || marshalledPoint
	copy(ret[1:], marshal)

	return ret
}

// DecodeUncompressed decodes a point in the uncompressed form
// Take a point P encoded (ie: written in affine form where each coordinate is MontDecoded)
// and encodes it by going back to Jacobian coordinates and montEncode all coordinates
func (e *G2) DecodeUncompressed(encoding []byte) error {
	if len(encoding) != G2UncompressedSize {
		return errors.New("wrong encoded point size")
	}
	if encoding[0]&serializationCompressed != 0 { // Also test the length of the encoding to make sure it is 65bytes
		return errors.New("point is compressed")
	}
	if encoding[0]&serializationBigY != 0 { // Also test that the bigY flag if not set
		return errors.New("bigY flag should not be set")
	}

	// Unmarshal the points and check their caps
	if e.p == nil {
		e.p = &twistPoint{}
	}

	// Removes the bits of the masking (This does a bitwise AND with `0001 1111`)
	// And thus removes the first 3 bits corresponding to the masking
	// Useless for now because in bn256, we added a full byte to enable masking
	// However, this is needed if we work over BLS12 and its underlying field
	bin := make([]byte, G2UncompressedSize)
	copy(bin, encoding)
	bin[0] &= serializationMask

	// Decode the point at infinity in the compressed form
	if encoding[0]&serializationInfinity != 0 {
		// Makes sense to check that all bytes of bin are 0x0 since we removed the masking above}
		for i := range bin {
			if bin[i] != 0 {
				return errors.New("invalid infinity encoding")
			}
		}
		e.p.SetInfinity()
		return nil
	}

	// We remove the flags and unmarshal the data
	_, err := e.Unmarshal(encoding[1:])
	return err
}

func (e *G2) IsHigherY() bool {
	// Check nil pointers
	if e.p == nil {
		e.p = &twistPoint{}
		e.p.MakeAffine()
	}

	// Note: the structures attributes are quite confusing here
	// In fact, each element of Fp2 is a polynomial with 2 terms
	// the `x` and `y` denote these coefficients, ie: xi + y
	// However, `x` and `y` are also used to denote the x and y **coordinates**
	// of an elliptic curve point. Hence, e.p.y represents the y-coordinate of the
	// point e, and e.p.y.y represents the **coefficient** y of the y-coordinate
	// of the elliptic curve point e.
	//
	// TODO: Rename the coefficients of the elements of Fp2 as c0 and c1 to clarify the code
	yCoordY := &gfP{}
	yCoordY.Set(&e.p.y.y)
	yCoordYNeg := &gfP{}
	gfpNeg(yCoordYNeg, yCoordY)

	res := gfpCmp(yCoordY, yCoordYNeg)
	if res == 1 { // yCoordY > yCoordNegY
		return true
	} else if res == -1 {
		return false
	}

	return false
}

func (e *G2) EncodeCompressed() []byte {
	// Check nil pointers
	if e.p == nil {
		e.p = &twistPoint{}
	}

	e.p.MakeAffine()
	ret := make([]byte, G2CompressedSize)

	// Flag the encoding with the compressed flag
	ret[0] |= serializationCompressed

	if e.p.IsInfinity() {
		// Flag the encoding with the infinity flag
		ret[0] |= serializationInfinity
		return ret
	}

	if e.IsHigherY() {
		// Flag the encoding with the bigY flag
		ret[0] |= serializationBigY
	}

	// We start the serialization of the coordinates at the index 1
	// Since the index 0 in the `ret` corresponds to the masking
	//
	// `temp` contains the the x-coordinate of the point
	// Thus, to fully encode `temp`, we need to Marshal it's x coefficient and y coefficient
	temp := gfP2Decode(&e.p.x)
	temp.x.Marshal(ret[1:])
	temp.y.Marshal(ret[FqElementSize+1:])

	return ret
}

// Takes a MontEncoded x and finds the corresponding y (one of the two possible y's)
func getYFromMontEncodedXG2(x *gfP2) (*gfP2, error) {
	// Check nil pointers
	if x == nil {
		return nil, errors.New("Cannot retrieve the y-coordinate from a nil pointer")
	}

	x2 := new(gfP2).Mul(x, x)
	x3 := new(gfP2).Mul(x2, x)
	rhs := new(gfP2).Add(x3, twistB) // twistB is MontEncoded, since it is create with newGFp

	yCoord, err := rhs.Sqrt()
	if err != nil {
		return nil, err
	}

	return yCoord, nil
}

// DecodeCompressed decodes a point in the compressed form
// Take a point P in G2 decoded (ie: written in affine form where each coordinate is MontDecoded)
// and encodes it by going back to Jacobian coordinates and montEncode all coordinates
func (e *G2) DecodeCompressed(encoding []byte) error {
	if len(encoding) != G2CompressedSize {
		return errors.New("wrong encoded point size")
	}
	if encoding[0]&serializationCompressed == 0 { // Also test the length of the encoding to make sure it is 33bytes
		return errors.New("point isn't compressed")
	}

	// Unmarshal the points and check their caps
	if e.p == nil {
		e.p = &twistPoint{}
	} else {
		e.p.x.SetZero()
		e.p.y.SetZero()
		e.p.z.SetOne()
		e.p.t.SetOne()
	}

	// Removes the bits of the masking (This does a bitwise AND with `0001 1111`)
	// And thus removes the first 3 bits corresponding to the masking
	bin := make([]byte, G2CompressedSize)
	copy(bin, encoding)
	bin[0] &= serializationMask

	// Decode the point at infinity in the compressed form
	if encoding[0]&serializationInfinity != 0 {
		if encoding[0]&serializationBigY != 0 {
			return errors.New("high Y bit improperly set")
		}

		// Similar to `for i:=0; i<len(bin); i++ {}`
		for i := range bin {
			// Makes sense to check that all bytes of bin are 0x0 since we removed the masking above
			if bin[i] != 0 {
				return errors.New("invalid infinity encoding")
			}
		}
		e.p.SetInfinity()
		return nil
	}

	// Decompress the point P (P =/= ∞)
	var err error
	if err = e.p.x.x.Unmarshal(bin[1:]); err != nil {
		return err
	}
	if err = e.p.x.y.Unmarshal(bin[FqElementSize+1:]); err != nil {
		return err
	}

	// MontEncode our field elements for fast finite field arithmetic
	// Needs to be done since the z and t coordinates are also encoded (ie: created with newGFp)
	montEncode(&e.p.x.x, &e.p.x.x)
	montEncode(&e.p.x.y, &e.p.x.y)

	y, err := getYFromMontEncodedXG2(&e.p.x)
	if err != nil {
		return err
	}
	e.p.y = *y

	// The flag serializationBigY is set (so the point pt with the higher Y is encoded)
	// but the point e retrieved from the `getYFromX` is NOT the higher, then we inverse
	if !e.IsHigherY() {
		if encoding[0]&serializationBigY != 0 {
			e.Neg(e)
		}
	} else {
		if encoding[0]&serializationBigY == 0 { // The point given by getYFromX is the higher but the mask is not set for higher y
			e.Neg(e)
		}
	}

	// No need to check that the point e.p is on the curve
	// since we retrieved y from x by using the curve equation.
	// Adding it would be redundant
	return nil
}
