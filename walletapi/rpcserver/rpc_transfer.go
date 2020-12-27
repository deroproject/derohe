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
import "sync"
import "context"
import "encoding/hex"
import "encoding/json"
import "runtime/debug"

//import	"log"
//import 	"net/http"

import "github.com/romana/rlog"

import "github.com/deroproject/derohe/structures"
import "github.com/deroproject/derohe/address"
import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/globals"

var lock sync.Mutex

func (w *WALLET_RPC_APIS) Transfer(ctx context.Context, p structures.Transfer_Params) (result structures.Transfer_Result, err error) {

	lock.Lock()
	defer lock.Unlock()

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	rlog.Debugf("transfer  handler")
	defer rlog.Debugf("transfer  handler finished")

	if len(p.Destinations) < 1 || p.Mixin != 0 && !crypto.IsPowerOf2(int(p.Mixin)) {
		return result, fmt.Errorf("invalid ringsize or destinations")
	}

	rlog.Debugf("Len destinations %d %+v", len(p.Destinations), p)

	payment_id := p.Payment_ID
	if len(payment_id) > 0 && len(payment_id) != 16 {
		return result, fmt.Errorf("payment id should be 16 hexchars") // we should give invalid payment ID
	}
	if _, err := hex.DecodeString(p.Payment_ID); err != nil {
		return result, fmt.Errorf("payment id should be 16 hexchars") // we should give invalid payment ID
	}
	rlog.Debugf("Payment ID %s", payment_id)

	b, err := json.Marshal(p)
	if err == nil {
		rlog.Debugf("Request can be repeated using below command")
		rlog.Debugf(`curl -X POST http://127.0.0.1:18092/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer_split","params":%s}' -H 'Content-Type: application/json'`, string(b))

	}

	var address_list []address.Address
	var amount_list []uint64
	for i := range p.Destinations {
		a, err := globals.ParseValidateAddress(p.Destinations[i].Address)
		if err != nil {
			rlog.Debugf("Warning Parsing address failed %s err %s\n", p.Destinations[i].Address, err)
			return result, fmt.Errorf("Warning Parsing address failed %s err %s\n", p.Destinations[i].Address, err)
		}
		address_list = append(address_list, *a)
		amount_list = append(amount_list, p.Destinations[i].Amount)

	}

	fees_per_kb := uint64(0) // fees  must be calculated by walletapi
	if !w.wallet.GetMode() { // if wallet is in online mode, use the fees, provided by the daemon, else we need to use what is provided by the user

		return result, fmt.Errorf("Wallet is in offline mode")
	}
	tx, err := w.wallet.Transfer(address_list, amount_list, 0, payment_id, fees_per_kb, p.Mixin, false)
	if err != nil {
		rlog.Warnf("Error while building Transaction err %s\n", err)
		return result, err

	}

	//rlog.Infof("fees %s \n", globals.FormatMoney(tx.Statement.Fees))

	//return nil, jsonrpc.ErrInvalidParams()

	if p.Do_not_relay == false { // we do not relay the tx, the user must submit it manually
		// TODO
		err = w.wallet.SendTransaction(tx)

		if err == nil {
			rlog.Debugf("Transaction sent successfully. txid = %s", tx.GetHash())
		} else {
			rlog.Debugf("Warning Transaction sending failed txid = %s, err %s", tx.GetHash(), err)
			return result, fmt.Errorf("Transaction sending failed txid = %s, err %s", tx.GetHash(), err)
		}

	}

	result.Fee = tx.Statement.Fees
	result.Tx_hash = tx.GetHash().String()
	if p.Get_tx_hex { // request need TX blobs, give them
		result.Tx_blob = hex.EncodeToString(tx.SerializeHeader())
	}
	//extract proof key and feed it in here
	if p.Get_tx_key {
		result.Tx_key = w.wallet.GetTXKey(tx.GetHash())
	}
	return result, nil
}
