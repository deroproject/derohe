// Copyright 2017-2018 DERO Project. All rights reserved.
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

package regpool

//import "fmt"
//import "bytes"
import (
	"encoding/hex"
	"testing"

	"github.com/deroproject/derohe/transaction"
)

// test the mempool interface with valid TX
func Test_regpool(t *testing.T) {

	// this tx is from random internal testnet, both tx are from same wallet
	tx_hex := "010000010ccf5f06ed0d8b66da41b3054438996fb57801e57b0809fec9816432715a1ae90004e22ceb7a312c7a5d1e19dd5eb6bec3ba182a77fdbd0004ac7ea2bece9cc8a00141663a9d5680f724ee9bfe4cf27e3a88e74986923e05f533d46643b052f397"

	//tx_hex2 := "010000018009d704feec7161952a952f306cd96023810c6788478a1c9fc50e7281ab7893ac02939da3bb500a6cf47bdc537f97b71a430acf832933459a6d2fbbc67cb2374909ceb166a4b5c582dec1a2b8629c073c949ffae201bb2c2562e8607eb1191003"

	var tx, dup_tx transaction.Transaction

	tx_raw, _ := hex.DecodeString(tx_hex)
	err := tx.Deserialize(tx_raw)
	dup_tx.Deserialize(tx_raw)

	if err != nil {
		t.Errorf("Tx Deserialisation failed")
	}

	pool, err := Init_Regpool(nil)

	if err != nil {
		t.Errorf("Pool initialization failed")
	}

	if len(pool.Regpool_List_TX()) != 0 {
		t.Errorf("Pool should be initialized in empty state")
	}

	if pool.Regpool_Add_TX(&tx, 0) != true {
		t.Errorf("Cannot Add transaction to pool in empty state")
	}

	if pool.Regpool_TX_Exist(tx.GetHash()) != true {
		t.Errorf("TX should already be in pool")
	}

	/*if len(pool.Mempool_List_TX()) != 1 {
		t.Errorf("Pool should  have 1 tx")
	}*/
	list_tx := pool.Regpool_List_TX()

	if len(list_tx) != 1 || list_tx[0] != tx.GetHash() {
		t.Errorf("Pool List tx failed")
	}

	get_tx := pool.Regpool_Get_TX(tx.GetHash())

	if tx.GetHash() != get_tx.GetHash() {
		t.Errorf("Pool get_tx failed")
	}

	// re-adding tx should faild
	if pool.Regpool_Add_TX(&tx, 0) == true || len(pool.Regpool_List_TX()) > 1 {
		t.Errorf("Pool should not allow duplicate TX")
	}

	// modify  tx and readd
	dup_tx.DestNetwork = 1 //modify tx so txid changes, still it should be rejected

	if tx.GetHash() == dup_tx.GetHash() {

		t.Errorf("tx and duplicate tx must have different hash")
	}

	if pool.Regpool_Add_TX(&dup_tx, 0) == true || len(pool.Regpool_List_TX()) > 1 {
		t.Errorf("Pool should not allow duplicate Key images")

	}

	// pool must have  1 key_image

	address_count := 0
	pool.address_map.Range(func(k, value interface{}) bool {
		address_count++
		return true
	})

	if address_count != 1 {
		t.Errorf("Pool doesnot have necessary key image")
	}

	if pool.Regpool_Delete_TX(dup_tx.GetHash()) != nil {
		t.Errorf("non existing TX cannot be deleted\n")
	}

	// pool must have  1 key_image
	address_count = 0
	pool.address_map.Range(func(k, value interface{}) bool {
		address_count++
		return true
	})
	if address_count != 1 {
		t.Errorf("Pool must have necessary key image")
	}

	// lets delete
	if pool.Regpool_Delete_TX(tx.GetHash()) == nil {
		t.Errorf("existing TX cannot be deleted\n")
	}

	address_count = 0
	pool.address_map.Range(func(k, value interface{}) bool {
		address_count++
		return true
	})
	if address_count != 0 {
		t.Errorf("Pool should not have any key image")
	}

	if len(pool.Regpool_List_TX()) != 0 {
		t.Errorf("Pool should  have 0 tx")
	}

}
