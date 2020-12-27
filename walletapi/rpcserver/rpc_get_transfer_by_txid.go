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

package rpcserver

import "fmt"
import "context"
import "runtime/debug"
import "encoding/hex"
import "github.com/deroproject/derohe/structures"

func (w *WALLET_RPC_APIS) GetTransferbyTXID(ctx context.Context, p structures.Get_Transfer_By_TXID_Params) (result structures.Get_Transfer_By_TXID_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	txid, err := hex.DecodeString(p.TXID)
	if err != nil {
		return result, fmt.Errorf("%s could NOT be hex decoded err %s", p.TXID, err)
	}

	if len(txid) != 32 {
		return result, fmt.Errorf("%s not 64 hex bytes", p.TXID)
	}

	// if everything is okay, fire the query and convert the result to output format
	entry := w.wallet.Get_Payments_TXID(txid)
	result.Transfer = structures.Transfer_Details{TXID: entry.TXID.String(),
		Payment_ID:  hex.EncodeToString(entry.PaymentID),
		Height:      entry.Height,
		Amount:      entry.Amount,
		Unlock_time: entry.Unlock_Time,
	}
	if entry.Height == 0 {
		return result, fmt.Errorf("Transaction not found. TXID %s", p.TXID)
	}

	for i := range entry.Details.Daddress {
		result.Transfer.Destinations = append(result.Transfer.Destinations,
			structures.Destination{
				Address: entry.Details.Daddress[i],
				Amount:  entry.Details.Amount[i],
			})
	}

	if len(entry.Details.PaymentID) >= 1 {
		result.Transfer.Payment_ID = entry.Details.PaymentID
	}

	if entry.Status == 0 { // if we have an amount
		result.Transfer.Type = "in"
		// send the result
		return result, nil

	}
	// setup in/out
	if entry.Status == 1 { // if we have an amount
		result.Transfer.Type = "out"
		// send the result
		return result, nil

	}

	return result, fmt.Errorf("Transaction not found. TXID %s", p.TXID)
}
