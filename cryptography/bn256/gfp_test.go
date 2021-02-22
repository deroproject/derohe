package bn256

import (
	"testing"
)

// Tests that negation works the same way on both assembly-optimized and pure Go
// implementation.
func TestGFpNeg(t *testing.T) {
	n := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	w := &gfP{0xfedcba9876543211, 0x0123456789abcdef, 0x2152411021524110, 0x0114251201142512}
	h := &gfP{}

	gfpNeg(h, n)
	if *h != *w {
		t.Errorf("negation mismatch: have %#x, want %#x", *h, *w)
	}
}

// Tests that addition works the same way on both assembly-optimized and pure Go
// implementation.
func TestGFpAdd(t *testing.T) {
	a := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	b := &gfP{0xfedcba9876543210, 0x0123456789abcdef, 0xfeebdaedfeebdaed, 0xdeadbeefdeadbeef}
	w := &gfP{0xc3df73e9278302b8, 0x687e956e978e3572, 0x254954275c18417f, 0xad354b6afc67f9b4}
	h := &gfP{}

	gfpAdd(h, a, b)
	if *h != *w {
		t.Errorf("addition mismatch: have %#x, want %#x", *h, *w)
	}
}

// Tests that subtraction works the same way on both assembly-optimized and pure Go
// implementation.
func TestGFpSub(t *testing.T) {
	a := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	b := &gfP{0xfedcba9876543210, 0x0123456789abcdef, 0xfeebdaedfeebdaed, 0xdeadbeefdeadbeef}
	w := &gfP{0x02468acf13579bdf, 0xfdb97530eca86420, 0xdfc1e401dfc1e402, 0x203e1bfe203e1bfd}
	h := &gfP{}

	gfpSub(h, a, b)
	if *h != *w {
		t.Errorf("subtraction mismatch: have %#x, want %#x", *h, *w)
	}
}

// Tests that multiplication works the same way on both assembly-optimized and pure Go
// implementation.
func TestGFpMul(t *testing.T) {
	a := &gfP{0x0123456789abcdef, 0xfedcba9876543210, 0xdeadbeefdeadbeef, 0xfeebdaedfeebdaed}
	b := &gfP{0xfedcba9876543210, 0x0123456789abcdef, 0xfeebdaedfeebdaed, 0xdeadbeefdeadbeef}
	w := &gfP{0xcbcbd377f7ad22d3, 0x3b89ba5d849379bf, 0x87b61627bd38b6d2, 0xc44052a2a0e654b2}
	h := &gfP{}

	gfpMul(h, a, b)
	if *h != *w {
		t.Errorf("multiplication mismatch: have %#x, want %#x", *h, *w)
	}
}

// Tests the conversion from big.Int to GFp element
func TestNewGFpFromBigInt(t *testing.T) {
	// Case 1
	twoBig := bigFromBase10("2")
	h := *newGFpFromBigInt(twoBig)
	twoHex := [4]uint64{0x0000000000000002, 0x0000000000000000, 0x0000000000000000, 0x0000000000000000}
	w := gfP(twoHex)

	if h != w {
		t.Errorf("conversion mismatch: have %s, want %s", h.String(), w.String())
	}

	// Case 2
	pMinus1Big := bigFromBase10("21888242871839275222246405745257275088696311157297823662689037894645226208582")
	h = *newGFpFromBigInt(pMinus1Big)
	pMinus1Hex := [4]uint64{0x3c208c16d87cfd46, 0x97816a916871ca8d, 0xb85045b68181585d, 0x30644e72e131a029}
	w = gfP(pMinus1Hex)

	if h != w {
		t.Errorf("conversion mismatch: have %s, want %s", h.String(), w.String())
	}
}

// Tests the conversion from GFp element to big.Int
func TestGFpToBigInt(t *testing.T) {
	// Case 1
	twoHex := [4]uint64{0x0000000000000002, 0x0000000000000000, 0x0000000000000000, 0x0000000000000000}
	twoBig := bigFromBase10("2")
	twoGFp := gfP(twoHex) // Not MontEncoded!
	w := twoBig
	h, err := twoGFp.gFpToBigInt()

	if err != nil {
		t.Errorf("Couldn't convert GFp to big.Int: %s", err)
	}

	if r := h.Cmp(w); r != 0 {
		t.Errorf("conversion mismatch: have %s, want %s", h.String(), w.String())
	}

	// Case 2
	pMinus1Hex := [4]uint64{0x3c208c16d87cfd46, 0x97816a916871ca8d, 0xb85045b68181585d, 0x30644e72e131a029}
	pMinus1Big := bigFromBase10("21888242871839275222246405745257275088696311157297823662689037894645226208582")
	pMinus1GFp := gfP(pMinus1Hex) // Not MontEncoded!
	w = pMinus1Big
	h, err = pMinus1GFp.gFpToBigInt()

	if err != nil {
		t.Errorf("Couldn't convert GFp to big.Int: %s", err)
	}

	if r := h.Cmp(w); r != 0 {
		t.Errorf("conversion mismatch: have %s, want %s", h.String(), w.String())
	}
}
