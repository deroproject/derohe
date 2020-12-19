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

package walletapi

import "testing"
import "crypto/sha256"

// functional test whether  the wrappers are okay
func Test_AEAD_Cipher(t *testing.T) {

	var key = sha256.Sum256([]byte("test"))
	var data = []byte("data")

	encrypted, err := EncryptWithKey(key[:], data)

	if err != nil {
		t.Fatalf("AEAD cipher failed err %s", err)
	}

	//t.Logf("Encrypted data %x %s", encrypted, string(encrypted))

	decrypted, err := DecryptWithKey(key[:], encrypted)
	if err != nil {
		t.Fatalf("AEAD cipher decryption failed, err %s", err)
	}

	if string(decrypted) != "data" {
		t.Fatalf("AEAD cipher encryption/decryption failed")
	}
}
