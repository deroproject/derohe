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

import "os"
import "bytes"
import "path/filepath"
import "testing"

//import "fmt"

import "github.com/deroproject/derosuite/crypto"

// quick testing of wallet creation
func Test_Wallet_DB(t *testing.T) {

	temp_db := filepath.Join(os.TempDir(), "dero_temporary_test_wallet.db")

	os.Remove(temp_db)
	w, err := Create_Encrypted_Wallet(temp_db, "QWER", *crypto.RandomScalar())
	if err != nil {
		t.Fatalf("Cannot create encrypted wallet, err %s", err)
	}
	w.Close_Encrypted_Wallet()

	w, err = Open_Encrypted_Wallet(temp_db, "QWER")
	if err != nil {
		t.Fatalf("Cannot open encrypted wallet, err %s", err)
	}

	os.Remove(temp_db)

	//  test deterministc keys
	key := []byte("test")

	if !bytes.Equal(w.Key2Key(key), w.Key2Key(key)) {
		t.Fatalf("Key2Key failed")
	}

	w.Close_Encrypted_Wallet()

}
