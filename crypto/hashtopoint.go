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
import "math/big"

//import "crypto/rand"
import "encoding/hex"

import "github.com/deroproject/derohe/crypto/bn256"

//import "golang.org/x/crypto/sha3"
import "github.com/deroproject/derohe/crypto/sha3"

// the original try and increment method A Note on Hashing to BN Curves https://www.normalesup.org/~tibouchi/papers/bnhash-scis.pdf
// see this for a simplified version https://github.com/clearmatics/mobius/blob/7ad988b816b18e22424728329fc2b166d973a120/contracts/bn256g1.sol

var FIELD_MODULUS, w = new(big.Int).SetString("30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd47", 16)
var GROUP_MODULUS, w1 = new(big.Int).SetString("30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001", 16)

// this file basically implements curve based items

type GeneratorParams struct {
	G    *bn256.G1
	H    *bn256.G1
	GSUM *bn256.G1

	Gs *PointVector
	Hs *PointVector
}

// converts a big int to 32 bytes, prepending zeroes
func ConvertBigIntToByte(x *big.Int) []byte {
	var dummy [128]byte
	joined := append(dummy[:], x.Bytes()...)
	return joined[len(joined)-32:]
}

// the number if already reduced
func HashtoNumber(input []byte) *big.Int {

	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(input)

	hash := hasher.Sum(nil)
	return new(big.Int).SetBytes(hash[:])
}

// calculate hash and reduce it by curve's order
func reducedhash(input []byte) *big.Int {
	return new(big.Int).Mod(HashtoNumber(input), bn256.Order)
}

func ReducedHash(input []byte) *big.Int {
	return new(big.Int).Mod(HashtoNumber(input), bn256.Order)
}

func makestring64(input string) string {
	for len(input) != 64 {
		input = "0" + input
	}
	return input
}

func makestring66(input string) string {
	for len(input) != 64 {
		input = "0" + input
	}
	return input + "00"
}

func hextobytes(input string) []byte {
	ibytes, err := hex.DecodeString(input)
	if err != nil {
		panic(err)
	}
	return ibytes
}

// p = p(u) = 36u^4 + 36u^3 + 24u^2 + 6u + 1
var FIELD_ORDER, _ = new(big.Int).SetString("30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd47", 16)

// Number of elements in the field (often called `q`)
// n = n(u) = 36u^4 + 36u^3 + 18u^2 + 6u + 1
var GEN_ORDER, _ = new(big.Int).SetString("30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001", 16)

var CURVE_B = new(big.Int).SetUint64(3)

// a = (p+1) / 4
var CURVE_A, _ = new(big.Int).SetString("c19139cb84c680a6e14116da060561765e05aa45a1c72a34f082305b61f3f52", 16)

func HashToPointNew(seed *big.Int) *bn256.G1 {
	y_squared := new(big.Int)
	one := new(big.Int).SetUint64(1)

	x := new(big.Int).Set(seed)
	x.Mod(x, GEN_ORDER)
	for {
		beta, y := findYforX(x)

		if y != nil {

			// fmt.Printf("beta %s y %s\n", beta.String(),y.String())

			// y^2 == beta
			y_squared.Mul(y, y)
			y_squared.Mod(y_squared, FIELD_ORDER)

			if beta.Cmp(y_squared) == 0 {

				//         fmt.Printf("liesoncurve test %+v\n", isOnCurve(x,y))
				//         fmt.Printf("x %s\n",x.Text(16))
				//         fmt.Printf("y %s\n",y.Text(16))

				xstring := x.Text(16)
				ystring := y.Text(16)

				var point bn256.G1
				xbytes, err := hex.DecodeString(makestring64(xstring))
				if err != nil {
					panic(err)
				}
				ybytes, err := hex.DecodeString(makestring64(ystring))
				if err != nil {
					panic(err)
				}

				if _, err := point.Unmarshal(append(xbytes, ybytes...)); err == nil {
					return &point
				} else {

					panic(fmt.Sprintf("not found err %s\n", err))
				}
			}
		}

		x.Add(x, one)
		x.Mod(x, FIELD_ORDER)
	}

}

/*
 * Given X, find Y
 *
 *   where y = sqrt(x^3 + b)
 *
 * Returns: (x^3 + b), y
**/
func findYforX(x *big.Int) (*big.Int, *big.Int) {
	// beta = (x^3 + b) % p
	xcube := new(big.Int).Exp(x, CURVE_B, FIELD_ORDER)
	xcube.Add(xcube, CURVE_B)
	beta := new(big.Int).Mod(xcube, FIELD_ORDER)
	//beta := addmod(mulmod(mulmod(x, x, FIELD_ORDER), x, FIELD_ORDER), CURVE_B, FIELD_ORDER);

	// y^2 = x^3 + b
	// this acts like: y = sqrt(beta)

	//ymod := new(big.Int).ModSqrt(beta,FIELD_ORDER) // this can return nil in some cases
	y := new(big.Int).Exp(beta, CURVE_A, FIELD_ORDER)
	return beta, y
}

/*
 * Verify if the X and Y coordinates represent a valid Point on the Curve
 *
 * Where the G1 curve is: x^2 = x^3 + b
**/
func isOnCurve(x, y *big.Int) bool {
	//p_squared := new(big.Int).Exp(x, new(big.Int).SetUint64(2), FIELD_ORDER);
	p_cubed := new(big.Int).Exp(x, new(big.Int).SetUint64(3), FIELD_ORDER)

	p_cubed.Add(p_cubed, CURVE_B)
	p_cubed.Mod(p_cubed, FIELD_ORDER)

	// return addmod(p_cubed, CURVE_B, FIELD_ORDER) == mulmod(p.Y, p.Y, FIELD_ORDER);
	return p_cubed.Cmp(new(big.Int).Exp(y, new(big.Int).SetUint64(2), FIELD_ORDER)) == 0
}

// this should be merged , simplified  just as simple as 25519
func HashToPoint(seed *big.Int) *bn256.G1 {
	/*
	    var x, _ = new(big.Int).SetString("0d36fdf1852f1563df9c904374055bb2a4d351571b853971764b9561ae203a9e",16)
	    var y, _ = new(big.Int).SetString("06efda2e606d7bafec34b82914953fa253d21ca3ced18db99c410e9057dccd50",16)



	   fmt.Printf("hardcode point on curve %+v\n", isOnCurve(x,y))
	   panic("done")
	*/
	return HashToPointNew(seed)

	seed_reduced := new(big.Int)
	seed_reduced.Mod(seed, FIELD_MODULUS)

	counter := 0

	p_1_4 := new(big.Int).Add(FIELD_MODULUS, new(big.Int).SetInt64(1))
	p_1_4 = p_1_4.Div(p_1_4, new(big.Int).SetInt64(4))

	for {
		tmp := new(big.Int)
		y, y_squared, y_resquare := new(big.Int), new(big.Int), new(big.Int) // basically y_sqaured = seed ^3 + 3 mod group order
		tmp.Exp(seed_reduced, new(big.Int).SetInt64(3), FIELD_MODULUS)
		y_squared.Add(tmp, new(big.Int).SetInt64(3))
		y_squared.Mod(y_squared, FIELD_MODULUS)

		y = y.Exp(y_squared, p_1_4, FIELD_MODULUS)

		y_resquare = y_resquare.Exp(y, new(big.Int).SetInt64(2), FIELD_MODULUS)

		if y_resquare.Cmp(y_squared) == 0 { // seed becomes x and y iis usy
			xstring := seed_reduced.Text(16)
			ystring := y.Text(16)

			var point bn256.G1
			xbytes, err := hex.DecodeString(makestring64(xstring))
			if err != nil {
				panic(err)
			}
			ybytes, err := hex.DecodeString(makestring64(ystring))
			if err != nil {
				panic(err)
			}

			if _, err = point.Unmarshal(append(xbytes, ybytes...)); err == nil {
				return &point
			} else {
				// continue finding
				counter++

				if counter%10000 == 0 {
					fmt.Printf("tried %d times\n", counter)
				}

			}

		}
		seed_reduced.Add(seed_reduced, new(big.Int).SetInt64(1))
		seed_reduced.Mod(seed_reduced, FIELD_MODULUS)
	}

	return nil
}
