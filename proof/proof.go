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

package proof

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
)

//import "github.com/deroproject/derosuite/walletapi" // to decode encrypted payment ID

// this function will prove detect and decode output amount for the tx
func Prove(proof string, input_tx string, ring_string [][]string, mainnet bool) (receivers []string, amounts []uint64, payload_raw [][]byte, payload_decoded []string, err error) {
	var tx transaction.Transaction

	addr, err := rpc.NewAddress(strings.TrimSpace(proof))
	if err != nil {
		return
	}

	if !addr.IsDERONetwork() || !addr.IsIntegratedAddress() || !addr.Proof {
		err = fmt.Errorf("Invalid proof ")
		return
	}

	args := addr.Arguments

	amount := uint64(0)
	var shared_key crypto.Hash

	if args.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) { // this service is expecting value to be specfic
		amount = args.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
		if err = ValidatePayloadProofAmount(amount); err != nil {
			err = fmt.Errorf("invalid proof amount: %v", err)
			return
		}
	} else {
		err = fmt.Errorf("Invalid proof.")
		return
	}

	if args.Has(rpc.DataHash, rpc.DataHash) { // this service is expecting value to be specfic
		shared_key = args.Value(rpc.DataHash, rpc.DataHash).(crypto.Hash)
	} else {
		err = fmt.Errorf("Invalid proof. missing key part")
		return
	}

	tx_hex, err := hex.DecodeString(input_tx)
	if err != nil {
		return
	}

	err = tx.Deserialize(tx_hex)
	if err != nil {
		return
	}

	// now lets decode the ring
	for i := range ring_string {
		for j := range ring_string[i] {
			var addr *rpc.Address
			if addr, err = rpc.NewAddress(ring_string[i][j]); err != nil {
				return
			}
			tx.Payloads[i].Statement.Publickeylist = append(tx.Payloads[i].Statement.Publickeylist, (*bn256.G1)(addr.PublicKey))
		}
	}

	// okay all inputs have been parsed
	var x bn256.G1
	x.ScalarMult(crypto.G, new(big.Int).SetUint64(amount))
	x.Add(new(bn256.G1).Set(&x), addr.PublicKey.G1())

	for t := range tx.Payloads {
		for k := range tx.Payloads[t].Statement.C {

			if x.String() == tx.Payloads[t].Statement.C[k].String() {

				astring := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[k]))
				astring.Mainnet = mainnet

				receivers = append(receivers, astring.String())
				amounts = append(amounts, amount)

				payload := tx.Payloads[t].RPCPayload
				switch tx.Payloads[t].RPCType {
				case transaction.ENCRYPTED_DEFAULT_PAYLOAD_CBOR:
					crypto.EncryptDecryptUserData(crypto.Keccak256(shared_key[:], tx.Payloads[t].Statement.Publickeylist[k].EncodeCompressed()), payload)
				case transaction.ENCRYPTED_DEFAULT_PAYLOAD_CBOR_V2:
					payload = payload[33:]
					crypto.EncryptDecryptUserData(crypto.Keccak256(shared_key[:], tx.Payloads[t].Statement.Publickeylist[k].EncodeCompressed()), payload)
				}

				// skip first byte as it is not guaranteed, even rest of the bytes are not

				payload_raw = append(payload_raw, payload[1:])
				var args rpc.Arguments
				if unmarshal_err := args.UnmarshalBinary(payload[1:]); unmarshal_err == nil {
					if args.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) {
						payload_amount := args.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
						if payload_amount != amount {
							err = fmt.Errorf("inconsistent proof: claimed amount (%d) does not match payload amount (%d)",
								amount, payload_amount)
							return
						}
					}

					payload_decoded = append(payload_decoded, fmt.Sprintf("%s", args))
				} else {
					payload_decoded = append(payload_decoded, unmarshal_err.Error())
				}

				return

			}
		}
	}

	err = fmt.Errorf("Wrong TX Key or wrong transaction")

	return

}
