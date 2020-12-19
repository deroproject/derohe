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
import "encoding/binary"

import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/crypto/bn256"
import "github.com/deroproject/derohe/address"
import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derosuite/walletapi" // to decode encrypted payment ID

// this function will prove detect and decode output amount for the tx
func Prove(proof string, input_tx string, mainnet bool) (receivers []string, amounts []uint64, payids [][]byte, err error) {
	var tx transaction.Transaction

	addr, err := address.NewAddress(proof)
	if err != nil {
		return
	}

	if !addr.IsDERONetwork() || !addr.IsIntegratedAddress() || !addr.Proof {
		err = fmt.Errorf("Invalid proof ")
		return
	}

	if len(addr.PaymentID) != 8 {
		err = fmt.Errorf("Invalid proof paymentid")
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

	// okay all inputs have been parsed

	amount := binary.LittleEndian.Uint64(addr.PaymentID)

	var x bn256.G1
	x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(amount)))
	x.Add(new(bn256.G1).Set(&x), addr.PublicKey.G1())

	for k := range tx.Statement.C {
		if x.String() == tx.Statement.C[k].String() {

			astring := address.NewAddressFromKeys((*crypto.Point)(tx.Statement.Publickeylist[k]))
			astring.Mainnet = mainnet

			receivers = append(receivers, astring.String())
			amounts = append(amounts, amount)

			//decode payment id
			output := crypto.EncryptDecryptPaymentID(addr.PublicKey.G1(), tx.PaymentID[:])
			payids = append(payids, output)
			return

		}
	}

	err = fmt.Errorf("Wrong TX Key or wrong transaction")

	return

}
