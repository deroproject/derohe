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
import "encoding/hex"
import "runtime/debug"
import "github.com/deroproject/derohe/structures"

func (w *WALLET_RPC_APIS) GetTransfers(ctx context.Context, p structures.Get_Transfers_Params) (result structures.Get_Transfers_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	//entries := h.r.w.Show_Transfers(p.In, p.In,p.Out,p.Failed, p.Pool,p.Min_Height,p.Max_Height)
	in_entries := w.wallet.Show_Transfers(p.In, p.In, false, false, false, false, p.Min_Height, p.Max_Height)
	out_entries := w.wallet.Show_Transfers(false, false, p.Out, false, false, false, p.Min_Height, p.Max_Height)
	for j := range in_entries {
		result.In = append(result.In, structures.Transfer_Details{TXID: in_entries[j].TXID.String(),
			Payment_ID:  hex.EncodeToString(in_entries[j].PaymentID),
			Height:      in_entries[j].Height,
			Amount:      in_entries[j].Amount,
			Unlock_time: in_entries[j].Unlock_Time,
			Type:        "in",
		})

	}

	for j := range out_entries {
		transfer := structures.Transfer_Details{TXID: out_entries[j].TXID.String(),
			Payment_ID:  hex.EncodeToString(out_entries[j].PaymentID),
			Height:      out_entries[j].Height,
			Amount:      out_entries[j].Amount,
			Unlock_time: out_entries[j].Unlock_Time,
			Type:        "out",
		}

		for i := range out_entries[j].Details.Daddress {
			transfer.Destinations = append(transfer.Destinations,
				structures.Destination{
					Address: out_entries[j].Details.Daddress[i],
					Amount:  out_entries[j].Details.Amount[i],
				})
		}
		result.Out = append(result.Out, transfer)

	}

	return result, nil
}
