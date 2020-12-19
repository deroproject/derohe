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

package block

//import "bytes"
import "testing"
import "encoding/hex"

import "github.com/deroproject/derohe/config"

//import "github.com/deroproject/derohe/crypto"

func Test_Generic_block_serdes(t *testing.T) {
	var bl, bldecoded Block

	genesis_tx_bytes, _ := hex.DecodeString(config.Mainnet.Genesis_Tx)
	err := bl.Miner_TX.DeserializeHeader(genesis_tx_bytes)

	if err != nil {
		t.Errorf("Deserialization test failed for Genesis TX err %s\n", err)
	}
	serialized := bl.Serialize()

	err = bldecoded.Deserialize(serialized)

	if err != nil {
		t.Errorf("Deserialization test failed for NULL block err %s\n", err)
	}

}

// this tests whether the PoW depends on everything in the BLOCK except Proof
func Test_PoW_Dependancy(t *testing.T) {
	var bl Block
	genesis_tx_bytes, _ := hex.DecodeString(config.Mainnet.Genesis_Tx)
	err := bl.Miner_TX.DeserializeHeader(genesis_tx_bytes)

	if err != nil {
		t.Errorf("Deserialization test failed for Genesis TX err %s\n", err)
	}

	Original_PoW := bl.GetPoWHash()

	{
		temp_bl := bl
		temp_bl.Major_Version++
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping Major Version")
		}
	}

	{
		temp_bl := bl
		temp_bl.Minor_Version++
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping Minor Version")
		}
	}

	{
		temp_bl := bl
		temp_bl.Timestamp++
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping Timestamp Version")
		}
	}

	{
		temp_bl := bl
		temp_bl.Miner_TX.Version++
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping Miner_TX")
		}
	}

	{
		temp_bl := bl
		temp_bl.Nonce++
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping Nonce")
		}
	}

	{
		temp_bl := bl
		temp_bl.ExtraNonce[31] = 1
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping Extra Nonce")
		}
	}

	{
		temp_bl := bl
		temp_bl.Tips = append(temp_bl.Tips, Original_PoW)
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping Tips")
		}
	}

	{
		temp_bl := bl
		temp_bl.Tx_hashes = append(temp_bl.Tx_hashes, Original_PoW)
		if Original_PoW == temp_bl.GetPoWHash() {
			t.Fatalf("POW Skipping TXs")
		}
	}

	{
		temp_bl := bl
		temp_bl.Proof[31] = 1
		if Original_PoW != temp_bl.GetPoWHash() {
			t.Fatalf("POW Including Proof")
		}
	}

}

/*
func Test_testnet_Genesis_block_serdes(t *testing.T) {

	testnet_genesis_block_hex := "010100112700000000000000000000000000000000000000000000000000000000000000000000023c01ff0001ffffffffffff07020bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a194321011d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d0000000000000000000000000000000000000000000000000000000000000000000000"

	testnet_genesis_block, _ := hex.DecodeString(testnet_genesis_block_hex)

	var bl Block
	err := bl.Deserialize(testnet_genesis_block)

	if err != nil {
		t.Errorf("Deserialization test failed for NULL block err %s\n", err)
	}

	// test the block serializer and deserializer whether it gives the same
	serialized := bl.Serialize()

	if !bytes.Equal(serialized, testnet_genesis_block) {
		t.Errorf("Serialization test failed for Genesis block %X\n", serialized)
	}

	// test block id
	if bl.GetHash() != config.Testnet.Genesis_Block_Hash {
		t.Error("genesis block ID failed \n")
	}

	hash := bl.GetHash()
	bl.SetExtraNonce(hash[:])
	for i := range hash {
		if hash[i] != bl.ExtraNonce[i] {
			t.Fatalf("Extra nonce test failed")
		}
	}
	if bl.SetExtraNonce(hash[:0]) == nil { // this should fail
		t.Fatalf("Extra nonce test failed")
	}
	if bl.SetExtraNonce(append([]byte{0}, hash[:]...)) != nil { // this should pass
		t.Fatalf("Extra nonce test failed")
	}

	bl.ClearExtraNonce()
	for i := range hash {
		if 0 != bl.ExtraNonce[i] {
			t.Fatalf("Extra nonce  clearing test failed")
		}
	}

	bl.Nonce = 99
	bl.ClearNonce()
	if bl.Nonce != 0 {
		t.Fatalf("Nonce clearing failed")
	}

	bl.Deserialize(testnet_genesis_block)
	block_work := bl.GetBlockWork()
	bl.SetExtraNonce(hash[:])
	bl.Nonce = 99
	bl.CopyNonceFromBlockWork(block_work)
	if bl.GetHash() != config.Testnet.Genesis_Block_Hash {
		t.Fatalf("Copynonce failed")
	}

	if nil == bl.CopyNonceFromBlockWork(hash[:]) { // this should give an error
		t.Fatalf("Copynonce test failed")
	}

	//if bl.GetReward() != 35184372088831 {
	//	t.Error("genesis block reward failed \n")
    //}

}
*/

/*
func Test_Genesis_block_serdes(t *testing.T) {

	mainnet_genesis_block_hex := "010000000000000000000000000000000000000000000000000000000000000000000010270000023c01ff0001ffffffffffff07020bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a194321011d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d0000"

	mainnet_genesis_block, _ := hex.DecodeString(mainnet_genesis_block_hex)

	var bl Block
	err := bl.Deserialize(mainnet_genesis_block)

	if err != nil {
		t.Errorf("Deserialization test failed for NULL block err %s\n", err)
	}

	// test the block serializer and deserializer whether it gives the same
	serialized := bl.Serialize()

	if !bytes.Equal(serialized, mainnet_genesis_block) {
		t.Errorf("Serialization test failed for Genesis block %X\n", serialized)
	}

	// calculate POW hash
	powhash := bl.GetPoWHash()
	if powhash != crypto.Hash([32]byte{0xa7, 0x3b, 0xd3, 0x7a, 0xba, 0x34, 0x54, 0x77, 0x6b, 0x40, 0x73, 0x38, 0x54, 0xa8, 0x34, 0x9f, 0xe6, 0x35, 0x9e, 0xb2, 0xc9, 0x1d, 0x93, 0xbc, 0x72, 0x7c, 0x69, 0x43, 0x1c, 0x1d, 0x1f, 0x95}) {
		t.Errorf("genesis block POW failed %x\n", powhash[:])
	}

	// test block id
	if bl.GetHash() != config.Mainnet.Genesis_Block_Hash {
		t.Error("genesis block ID failed \n")
	}

	if bl.GetReward() != 35184372088831 {
		t.Error("genesis block reward failed \n")

	}

}

func Test_Block_38373_serdes(t *testing.T) {

	block_hex := "0606f0cac5d405b325cd7b2cb9f7d9500f37b5faf8dacd1506a73a6261b476d1f8aea4d59e54d93989000002a1ac0201ffe5ab0201e6fcee8183d1060288195982ed85017ba561f276f17986c54be81001057d3d696be5ec49d99648192b010877b53197c749557b97aad154d4a85dff4498158ec8e16cb9034562676b091d0208000000009699cce20001d0e1a493c61ba77865f17b27223474bf93115267a596258cb291fbc18ac9cd20"

	block, _ := hex.DecodeString(block_hex)

	var bl Block
	err := bl.Deserialize(block)

	if err != nil {
		t.Errorf("Deserialization test failed for NULL block err %s\n", err)
	}

	// test the block serializer and deserializer whether it gives the same
	serialized := bl.Serialize()

	if !bytes.Equal(serialized, block) {
		t.Errorf("Serialization test failed for block %X\n", serialized)
	}

	// test block hash
	if bl.GetHash().String() != "02727780cade8a026c01dc0e0b9a908bf6f82ca1fe3ca61f83377a276c42c8b1" {
		t.Errorf("block hash failed \n")
	}

	powhash := bl.GetPoWHash()
	if powhash != crypto.HashHexToHash("e918f3452df59edaeed6dfec1524adc4a191498e9aa02a709a20f97303000000") {
		t.Errorf("block POW failed %x\n", powhash[:])
	}

}

func Test_Block_38374_serdes(t *testing.T) {

	block_hex := "0606b7ccc5d40502727780cade8a026c01dc0e0b9a908bf6f82ca1fe3ca61f83377a276c42c8b11700800002a2ac0201ffe6ab0201f0e588d08edc06021a61261e226bad3dace02ce380c8da5abde1567cff8bb78069dae79e3a778ac72b01b62fda03efb8860fb6dd270b177f2e7a56ef7f9dd35a4af8852512653b191136020800000000e899cce2001734a9ff779afd4d1fce7a30815402fd7f7ce694be95ed69ff42e498e40af25c5511b52aff7b16df5e0712a3a5b59913f59658cb7e44201b1a78631bd87d7c3e1dc66c9fc7d5260f6f0fa99914f7a1f35d3ddb3e2a04af516f8135f6f607acfe5314f2c9c0dbe58bc981527ff90fa236b2663ee6d295ff1b5ad4add1c30cb0393d5e287c7ca687f04701485174bd0c7b2ac4a1b8982dd6e1ef8df569f7bf03668d12c99b329118c00d30e341808e94ff8ec31374104b2a37b785def153216d8bab52c3bd48d408e6d96e344344b243a3911e058909e1f26aac10482a4af1fcc86d23116a483e45d705256261ee6233ebc9b4f8993580ed2d9d7c598ae58a445134af1a325d26222418f518df2c8997c29f4495237ba6271fb2d517dd6f66c9c1ced406e5823594deb5f3952a9e80eae87ce2df8a8290cc1c3f12e474bfa38ab12845088a1f543790d8a7b7cf77e757e5299d28d7e206520b259bc55a63c3a8c987c6215f6fb186f6d87ccf299965de42a004ca38aa0d4dd16144adf2d7ce31fa74bc3bce1708e03ed2396b678c85d8d3f4d7ee76f5c13b53312c80a4240d9e6495508159aeab1f330bc331e3b81310f5ba749063677c62ee5bde87129b15241ee895e0437a808b9b03c77b86b8b641dc6bfbc70206415c2b2e497a6bc0e4dcb2ea24b75f1caec50cd2ab6426e91f41f11545d02c01f0530f23d667c4e04f16989b11ee6ade7b7a210e744fdbff45aab0e09bc718e847a5acd68c6da306e0ae9c9c5a97eae96a11968dfcdd414f8f4957ea45fd21f49fef889b86d3298224c3a2d21c4ccc9ff0fff6f04cb4a3998e5cc935afbfbfc79af766227a60a32275ad8480448b06fe78cbdaaa03acab10a6265154bf92bcec87a055e770f8c69581319a5db766b16050ddf8b448d6c784d1ec48072c702c41a864e5965c12eb450c36466f481a577fbeef6d89e8cfcdaea42e3c0dc8066b4681868b57270917c5b192d3a1457fb56bd85f2a58af0979dc1b1e6279c08e2a5013cb5643d21b17495d778dd8"

	block, _ := hex.DecodeString(block_hex)

	var bl, bl2 Block
	err := bl.Deserialize(block)

	if err != nil {
		t.Errorf("Deserialization test failed for NULL block err %s\n", err)
	}

	// test the block serializer and deserializer whether it gives the same
	serialized := bl.Serialize()

	if !bytes.Equal(serialized, block) {
		t.Errorf("Serialization test failed for block %X\n", serialized)
	}

	// test block hash
	if bl.GetHash().String() != "d76d83e03c1d5d223c666c2cbcaa781fb74e53f8eb183a927aba81f44108bf13" {
		t.Errorf("block hash failed \n")
	}

	powhash := bl.GetPoWHash()
	if powhash != crypto.HashHexToHash("7457a3d344b4c3bb57f505b79c8c915ab0364657f9577a858137f39d02000000") {
		t.Errorf(" block POW failed %x\n", powhash[:])
	}

	err = bl2.DeserializeHeader(block)

	if bl.Major_Version != bl2.Major_Version ||
		bl.Minor_Version != bl2.Minor_Version ||
		bl.Timestamp != bl2.Timestamp ||
		bl.Prev_Hash.String() != bl2.Prev_Hash.String() ||
		bl.Nonce != bl2.Nonce {
		t.Errorf(" block Deserialize header failed %x\n", powhash[:])

	}

}

func Test_Treehash_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Treehash did not panic on 0 inputs")
		}
	}()

	// The following is the code under test
	var hashes []crypto.Hash

	TreeHash(hashes)
}
*/

// test all invalid edge cases, which will return error
func Test_Block_Edge_Cases(t *testing.T) {
	tests := []struct {
		name     string
		blockhex string
	}{
		{
			name:     "Invalid Major Version",
			blockhex: "80808080808080808080", // Major_Version is taking more than 9 bytes, trigger error
		},
		{
			name:     "Invalid Minor Version",
			blockhex: "0280808080808080808080", // Mijor_Version is taking more than 9 bytes, trigger error
		},

		{
			name:     "Invalid timestamp",
			blockhex: "020280808080808080808080", // timestamp is taking more than 9 bytes, trigger error
		},

		{
			name:     "Incomplete header",
			blockhex: "020255", // prev hash is not provided, controlled panic
		},
	}

	for _, test := range tests {
		block, err := hex.DecodeString(test.blockhex)
		if err != nil {
			t.Fatalf("Block hex could not be hex decoded")
		}

		//t.Logf("%s failed", test.name)
		var bl Block
		err = bl.Deserialize(block)

		if err == nil {
			t.Fatalf("%s failed", test.name)
		}

	}
}

/*
// this edge case occurred in monero and affected all CryptoNote coins
// bug occured when > 512 transactions were present, causing monero network to split and halt
// test case from monero block 202612  bbd604d2ba11ba27935e006ed39c9bfdd99b76bf4a50654bc1e1e61217962698
// the test is empty because we do NOT support v1 transactions
// however, this test needs to be added for future attacks
func Test_Treehash_Egde_Case(t *testing.T) {

}
*/
