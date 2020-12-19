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

package transaction

//import "fmt"
import "testing"
import "encoding/hex"

//import "github.com/deroproject/derohe/crypto"

// parse the tx and verify
func Test_Genesis_Tx(t *testing.T) {

	// mainnet genesis tx
	Genesis_Tx_hex := "" +
		"01" + // version
		"ff00" + // PREMINE_FLAG
		"a01f9bcc1208dee302769931ad378a4c0c4b2c21b0cfb3e752607e12d2b6fa6425" // miners public key

	tx_data_blob, _ := hex.DecodeString(Genesis_Tx_hex)

	var tx Transaction

	err := tx.DeserializeHeader(tx_data_blob)

	if err != nil {
		t.Errorf("Deserialize mainnet Genesis tx failed err %s\n", err)
		return
	}

	if tx.IsCoinbase() {
		t.Errorf("mainnet Genesis tx  cannot be coinbase\n")
	}

	if !tx.IsPremine() {
		t.Errorf("mainnet Genesis tx  must be Premine\n")
	}
}

/*
// test all error cases, except ring signature cases
func Test_Edge_Cases(t *testing.T) {

	tests := []struct {
		name  string
		txhex string
	}{
		{
			name:  "Invalid  Version",
			txhex: "80808080808080808080", // Major_Version is taking more than 9 bytes, trigger error
		},
		{
			name:  "Invalid Unlock time",
			txhex: "0280808080808080808080", // unlock time is taking more than 9 bytes, trigger error
		},

		{
			name:  "vin length cannot be zero",
			txhex: "020200", //  vin length cannot be zero, trigger error
		},

		{
			name:  "Invalid vin length",
			txhex: "020280808080808080808080", // invalid vin length is taking more than 9 bytes, trigger error
		},

		{
			name: "Miner TX vin is invalid",
			txhex: "02" + // version
				"3c" + // unlock time
				"01" + // vin length
				"00" + // vin #1
				"80808080808080808080" + // height gen input
				"01" + // vout length
				"ffffffffffff07" + // output #1 amount
				"02" + // output 1 type
				"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
				"21" + // extra length in bytes
				"01" + // extra pubkey tag
				"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
				"00", // RCT signature none
		},

		{
			name: "Miner TX height gen is invalid",
			txhex: "02" + // version
				"3c" + // unlock time
				"01" + // vin length
				"ff" + // vin #1
				"80808080808080808080" + // height gen input
				"01" + // vout length
				"ffffffffffff07" + // output #1 amount
				"02" + // output 1 type
				"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
				"21" + // extra length in bytes
				"01" + // extra pubkey tag
				"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
				"00", // RCT signature none
		},

		{
			name: "TX Vout length is invalid",
			txhex: "02" + // version
				"3c" + // unlock time
				"01" + // vin length
				"ff" + // vin #1
				"50" + // height gen input
				"80808080808080808080" + // vout length
				"ffffffffffff07" + // output #1 amount
				"02" + // output 1 type
				"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
				"21" + // extra length in bytes
				"01" + // extra pubkey tag
				"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
				"00", // RCT signature none
		},

		{
			name: "TX Vout amount is invalid",
			txhex: "02" + // version
				"3c" + // unlock time
				"01" + // vin length
				"ff" + // vin #1
				"50" + // height gen input
				"01" + // vout length
				"80808080808080808080" + // output #1 amount
				"02" + // output 1 type
				"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
				"21" + // extra length in bytes
				"01" + // extra pubkey tag
				"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
				"00", // RCT signature none
		},

		{
			name: "TX Vout type is invalid",
			txhex: "02" + // version
				"3c" + // unlock time
				"01" + // vin length
				"ff" + // vin #1
				"50" + // height gen input
				"00" + // vout length
				"80808080808080808080" + // output #1 amount
				"02" + // output 1 type
				"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
				"21" + // extra length in bytes
				"01" + // extra pubkey tag
				"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
				"00", // RCT signature none
		},

		{
			name: "TX Vout type not VOUT_TO_KEY",
			txhex: "02" + // version
				"3c" + // unlock time
				"01" + // vin length
				"ff" + // vin #1
				"00" + // height gen input
				"01" + // vout length
				"ffffffffffff07" + // output #1 amount
				"00" + // output 1 type
				"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
				"21" + // extra length in bytes
				"01" + // extra pubkey tag
				"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
				"00", // RCT signature none
		},

		{
			name: "TX Extra length cannot be invalid",
			txhex: "02" + // version
				"3c" + // unlock time
				"01" + // vin length
				"ff" + // vin #1
				"00" + // height gen input
				"01" + // vout length
				"ffffffffffff07" + // output #1 amount
				"02" + // output 1 type
				"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
				"80808080808080808080" + // extra length in bytes
				"01" + // extra pubkey tag
				"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
				"00", // RCT signature none
		},
	}

	for _, test := range tests {
		tx_raw, err := hex.DecodeString(test.txhex)
		if err != nil {
			t.Fatalf("Tx hex could not be hex decoded")
		}

		var tx Transaction
		err = tx.DeserializeHeader(tx_raw)

		if err == nil {
			t.Fatalf("%s failed", test.name)
		}

	}

}

// test panic if transaction is without ins
func Test_Edges_Serialization_0inputs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Transaction did not panic on 0 inputs")
		}
	}()

	// The following is the code under test
	var tx Transaction
	tx.Serialize()
}

// test panic if transaction is without  outs or unknown vouts type
func Test_Edges_Serialization_0outputs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Transaction did not panic on 0 outputs")
		}
	}()

	// The following is the code under test
	var tx Transaction
	tx.Vin = append(tx.Vin, Txin_gen{Height: 99}) // add input height
	tx.Serialize()
}

// test panic if transaction is without  unknown vouts type
func Test_Edges_Serialization_invalidoutputs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Transaction did not panic on invalid outputs")
		}
	}()

	// The following is the code under test
	var tx Transaction
	tx.Vin = append(tx.Vin, Txin_gen{Height: 99}) // add input height
	tx.Vout = append(tx.Vout, Tx_out{Amount: 0, Target: "NULL"})
	tx.Serialize()
}

// test panic if transaction is  invalid version
func Test_Edges_Invalid_version(t *testing.T) {

	// The following is the code under test
	// mainnet genesis tx
	Genesis_Tx_hex := "" +
		"02" + // version
		"3c" + // unlock time
		"01" + // vin length
		"ff" + // vin #1
		"00" + // height gen input
		"01" + // vout length
		"ffffffffffff07" + // output #1 amount
		"02" + // output 1 type
		"0bf6522f9152fa26cd1fc5c022b1a9e13dab697f3acf4b4d0ca6950a867a1943" + // output #1 key
		"21" + // extra length in bytes
		"01" + // extra pubkey tag
		"1d92826d0656958865a035264725799f39f6988faa97d532f972895de849496d" + // tx pubkey
		"00" // RCT signature none

	tx_data_blob, _ := hex.DecodeString(Genesis_Tx_hex)

	var tx Transaction

	err := tx.DeserializeHeader(tx_data_blob)
	if err != nil {
		t.Fatalf("Geneis Transaction could be deserialized")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Transaction did not panic on invalid version")
		}
	}()

	tx.Version = 3
	tx.GetHash() //panic since version is unknown
}

*/
