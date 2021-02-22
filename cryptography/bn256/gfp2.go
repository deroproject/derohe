package bn256

import (
	"errors"
	"math"
	"math/big"
)

// For details of the algorithms used, see "Multiplication and Squaring on
// Pairing-Friendly Fields, Devegili et al.
// http://eprint.iacr.org/2006/471.pdf.

// gfP2 implements a field of size p² as a quadratic extension of the base field
// where i²=-1.
type gfP2 struct {
	x, y gfP // value is xi+y.
}

func gfP2Decode(in *gfP2) *gfP2 {
	out := &gfP2{}
	montDecode(&out.x, &in.x)
	montDecode(&out.y, &in.y)
	return out
}

func (e *gfP2) String() string {
	return "(" + e.x.String() + ", " + e.y.String() + ")"
}

func (e *gfP2) Set(a *gfP2) *gfP2 {
	e.x.Set(&a.x)
	e.y.Set(&a.y)
	return e
}

func (e *gfP2) SetZero() *gfP2 {
	e.x = gfP{0}
	e.y = gfP{0}
	return e
}

func (e *gfP2) SetOne() *gfP2 {
	e.x = gfP{0}
	e.y = *newGFp(1)
	return e
}

func (e *gfP2) IsZero() bool {
	zero := gfP{0}
	return e.x == zero && e.y == zero
}

func (e *gfP2) IsOne() bool {
	zero, one := gfP{0}, *newGFp(1)
	return e.x == zero && e.y == one
}

func (e *gfP2) Conjugate(a *gfP2) *gfP2 {
	e.y.Set(&a.y)
	gfpNeg(&e.x, &a.x)
	return e
}

func (e *gfP2) Neg(a *gfP2) *gfP2 {
	gfpNeg(&e.x, &a.x)
	gfpNeg(&e.y, &a.y)
	return e
}

func (e *gfP2) Add(a, b *gfP2) *gfP2 {
	gfpAdd(&e.x, &a.x, &b.x)
	gfpAdd(&e.y, &a.y, &b.y)
	return e
}

func (e *gfP2) Sub(a, b *gfP2) *gfP2 {
	gfpSub(&e.x, &a.x, &b.x)
	gfpSub(&e.y, &a.y, &b.y)
	return e
}

// See "Multiplication and Squaring in Pairing-Friendly Fields",
// http://eprint.iacr.org/2006/471.pdf Section 3 "Schoolbook method"
func (e *gfP2) Mul(a, b *gfP2) *gfP2 {
	tx, t := &gfP{}, &gfP{}
	gfpMul(tx, &a.x, &b.y) // tx = a.x * b.y
	gfpMul(t, &b.x, &a.y)  // t = b.x * a.y
	gfpAdd(tx, tx, t)      // tx = a.x * b.y + b.x * a.y

	ty := &gfP{}
	gfpMul(ty, &a.y, &b.y) // ty = a.y * b.y
	gfpMul(t, &a.x, &b.x)  // t = a.x * b.x
	// We do a subtraction in the field since β = -1 in our case
	// In fact, Fp2 is built using the irreducible polynomial X^2 - β, where β = -1 = p-1
	gfpSub(ty, ty, t) // ty = a.y * b.y - a.x * b.x

	e.x.Set(tx) // e.x = a.x * b.y + b.x * a.y
	e.y.Set(ty) // e.y = a.y * b.y - a.x * b.x
	return e
}

func (e *gfP2) MulScalar(a *gfP2, b *gfP) *gfP2 {
	gfpMul(&e.x, &a.x, b)
	gfpMul(&e.y, &a.y, b)
	return e
}

// MulXi sets e=ξa where ξ=i+9 and then returns e.
func (e *gfP2) MulXi(a *gfP2) *gfP2 {
	// (xi+y)(i+9) = (9x+y)i+(9y-x)
	tx := &gfP{}
	gfpAdd(tx, &a.x, &a.x)
	gfpAdd(tx, tx, tx)
	gfpAdd(tx, tx, tx)
	gfpAdd(tx, tx, &a.x)

	gfpAdd(tx, tx, &a.y)

	ty := &gfP{}
	gfpAdd(ty, &a.y, &a.y)
	gfpAdd(ty, ty, ty)
	gfpAdd(ty, ty, ty)
	gfpAdd(ty, ty, &a.y)

	gfpSub(ty, ty, &a.x)

	e.x.Set(tx)
	e.y.Set(ty)
	return e
}

func (e *gfP2) Square(a *gfP2) *gfP2 {
	// Complex squaring algorithm:
	// (xi+y)² = (x+y)(y-x) + 2*i*x*y
	// - "Devegili OhEig Scott Dahab --- Multiplication and Squaring on Pairing-Friendly Fields.pdf"; Section 3 (Complex squaring)
	// - URL: https://eprint.iacr.org/2006/471.pdf
	// Here, since the non residue used is β = -1 in Fp, then we have:
	// c0 = (a0 + a1)(a0 + βa1) - v0 - βv0 => c0 = (a0 + a1)(a0 - a1)
	// c1 = 2v0, where v0 is a0 * a1 (= x * y, with our notations)
	tx, ty := &gfP{}, &gfP{}
	gfpSub(tx, &a.y, &a.x) // a.y - a.x
	gfpAdd(ty, &a.x, &a.y) // a.x + a.y
	gfpMul(ty, tx, ty)

	gfpMul(tx, &a.x, &a.y)
	gfpAdd(tx, tx, tx)

	e.x.Set(tx)
	e.y.Set(ty)
	return e
}

func (e *gfP2) Invert(a *gfP2) *gfP2 {
	// See "Implementing cryptographic pairings", M. Scott, section 3.2.
	// ftp://136.206.11.249/pub/crypto/pairings.pdf
	t1, t2 := &gfP{}, &gfP{}
	gfpMul(t1, &a.x, &a.x)
	gfpMul(t2, &a.y, &a.y)
	gfpAdd(t1, t1, t2)

	inv := &gfP{}
	inv.Invert(t1)

	gfpNeg(t1, &a.x)

	gfpMul(&e.x, t1, inv)
	gfpMul(&e.y, &a.y, inv)
	return e
}

// Exp is a function to exponentiate field elements
// This function navigates the big.Int binary representation
// from left to right (assumed to be in big endian)
// When going from left to right, each bit is checked, and when the first `1` bit is found
// the `foundOne` flag is set, and the "exponentiation begins"
//
// Eg: Let's assume that we want to exponentiate 3^5
// then the exponent is 5 = 0000 0101
// We navigate 0000 0101 from left to right until we reach 0000 0101
//                                                               ^
//                                                               |
// When this bit is reached, the flag `foundOne` is set, and and we do:
// res = res * 3 = 3
// Then, we move on to the left to read the next bit, and since `foundOne` is set (ie:
// the exponentiation has started), then we square the result, and do:
// res = res * res = 3*3 = 3^2
// The bit is `0`, so we continue
// Next bit is `1`, so we do: res = res * res = 3^2 * 3^2 = 3^4
// and because the bit is `1`, then, we do res = res * 3 = 3^4 * 3 = 3^5
// We reached the end of the bit string, so we can stop.
//
// The binary representation of the exponent is assumed to be binary big endian
//
// Careful, since `res` is initialized with SetOne() and since this function
// initializes the calling gfP2 to the one element of the Gfp2 which is montEncoded
// then, we need to make sure that the `e` element of gfP2 used to call the Exp function
// is also montEncoded (ie; both x and y are montEncoded)
/*
TODO: Refactor this function like this:
func (e *gfP2) Exp(a *gfP2, exponent *big.Int) *gfP2 {
	sum := (&gfP2{}).SetOne()
	t := &gfP2{}

	for i := exponent.BitLen() - 1; i >= 0; i-- {
		t.Square(sum)
		if exponent.Bit(i) != 0 {
			sum.Mul(t, a)
		} else {
			sum.Set(t)
		}
	}

	e.Set(sum)
	return e
}
*/
func (e *gfP2) Exp(exponent *big.Int) *gfP2 {
	res := &gfP2{}
	res = res.SetOne()

	base := &gfP2{}
	base = base.Set(e)

	foundOne := false
	exponentBytes := exponent.Bytes() // big endian bytes slice

	for i := 0; i < len(exponentBytes); i++ { // for each byte (remember the slice is big endian)
		for j := 0; j <= 7; j++ { // A byte contains the powers of 2 to 2^7 to 2^0 from left to right
			if foundOne {
				res = res.Mul(res, res)
			}

			if uint(exponentBytes[i])&uint(math.Pow(2, float64(7-j))) != uint(0) { // a byte contains the powers of 2 from 2^7 to 2^0 hence why we do 2^(7-j) (big-endian assumed)
				foundOne = true
				res = res.Mul(res, base)
			}
		}
	}

	e.Set(res)
	return e
}

// Sqrt returns the square root of e in GFp2
// See:
// - "A High-Speed Square Root Algorithm for Extension Fields - Especially for Fast Extension Fields"
// - URL: https://core.ac.uk/download/pdf/12530172.pdf
//
// - "Square Roots Modulo p"
// - URL: http://www.cmat.edu.uy/~tornaria/pub/Tornaria-2002.pdf
//
// - "Faster square roots in annoying finite fields"
// - URL: http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.21.9172
func (e *gfP2) Sqrt() (*gfP2, error) {
	// In GF(p^m), Euler's Criterion is defined like EC(x) = x^((p^m -1) / 2), if EC(x) == 1, x is a QR; if EC(x) == -1, x is a QNR
	// `euler` here, is the exponent used in Euler's criterion, thus, euler = (p^m -1) / 2 for GF(p^m)
	// here, we work over GF(p^2), so euler = (p^2 -1) / 2, where p = 21888242871839275222246405745257275088696311157297823662689037894645226208583
	////euler := bigFromBase10("239547588008311421220994022608339370399626158265550411218223901127035046843189118723920525909718935985594116157406550130918127817069793474323196511433944")

	// modulus^2 = 2^s * t + 1 => p^2 = 2^s * t + 1, where t is odd
	// In our case, p^2 = 2^s * t + 1, where s = 4, t = 29943448501038927652624252826042421299953269783193801402277987640879380855398639840490065738714866998199264519675818766364765977133724184290399563929243
	////t := bigFromBase10("29943448501038927652624252826042421299953269783193801402277987640879380855398639840490065738714866998199264519675818766364765977133724184290399563929243")
	////s := bigFromBase10("4")
	s := 4

	// tMinus1Over2 = (t-1) / 2
	tMinus1Over2 := bigFromBase10("14971724250519463826312126413021210649976634891596900701138993820439690427699319920245032869357433499099632259837909383182382988566862092145199781964621")

	// A non quadratic residue in Fp
	////nonResidueFp := bigFromBase10("21888242871839275222246405745257275088696311157297823662689037894645226208582")

	// A non quadratic residue in Fp2. Here nonResidueFp2 = i + 2 (Euler Criterion applied to this element of Fp2 shows that this element is a QNR)
	////nonResidueFp2 := &gfP2{*newGFp(1), *newGFp(2)}

	// nonResidueFp2 ^ t
	nonResidueFp2ToTXCoord := bigFromBase10("314498342015008975724433667930697407966947188435857772134235984660852259084")
	nonResidueFp2ToTYCoord := bigFromBase10("5033503716262624267312492558379982687175200734934877598599011485707452665730")
	nonResidueFp2ToT := &gfP2{*newMontEncodedGFpFromBigInt(nonResidueFp2ToTXCoord), *newMontEncodedGFpFromBigInt(nonResidueFp2ToTYCoord)}

	// Start algorithm
	// Initialize the algorithm variables
	v := s
	z := nonResidueFp2ToT
	w := new(gfP2).Set(e)
	w = w.Exp(tMinus1Over2)
	x := new(gfP2).Mul(e, w)
	b := new(gfP2).Mul(x, w) // contains e^t

	// Check if the element is a QR
	// Since p^2 = 2^s * t + 1 => t = (p^2 - 1)/2
	// Thus, since we have b = e^t, and since we want to test if e is a QR
	// we need to square b (s-1) times. That way we'd have
	// (e^t)^{2^(s-1)} which equals e^{(p^2 - 1)/2} => Euler criterion
	bCheck := new(gfP2).Set(b)
	for i := 0; i < s-1; i++ { // s-1 == 3 here (see comment above)
		bCheck = bCheck.Square(bCheck)
	}

	if !bCheck.IsOne() {
		return nil, errors.New("Cannot extract a root. The element is not a QR in Fp2")
	}

	// Extract the root of the quadratic residue using the Tonelli-Shanks algorithm
	for !b.IsOne() {
		m := 0
		b2m := new(gfP2).Set(b)
		for !b2m.IsOne() {
			/* invariant: b2m = b^(2^m) after entering this loop */
			b2m = b2m.Square(b2m)
			m++
		}

		j := v - m - 1
		w = z
		for j > 0 {
			w = w.Square(w)
			j--
		} // w = z^2^(v-m-1)

		z = new(gfP2).Square(w)
		b = b.Mul(b, z)
		x = x.Mul(x, w)
		v = m
	}

	return x, nil
}
