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

package mempool

//import "fmt"
//import "bytes"
import "testing"
import "encoding/hex"

import "github.com/deroproject/derohe/transaction"

// test the mempool interface with valid TX
func Test_mempool(t *testing.T) {

	// this tx is from  internal testnet
	// tx_id 499002f3fb7fea8a71dac93dea65c0ff74be05b0858078a27a48d78b71eacf87
	tx_hex := "010000030101000000000000000000000000000000000000000000000000000000000000000000002aa937d8879cebc74425f4fee7bcc81b424dd6f2130d0548a0c4afa7f9e40b1c41ee8f9487971ee3ecf35cef40d26b6382b49b6315370f96696cda3394a550c4bd5d301a3b91737c354573ffa0d9e921a523af274b2ccc5d7078efd5c489c8deba43a0205aafd9f89b80cd432825f2f0bcbfa61f82bc30c2a83a7976e6743aad48fbdb8c7799cbf3131d156d2ac7b780c80202000d136ae832342f1ecf99a935e1dcdc255c7f33297b204383e7539f31278fd097001e261e85c34489c313aaa05ce4abb3772d31e5d192321dc2cce7fe45b21f4d114244b8fbf5a6cc1f0119b69ed785e7c8739c306b4a35278159bbb6977a28ea71ea0ffc5a6398ec935e000b8b5806e6e7a0bd8b57e24edd21cfafa79a54bf7d3bc74643294b88920b3b900113dad7529adb51a5b634140f01c8389c57c92af984d16cf4400a463a448fd7e7007ed6b651c46aab06f6f9677d50ee78cf98a3e121b325a0cab4bf67910f9a92d0153222d7f87cedc63a5b773a5c5482479e554d56d906eaf5d5f9b1c37d9f52170029bd3c397bcb89dca495d7ee8cab0e5e4cd8b204dff933e8673cacece063830d010676be97476c5028c633ca3160a7367352750304ee2be99bf6311c198b1207a20006364d038c0d479f5264c085c8f210ea88f8a7f08535240c3541c641aadae496002449b5477c26ff74be1159c36d71d9dbca3ab2c2a0804590637fb79e0d89e5d801053afa58bcdbf17aa0ce544e3ca12e9b5c204aaea6a0b5698fef75669c1d91690114acfe9439eff58fe532aa243187adadf7118ba78f9772429b3041a07cb203df0125ffa09b619faa1b890d0a5323a4f162b4d0d1a6864f9a0261820bfc901dea4501098ad5f3c3bb0629de269f3e01633b4e0ee6a8f74e648aa40f57dd2496ed7230010f100ede75074bafc160fc1327545a44fd8d34627b136fed6de0bfa72df0e1330116108dc5545f42b9f5b8e302d1546930d9840c18fc92e425bacd0856e842f60f011b7f689aa43b95bbfd02ee20aab2cac2077c9639b9e4f84b8b221b37cdaf13a2010b979e41274c4338395dfc447c0d4a548caf414ee425aab1748475cca9faef4e012051ff90139adccfbf0459e3a5c0638dfb1654ee728cba59c6803ab1daaddda9011ef927e921f956134892499c7513bc2e077986295c050633a1566d449fd0954a00071f56b24c65aaa4549be3c859ac8ec75fe302d89e8755166da3f7d0d08ae285011983fdc2aa2a0611187b0b86119508e3f7642dfe7380f5bdbe6ad27f0dc4ccf201127e3c3923518fec22a4ead80845bbe6008d9b790bd50ec6ef83b087d0490995011982ad7c49288a3115c638a04e14a0c74a75008359b40aff422e9972e276929800237a7bfd1fe105865abbc36e926de565530da393da1ddc0b1005bb0c31504402002569e7e95e54711903c29f63cb8c6b58d47ea3299c455c8738b543d552e8dffe012ea47467abedda0d168d06bcf2f45ab8fa090b90d6e4662d4052df0488084403057542fc7603a7d717dd8267dc036330e3083d2a1562bf5b1ecfbce99804c05100000000000000000000000000000000000000000000000000000000000000000e377bd9c8a2b6faf14c766e51fc9415ed979d604e3d5dcc800b3c5a9c6bec1f239a65bdeb2028926a6fa13fdf2caf0a09ba17e1420b74bef294aa4c9731d2c71f46ae2c57612f525f14cf49e86be5a984aab4d8c25ec3818e19612ba07722d500082c5b600665b3a718fad7516b976749ac3692a6ccf60225c04d1da272a95780012c932f36c1cca1753377ad16b2dbecabb58a2efbf8e88a066d80efba20addf8208d52649fc24ba86ecd87da10efd8ab480c48cadf4ee205e60d10f6e1f1527dd01171915999416a85d08a06e4a8ed18797fcd62910e6624222e120bdd60a00db22fb4d9f0a7aee7570f72b99c673195c67b7dc5e53aa5a32ffc04b409aa79cb52faa6cb8374cd242bd90fd2da9a891fc18022112ecb943b8b0f6c69a4b2e33ad0b16abbd22b38f11f1e76bfd6c1950864753ef84ce0ba869b82f909fbe2751ef0e02b170d107ec7cf6c25d9fdbdd86f5e4790e532254f89ad6cd477bf30816e207ce092de3148c4fd818812abfcadc03a325991fb7dd48b5a54266137709715a11556344ae9bbb2afd1e7ea7fa417f1308d20fdef8dd2168f89101d0643ce62217ee68724101b6e8644ad0860940f95b1674a1befa7ec45235d2c93043087e9001211d72b9f744364770a276f99a183dc184f90ad09d034119f5e4754249046bb201156984d6a418664bda88f98075679d6bb8cc4c644ff727278931797db12a2c65012698726eabb2580faecafa12befe8855a7074da2b4f2d2cd74fd481e818f2287001d445415f6abc3b4a9ff7043906d8f83d5c61ad853b12a72c152a21b9627d8bf00004d5a3deaf642e40e1401ead28babb7ce0263afcc3b3f1e7876a31d6fbea5e90125bff3189cf6ab0e68caa851e60fc92483d3bc2a5ecd511594e7f80f565301c101247b8928fb1866a98b99add332a7e6065cf27ed8a7d48d7775727299308f0ab9002cb7663076e2883fe31b3397f6f5460491835f1a23c173f858fb93cbc312cf360117c5b8e4a25c2e3228b6f4d06525f96e818772292b095b7d344eb3f9f18735bd000889c0fcc3c2f25fe74c915b178db653b0d7725efdc6e2e80a2770738cc3bff9002a578a39364fa16dc82fd7f15872da799f2ddc6422491b9e5c6447ea87adb01a00187461816522889fc2bcb1dc1b0b5d2537df3ae2bb58cbef2b58cb678ba6b9f30121f1cf89f1e9840aace8469b06edf81e9930ddfb8aef4a2c3d46e8551cc40ab400"

	var tx, dup_tx transaction.Transaction

	tx_raw, _ := hex.DecodeString(tx_hex)
	err := tx.Deserialize(tx_raw)
	dup_tx.Deserialize(tx_raw)

	if err != nil {
		t.Errorf("Tx Deserialisation failed")
	}

	pool, err := Init_Mempool(nil)

	if err != nil {
		t.Errorf("Pool initialization failed")
	}

	if len(pool.Mempool_List_TX()) != 0 {
		t.Errorf("Pool should be initialized in empty state")
	}

	if pool.Mempool_Add_TX(&tx, 0) != true {
		t.Errorf("Cannot Add transaction to pool in empty state")
	}

	if pool.Mempool_TX_Exist(tx.GetHash()) != true {
		t.Errorf("TX should already be in pool")
	}

	list_tx := pool.Mempool_List_TX()
	if len(list_tx) != 1 || list_tx[0] != tx.GetHash() {
		t.Errorf("Pool List tx failed")
	}

	get_tx := pool.Mempool_Get_TX(tx.GetHash())

	if tx.GetHash() != get_tx.GetHash() {
		t.Errorf("Pool get_tx failed")
	}

	// re-adding tx should faild
	if pool.Mempool_Add_TX(&tx, 0) == true || len(pool.Mempool_List_TX()) > 1 {
		t.Errorf("Pool should not allow duplicate TX")
	}

	// modify  tx and readd
	dup_tx.DestNetwork = 1 //modify tx so txid changes, still it should be rejected

	if tx.GetHash() == dup_tx.GetHash() {
		t.Errorf("tx and duplicate tx must have different hash")
	}

	if pool.Mempool_Add_TX(&dup_tx, 0) == true {
		t.Errorf("Pool should not allow duplicate Key images %d", len(pool.Mempool_List_TX()))
	}

	if len(pool.Mempool_List_TX()) != 1 {
		t.Errorf("Pool should have only 1 tx, actual %d", len(pool.Mempool_List_TX()))
	}

	// pool must have  1 key_image
	key_image_count := 0
	pool.nonces.Range(func(k, value interface{}) bool {
		key_image_count++
		return true
	})

	if key_image_count != 1 {
		t.Errorf("Pool doesnot have necessary key image")
	}

	if pool.Mempool_Delete_TX(dup_tx.GetHash()) != nil {
		t.Errorf("non existing TX cannot be deleted\n")
	}

	// pool must have  1 key_image
	key_image_count = 0
	pool.nonces.Range(func(k, value interface{}) bool {
		key_image_count++
		return true
	})
	if key_image_count != 1 {
		t.Errorf("Pool must have necessary key image")
	}

	// lets delete
	if pool.Mempool_Delete_TX(tx.GetHash()) == nil {
		t.Errorf("existing TX cannot be deleted\n")
	}

	key_image_count = 0
	pool.nonces.Range(func(k, value interface{}) bool {
		key_image_count++
		return true
	})
	if key_image_count != 0 {
		t.Errorf("Pool should not have any key image")
	}

	if len(pool.Mempool_List_TX()) != 0 {
		t.Errorf("Pool should  have 0 tx")
	}

}
