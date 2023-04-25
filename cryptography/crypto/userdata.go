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

import "math/big"
import "golang.org/x/crypto/chacha20"
import "github.com/deroproject/derohe/cryptography/bn256"

import "github.com/go-logr/logr"

var Logger logr.Logger = logr.Discard() // default discard all logs, someone needs to set this up

// this function is used to encrypt/decrypt payment id,srcid and other userdata
// as the operation is symmetric XOR, is the same in both direction
func EncryptDecryptUserData(key [32]byte, inputs ...[]byte) {
	var nonce [24]byte // nonce is 24 bytes, we will use xchacha20

	cipher, err := chacha20.NewUnauthenticatedCipher(key[:], nonce[:])
	if err != nil {
		panic(err)
	}

	for _, input := range inputs {
		cipher.XORKeyStream(input, input)
	}
	return
}

// does an ECDH and generates a shared secret
// https://en.wikipedia.org/wiki/Elliptic_curve_Diffie%E2%80%93Hellman
func GenerateSharedSecret(secret *big.Int, peer_publickey *bn256.G1) (shared_key [32]byte) {

	shared_point := new(bn256.G1).ScalarMult(peer_publickey, secret)
	compressed := shared_point.EncodeCompressed()
	if len(compressed) != 33 {
		panic("point compression needs to be fixed")
	}

	shared_key = Keccak256(compressed[:])

	return
}
