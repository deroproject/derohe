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

package blockchain

import (
	"encoding/hex"
	"fmt"

	"github.com/stratumfarm/derohe/block"
	"github.com/stratumfarm/derohe/cryptography/crypto"
	"github.com/stratumfarm/derohe/globals"
)

// generates a genesis block
func Generate_Genesis_Block() (bl block.Block) {

	genesis_tx_blob, err := hex.DecodeString(globals.Config.Genesis_Tx)
	if err != nil {
		panic("Failed to hex decode genesis Tx " + err.Error())
	}
	err = bl.Miner_TX.Deserialize(genesis_tx_blob)

	if err != nil {
		panic(fmt.Sprintf("Failed to parse genesis tx err %s hex %s ", err, globals.Config.Genesis_Tx))
	}

	if !bl.Miner_TX.IsPremine() {
		panic("miner tx not premine")
	}

	//rlog.Tracef(2, "Hash of Genesis Tx %x\n", bl.Miner_tx.GetHash())

	// verify whether tx is coinbase and valid

	// setup genesis block header
	bl.Major_Version = 1
	bl.Minor_Version = 1
	bl.Timestamp = 0 // first block timestamp

	var zerohash crypto.Hash
	_ = zerohash
	//bl.Tips = append(bl.Tips,zerohash)
	//bl.Prev_hash is automatic zero

	logger.V(1).Info("Hash of genesis block", "blid", bl.GetHash())

	serialized := bl.Serialize()

	var bl2 block.Block
	err = bl2.Deserialize(serialized)
	if err != nil {
		panic(fmt.Sprintf("error while serdes genesis block err %s", err))
	}
	if bl.GetHash() != bl2.GetHash() {
		panic("hash mismatch serdes genesis block")
	}

	return
}
