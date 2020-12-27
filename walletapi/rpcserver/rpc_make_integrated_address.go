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

//import	"log"
//import 	"net/http"

import "github.com/deroproject/derohe/structures"

func (w *WALLET_RPC_APIS) MakeIntegratedAddress(ctx context.Context, p structures.Make_Integrated_Address_Params) (result structures.Make_Integrated_Address_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	var payment_id []byte
	if p.Payment_id != "" {
		payid, err := hex.DecodeString(p.Payment_id)
		if err != nil {
			return result, fmt.Errorf("%s could NOT be hex decoded err %s", p.Payment_id, err)
		}

		if len(payid) != 8 {
			return result, fmt.Errorf("%s not 16  hex bytes", p.Payment_id)
		}
		payment_id = payid
	}

	switch len(payment_id) {
	case 8:
		addr := w.wallet.GetRandomIAddress8()
		copy(addr.PaymentID, payment_id)
		result.Integrated_Address = addr.String()
		result.Payment_id = hex.EncodeToString(payment_id)
	default:
		addr := w.wallet.GetRandomIAddress8()
		result.Integrated_Address = addr.String() // default return 8 byte encrypted payment ids
		result.Payment_id = hex.EncodeToString(addr.PaymentID)
	}

	return result, nil
}
