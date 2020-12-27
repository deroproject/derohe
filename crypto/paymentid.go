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

import "github.com/deroproject/derohe/crypto/bn256"

// BUG BUG BUG  this needs to be updated and add more context
// this function is used to encrypt/decrypt payment id
// as the operation is symmetric XOR, is the same in both direction
//
func EncryptDecryptPaymentID(blinder *bn256.G1, input []byte) (output []byte) {

	// input must be exactly 8 bytes long
	if len(input) != 8 {
		panic("Encrypted payment ID must be exactly 8 bytes long")
	}
	output = make([]byte, 8, 8)

	blinder_compressed := blinder.EncodeCompressed()
	if len(blinder_compressed) != 33 {
		panic("point compression needs to be fixed")
	}
	// Todo we should take the hash

	blinder_compressed = blinder_compressed[25:] // we will use last 8 bytes

	for l := range input {
		output[l] = input[l] ^ blinder_compressed[l] // xor the bytes with the hash
	}

	/*
		var tmp_buf [33]byte
		copy(tmp_buf[:], derivation[:]) // copy derivation key to buffer
		tmp_buf[32] = ENCRYPTED_PAYMENT_ID_TAIL

		// take hash
		hash := crypto.Keccak256(tmp_buf[:]) // take hash of entire 33 bytes, 32 bytes derivation key, 1 byte tail

		output = make([]byte, 8, 8)
		for i := range input {
			output[i] = input[i] ^ hash[i] // xor the bytes with the hash
		}
	*/
	return
}
