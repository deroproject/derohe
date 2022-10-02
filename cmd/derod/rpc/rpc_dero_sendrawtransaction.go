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

package rpc

import "fmt"
import "context"
import "encoding/hex"
import "runtime/debug"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/p2p"
import "github.com/deroproject/derohe/transaction"

// NOTE: finally we have shifted to json api
func SendRawTransaction(ctx context.Context, p rpc.SendRawTransaction_Params) (result rpc.SendRawTransaction_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	var tx transaction.Transaction

	//	rlog.Debugf("Incoming TX from RPC Server")

	//lets decode the tx from hex
	tx_bytes, err := hex.DecodeString(p.Tx_as_hex)

	if err != nil {
		result.Status = "TX could be hex decoded"
		return
	}
	if len(tx_bytes) < 99 {
		result.Status = "TX insufficient length"
		return
	}

	// fmt.Printf("txbytes length %d data %s\n", len(p.Tx_as_hex), p.Tx_as_hex)
	// lets add tx to pool, if we can do it, so  can every one else
	err = tx.Deserialize(tx_bytes)
	if err != nil {
		return
	}

	// lets try to add it to pool

	if err = chain.Add_TX_To_Pool(&tx); err == nil {
		p2p.Broadcast_Tx(&tx, 0) // broadcast tx
		result.Status = "OK"
		result.TXID = fmt.Sprintf("%s", tx.GetHash())
	} else {
		err = fmt.Errorf("Transaction %s rejected by daemon err '%s'", tx.GetHash(), err)
	}
	return
}
