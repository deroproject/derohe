package bn256

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"math/big"
)

// FpUint64Size is the number of uint64 chunks to represent a field element
const FpUint64Size = 4

type gfP [FpUint64Size]uint64

func newGFp(x int64) (out *gfP) {
	if x >= 0 {
		out = &gfP{uint64(x)}
	} else {
		out = &gfP{uint64(-x)}
		gfpNeg(out, out)
	}

	montEncode(out, out)
	return out
}

func (e *gfP) String() string {
	return fmt.Sprintf("%16.16x%16.16x%16.16x%16.16x", e[3], e[2], e[1], e[0])
}

/*
func byteToUint64(in []byte) (uint64, error) {
	if len(in) > 8 {
		return 0, errors.New("the input bytes length should be equal to 8 (or smaller)")
	}

	// Takes the bytes in the little endian order
	// The byte 0x64 translate in a uint64 of the shape 0x64 (= 0x0000000000000064) rather than 0x6400000000000000
	res := binary.LittleEndian.Uint64(in)
	return res, nil
}
*/

// Makes sure that the
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
	for i := 0; i < FpUint64Size; i++ {
		buf := bytes.NewBuffer(inBytes[i*8 : (i+1)*8])
		binary.Read(buf, binary.BigEndian, &n)
		out[(FpUint64Size-1)-i] = n // In gfP field elements are represented as little-endian 64-bit words
	}

	return out
}

// Returns a new element of GFp montgomery encoded
func newMontEncodedGFpFromBigInt(in *big.Int) *gfP {
	res := newGFpFromBigInt(in)
	montEncode(res, res)

	return res
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

func (e *gfP) Set(f *gfP) {
	e[0] = f[0]
	e[1] = f[1]
	e[2] = f[2]
	e[3] = f[3]
}

func (e *gfP) Invert(f *gfP) {
	bits := [4]uint64{0x3c208c16d87cfd45, 0x97816a916871ca8d, 0xb85045b68181585d, 0x30644e72e131a029}

	sum, power := &gfP{}, &gfP{}
	sum.Set(rN1)
	power.Set(f)

	for word := 0; word < 4; word++ {
		for bit := uint(0); bit < 64; bit++ {
			if (bits[word]>>bit)&1 == 1 {
				gfpMul(sum, sum, power)
			}
			gfpMul(power, power, power)
		}
	}

	gfpMul(sum, sum, r3)
	e.Set(sum)
}

func (e *gfP) Marshal(out []byte) {
	for w := uint(0); w < 4; w++ {
		for b := uint(0); b < 8; b++ {
			out[8*w+b] = byte(e[3-w] >> (56 - 8*b))
		}
	}
}

func (e *gfP) Unmarshal(in []byte) error {
	// Unmarshal the bytes into little endian form
	for w := uint(0); w < 4; w++ {
		for b := uint(0); b < 8; b++ {
			e[3-w] += uint64(in[8*w+b]) << (56 - 8*b)
		}
	}
	// Ensure the point respects the curve modulus
	for i := 3; i >= 0; i-- {
		if e[i] < p2[i] {
			return nil
		}
		if e[i] > p2[i] {
			return errors.New("bn256: coordinate exceeds modulus")
		}
	}
	return errors.New("bn256: coordinate equals modulus")
}

// Note: This function is only used to distinguish between points with the same x-coordinates
// when doing point compression.
// An ordered field must be infinite and we are working over a finite field here
func gfpCmp(a, b *gfP) int {
	for i := FpUint64Size - 1; i >= 0; i-- { // Remember that the gfP elements are written as little-endian 64-bit words
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

// In Montgomery representation, an element x is represented by xR mod p, where
// R is a power of 2 corresponding to the number of machine-words that can contain p.
// (where p is the characteristic of the prime field we work over)
// See: https://web.wpi.edu/Pubs/ETD/Available/etd-0430102-120529/unrestricted/thesis.pdf
func montEncode(c, a *gfP) { gfpMul(c, a, r2) }
func montDecode(c, a *gfP) { gfpMul(c, a, &gfP{1}) }
