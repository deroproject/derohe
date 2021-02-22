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

import "fmt"
import "math/big"
import "encoding/hex"

import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/cryptography/bn256"
import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derosuite/walletapi" // to decode encrypted payment ID

// this function will prove detect and decode output amount for the tx
func Prove(proof string, input_tx string, ring [][][]byte, mainnet bool) (receivers []string, amounts []uint64, payload_raw [][]byte, payload_decoded []string, err error) {
	var tx transaction.Transaction

	addr, err := rpc.NewAddress(proof)
	if err != nil {
		return
	}

	if !addr.IsDERONetwork() || !addr.IsIntegratedAddress() || !addr.Proof {
		err = fmt.Errorf("Invalid proof ")
		return
	}

	args := addr.Arguments

	amount := uint64(0)

	if args.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) { // this service is expecting value to be specfic
		amount = args.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
	} else {
		err = fmt.Errorf("Invalid proof.")
		return
	}

	tx_hex, err := hex.DecodeString(input_tx)
	if err != nil {
		return
	}

	err = tx.DeserializeHeader(tx_hex)
	if err != nil {
		return
	}

	// now lets decode the ring

	for t := range ring {
		for i := range ring[t] {
			point_compressed := ring[t][i][:]

			var p bn256.G1
			if err = p.DecodeCompressed(point_compressed[:]); err != nil {
				err = fmt.Errorf("Invalid Ring member.")
				return
			}

			tx.Payloads[t].Statement.Publickeylist = append(tx.Payloads[t].Statement.Publickeylist, &p)
		}
	}

	// okay all inputs have been parsed
	var x bn256.G1
	x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(amount)))
	x.Add(new(bn256.G1).Set(&x), addr.PublicKey.G1())

	for t := range tx.Payloads {
		for k := range tx.Payloads[t].Statement.C {

			if x.String() == tx.Payloads[t].Statement.C[k].String() {

				astring := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[k]))
				astring.Mainnet = mainnet

				receivers = append(receivers, astring.String())
				amounts = append(amounts, amount)

				crypto.EncryptDecryptUserData(addr.PublicKey.G1(), tx.Payloads[t].RPCPayload)
				// skip first byte as it is not guaranteed, even rest of the bytes are not

				payload_raw = append(payload_raw, tx.Payloads[t].RPCPayload[1:])
				var args rpc.Arguments
				if err := args.UnmarshalBinary(tx.Payloads[t].RPCPayload[1:]); err == nil {
					payload_decoded = append(payload_decoded, fmt.Sprintf("%s", args))
				} else {
					payload_decoded = append(payload_decoded, err.Error())
				}

				return

			}
		}
	}

	err = fmt.Errorf("Wrong TX Key or wrong transaction")

	return

}
