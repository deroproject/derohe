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
import (
	"math/big"

	"github.com/stratumfarm/derohe/cryptography/bn256"
) //import "crypto/rand"
//import "encoding/hex"

//import "golang.org/x/crypto/sha3"

type Polynomial struct {
	coefficients []*big.Int
}

func NewPolynomial(input []*big.Int) *Polynomial {
	if input == nil {
		return &Polynomial{coefficients: []*big.Int{new(big.Int).SetInt64(1)}}
	}
	return &Polynomial{coefficients: input}
}

func (p *Polynomial) Length() int {
	return len(p.coefficients)
}

func (p *Polynomial) Mul(m *Polynomial) *Polynomial {
	var product []*big.Int
	for i := range p.coefficients {
		product = append(product, new(big.Int).Mod(new(big.Int).Mul(p.coefficients[i], m.coefficients[0]), bn256.Order))
	}
	product = append(product, new(big.Int)) // add 0 element

	if m.coefficients[1].IsInt64() && m.coefficients[1].Int64() == 1 {
		for i := range product {
			if i > 0 {
				tmp := new(big.Int).Add(product[i], p.coefficients[i-1])

				product[i] = new(big.Int).Mod(tmp, bn256.Order)

			} else { // do nothing

			}
		}
	}
	return NewPolynomial(product)
}

type dummy struct {
	list [][]*big.Int
}

func RecursivePolynomials(list [][]*big.Int, accum *Polynomial, a, b []*big.Int) (rlist [][]*big.Int) {
	var d dummy
	d.recursivePolynomialsinternal(accum, a, b)

	return d.list
}

func (d *dummy) recursivePolynomialsinternal(accum *Polynomial, a, b []*big.Int) {
	if len(a) == 0 {
		d.list = append(d.list, accum.coefficients)
		return
	}

	atop := a[len(a)-1]
	btop := b[len(b)-1]

	left := NewPolynomial([]*big.Int{new(big.Int).Mod(new(big.Int).Neg(atop), bn256.Order), new(big.Int).Mod(new(big.Int).Sub(new(big.Int).SetInt64(1), btop), bn256.Order)})
	right := NewPolynomial([]*big.Int{atop, btop})

	d.recursivePolynomialsinternal(accum.Mul(left), a[:len(a)-1], b[:len(b)-1])
	d.recursivePolynomialsinternal(accum.Mul(right), a[:len(a)-1], b[:len(b)-1])
}
