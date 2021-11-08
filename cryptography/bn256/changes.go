package bn256

// This file implement some util functions for the MPC
// especially the serialization and deserialization functions for points in G1
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

// B is constant of the curve
const B = 3

var p = P

// Compress G1 by dropping the Y (but retaining its most significant non-zero byte). It returns a [33]byte
func (e *G1) Compress() []byte {
	eb := e.Marshal()
	y := new(big.Int).SetBytes(eb[32:])

	// calculating the other possible solution of y²=x³+3
	y2 := new(big.Int).Sub(p, y)
	// if the specular solution is a bigger nr. we encode 0x00
	if y.Cmp(y2) < 0 {
		eb[32] = 0x00
	} else { // the specular solution is lower
		eb[32] = 0x01
	}

	//appending to X the information about which nr to pick for Y
	//if the smaller or the bigger
	return eb[0:33]
}

func marshal(xb []byte, yi *big.Int) *G1 {
	yb := yi.Bytes()
	paddingLength := 32 - len(yb)
	// instantiating the byte array representing G1
	g := make([]byte, 64)

	// copy X byte representation at the beginning of G1 reconstructed slice
	copy(g, xb)

	// do we need padding?
	if paddingLength > 0 {
		// create a padding byte slice for Y byte representation to be 32 bytes
		padding := make([]byte, paddingLength)
		// padding goes at the head of the Y array
		copy(g[32:32+paddingLength], padding)
	}

	// copy the Y byte representation to G1
	copy(g[32+paddingLength:], yb)

	// unmarshalling into a new instance of G1
	g1 := new(G1)
	g1.Unmarshal(g)
	return g1
}

// xToY reconstructs Y from X using the curve equation.
// it provides two solutions
func xToY(xb []byte) (*big.Int, *big.Int, bool) {
	xi := new(big.Int).SetBytes(xb[0:32])
	x3 := new(big.Int).Mul(xi, xi)
	x3.Mul(x3, xi)

	t := new(big.Int).Add(x3, big.NewInt(B))
	y1 := new(big.Int).ModSqrt(t, p)
	if y1 == nil {
		return nil, nil, false
	}

	y2 := new(big.Int).Sub(p, y1)
	return y1, y2, true
	// yp1, yp2, ok := cipolla(*t, *p)
	// return &yp1, &yp2, ok
}

// IsHigherY is used to distinguish between the 2 points of E
// that have the same x-coordinate
// The point e is assumed to be given in the affine form
func (e *G1) IsHigherY() bool {
	// Check nil pointers
	if e.p == nil {
		e.p = &curvePoint{}
	}

	var yCoord gfP
	//yCoord.Set(&e.p.y)
	yCoord = e.p.y

	var yCoordNeg gfP
	gfpNeg(&yCoordNeg, &yCoord)

	res := gfpCmp_p(&yCoord, &yCoordNeg)
	if res == 1 { // yCoord > yCoordNeg
		return true
	} else if res == -1 {
		return false
	}

	return false
}

// Takes a MontEncoded x and finds the corresponding y (one of the two possible y's)
func getYFromMontEncodedX(x *gfP) (*gfP, error) {
	// Check nil pointers
	if x == nil {
		return nil, errors.New("Cannot retrieve the y-coordinate form a nil pointer")
	}

	// Operations on montgomery encoded field elements
	x2 := &gfP{}
	gfpMul(x2, x, x)

	x3 := &gfP{}
	gfpMul(x3, x2, x)

	rhs := &gfP{}
	gfpAdd(rhs, x3, curveB) // curveB is MontEncoded, since it is create with newGFp

	// Montgomery decode rhs
	// Needed because when we create a GFp element
	// with gfP{}, then it is not montEncoded. However
	// if we create an element of GFp by using `newGFp()`
	// then this field element is Montgomery encoded
	// Above, we have been working on Montgomery encoded field elements
	// here we solve the quad. resid. over F (not encoded)
	// and then we encode back and return the encoded result
	//
	// Eg:
	// - Px := &gfP{1} => 0000000000000000000000000000000000000000000000000000000000000001
	// - PxNew := newGFp(1) => 0e0a77c19a07df2f666ea36f7879462c0a78eb28f5c70b3dd35d438dc58f0d9d
	montDecode(rhs, rhs)
	rhsBig, err := rhs.gFpToBigInt()
	if err != nil {
		return nil, err
	}

	// Note, if we use the ModSqrt method, we don't need the exponent, so we can comment these lines
	yCoord := big.NewInt(0)
	res := yCoord.ModSqrt(rhsBig, P)
	if res == nil {
		return nil, errors.New("not a square mod P")
	}

	yCoordGFp := newGFpFromBigInt(yCoord)
	montEncode(yCoordGFp, yCoordGFp)

	return yCoordGFp, nil
}

func padBytes(bb []byte) ([]byte, error) {
	if len(bb) > 32 {
		return []byte{}, errors.New("Cannot pad the given byte slice as the length exceed the padding length")
	}

	if len(bb) == 32 {
		return bb, nil
	}

	padSlice := make([]byte, 32)
	index := len(padSlice) - len(bb)
	copy(padSlice[index:], bb)
	return padSlice, nil
}

// Convert a big.Int into gfP
func newGFpFromBigInt(in *big.Int) (out *gfP) {
	// in >= P, so we mod it to get back in the field
	// (ie: we get the smallest representative of the equivalence class mod P)
	if res := in.Cmp(P); res >= 0 {
		// We need to mod P to get back into the field
		in.Mod(in, P)
	}

	inBytes := in.Bytes()
	// We want to work on byte slices of length 32 to re-assemble our GFpe element
	if len(inBytes) < 32 {
		// Safe to ignore the err as we are in the if so the condition is satisfied
		inBytes, _ = padBytes(inBytes)
	}

	out = &gfP{}
	var n uint64
	// Now we have the guarantee that inBytes has length 32 so it makes sense to run this for
	// loop safely (we won't exceed the boundaries of the container)
	for i := 0; i < 4; i++ {
		buf := bytes.NewBuffer(inBytes[i*8 : (i+1)*8])
		binary.Read(buf, binary.BigEndian, &n)
		out[(4-1)-i] = n // In gfP field elements are represented as little-endian 64-bit words
	}

	return out
}

// Convert a gfP into a big.Int
func (e *gfP) gFpToBigInt() (*big.Int, error) {
	str := e.String()

	out := new(big.Int)
	_, ok := out.SetString(str, 16)
	if !ok {
		return nil, errors.New("couldn't create big.Int from gfP element")
	}

	return out, nil
}

// Decompress unzip the Y coordinate using the curve. Y is always positive
// TODO: use native gfP representation instead of big.Int
func Decompress(xb []byte) (*G1, error) {
	if len(xb) != 33 {
		return nil, errors.New("bn256: not enough data on compressed point")
	}

	// TODO: we need to make sure that cipolla's results are always in the same order
	y1, y2, ok := xToY(xb)

	if !ok {
		return nil, errors.New("bn256: Cannot decompress")
	}

	smaller := y1.Cmp(y2) < 0
	if xb[32] == 0x00 && smaller {
		return marshal(xb, y1), nil
	}

	if xb[32] == 0x01 && smaller {
		return marshal(xb, y2), nil
	}

	if xb[32] == 0x00 {
		return marshal(xb, y2), nil
	}

	return marshal(xb, y1), nil
}

// Note: This function is only used to distinguish between points with the same x-coordinates
// when doing point compression.
// An ordered field must be infinite and we are working over a finite field here
func gfpCmp_p(a, b *gfP) int {
	for i := 4 - 1; i >= 0; i-- { // Remember that the gfP elements are written as little-endian 64-bit words
		if a[i] > b[i] { // As soon as we figure out that the MSByte of A > MSByte of B, we return
			return 1
		} else if a[i] == b[i] { // If the current bytes are equal we continue as we cannot conclude on A and B relation
			continue
		} else { // a[i] < b[i] so we can directly conclude and we return
			return -1
		}
	}

	return 0
}

// DecompressAmbiguous returns both solutions to the decompression function
func DecompressAmbiguous(xb []byte) (*G1, *G1, error) {
	if len(xb) != 33 {
		return nil, nil, errors.New("bn256: not enough data on compressed point")
	}

	y1, y2, ok := xToY(xb)

	if !ok {
		return nil, nil, errors.New("bn256: Cannot decompress")
	}

	if y1 == nil && y2 == nil {
		fmt.Printf("%v\n", new(big.Int).SetBytes(xb).String())
	}

	return marshal(xb, y1), marshal(xb, y2), nil
}

func (e *G1) EncodeCompressed() []byte {
	return e.Compress()
}

// returns to buffer rather than allocation from GC
func (e *G1) EncodeCompressedToBuf(ret []byte) {
	buf := e.Compress()
	copy(ret, buf)
	return
}

func (e *G1) DecodeCompressed(encoding []byte) error {
	if len(encoding) != 33 {
		return errors.New("wrong encoded point size")
	}
	//if encoding[0]&serializationCompressed == 0 { // Also test the length of the encoding to make sure it is 33bytes
	//	return errors.New("point isn't compressed")
	//}

	p, err := Decompress(encoding)
	if err != nil {
		return err
	}
	e.Set(p)
	return nil
}

func (e *G1) EncodeUncompressed() []byte {
	return e.Marshal()
}

func (e *G1) DecodeUncompressed(input []byte) error {
	_, err := e.Unmarshal(input)
	return err
}

var point0 = *newGFp(0)
var point1 = *newGFp(1)

// this will do batch inversions and thus optimize lookup table generation
// Montgomery Batch Inversion based trick
type G1Array []*G1

func (points G1Array) MakeAffine() {
	// point0 := *newGFp(0)
	// point1 := *newGFp(1)

	accum := newGFp(1)

	var scratch_backup [256]gfP

	var scratch []gfP
	if len(points) <= 256 {
		scratch = scratch_backup[:0] // avoid allocation is possible
	}
	for _, e := range points {
		if e.p == nil {
			e.p = &curvePoint{}
		}
		scratch = append(scratch, *accum)
		if e.p.z == point1 {
			continue
		} else if e.p.z == point0 { // return point at infinity if z = 0
			e.p.x = gfP{0}
			e.p.y = point1
			e.p.t = gfP{0}
			continue
		}

		gfpMul(accum, accum, &e.p.z) //  accum *= z

		/*
		   	    zInv := &gfP{}
		   	    zInv.Invert(&e.p.z)
		           fmt.Printf("%d inv %s\n",i, zInv)
		*/
	}

	zInv_accum := gfP{}
	zInv_accum.Invert(accum)

	tmp := gfP{}
	zInv := &gfP{}

	for i := len(points) - 1; i >= 0; i-- {
		e := points[i]

		if e.p.z == point1 {
			continue
		} else if e.p.z == point0 { // return point at infinity if z = 0
			continue
		}

		tmp = gfP{}
		gfpMul(&tmp, &zInv_accum, &e.p.z)
		gfpMul(zInv, &zInv_accum, &scratch[i])
		zInv_accum = tmp
		// fmt.Printf("%d inv %s\n",i, zInv)

		t, zInv2 := &gfP{}, &gfP{}
		gfpMul(t, &e.p.y, zInv)   // t = y/z
		gfpMul(zInv2, zInv, zInv) // zInv2 = 1/(z^2)

		gfpMul(&e.p.x, &e.p.x, zInv2) // x = x/(z^2)
		gfpMul(&e.p.y, t, zInv2)      // y = y/(z^3)

		e.p.z = point1
		e.p.t = point1
	}
}
