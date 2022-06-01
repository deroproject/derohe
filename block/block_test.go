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
import (
	"encoding/hex"
	"testing"

	"github.com/stratumfarm/derohe/config"
)

//import "github.com/stratumfarm/derohe/crypto"

func Test_Generic_block_serdes(t *testing.T) {
	var bl, bldecoded Block

	genesis_tx_bytes, _ := hex.DecodeString(config.Mainnet.Genesis_Tx)
	err := bl.Miner_TX.Deserialize(genesis_tx_bytes)

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
	err := bl.Miner_TX.Deserialize(genesis_tx_bytes)

	if err != nil {
		t.Errorf("Deserialization test failed for Genesis TX err %s\n", err)
	}

	Original_PoW := bl.GetHash()

	{
		temp_bl := bl
		temp_bl.Major_Version++
		if Original_PoW == temp_bl.GetHash() {
			t.Fatalf("POW Skipping Major Version")
		}
	}

	{
		temp_bl := bl
		temp_bl.Minor_Version++
		if Original_PoW == temp_bl.GetHash() {
			t.Fatalf("POW Skipping Minor Version")
		}
	}

	{
		temp_bl := bl
		temp_bl.Timestamp++
		if Original_PoW == temp_bl.GetHash() {
			t.Fatalf("POW Skipping Timestamp Version")
		}
	}

	{
		temp_bl := bl
		temp_bl.Miner_TX.Version++
		if Original_PoW == temp_bl.GetHash() {
			t.Fatalf("POW Skipping Miner_TX")
		}
	}

	{
		temp_bl := bl
		temp_bl.Tips = append(temp_bl.Tips, Original_PoW)
		if Original_PoW == temp_bl.GetHash() {
			t.Fatalf("POW Skipping Tips")
		}
	}

	{
		temp_bl := bl
		temp_bl.Tx_hashes = append(temp_bl.Tx_hashes, Original_PoW)
		if Original_PoW == temp_bl.GetHash() {
			t.Fatalf("POW Skipping TXs")
		}
	}

	{
		temp_bl := bl
		temp_bl.Proof[31] = 1
		if Original_PoW == temp_bl.GetHash() {
			t.Fatalf("POW Skipping Proof")
		}
	}

}

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
