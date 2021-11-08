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
import "runtime/debug"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/cryptography/crypto"

var lock sync.Mutex

func Transfer(ctx context.Context, p rpc.Transfer_Params) (result rpc.Transfer_Result, err error) {

	lock.Lock()
	defer lock.Unlock()

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	w := fromContext(ctx)

	for _, t := range p.Transfers {
		_, err = t.Payload_RPC.CheckPack(transaction.PAYLOAD0_LIMIT)
		if err != nil {
			return
		}
	}

	if !w.wallet.GetMode() { // if wallet is in online mode, use the fees, provided by the daemon, else we need to use what is provided by the user
		return result, fmt.Errorf("Wallet is in offline mode")
	}

	// translate rpc to arguments

	//fmt.Printf("incoming transfer params %+v\n", p)

	if p.SC_Code != "" {
		p.SC_RPC = append(p.SC_RPC, rpc.Argument{rpc.SCACTION, rpc.DataUint64, uint64(rpc.SC_INSTALL)})
		p.SC_RPC = append(p.SC_RPC, rpc.Argument{rpc.SCCODE, rpc.DataString, p.SC_Code})
	}

	if p.SC_ID != "" {
		p.SC_RPC = append(p.SC_RPC, rpc.Argument{rpc.SCACTION, rpc.DataUint64, uint64(rpc.SC_CALL)})
		p.SC_RPC = append(p.SC_RPC, rpc.Argument{rpc.SCID, rpc.DataHash, crypto.HashHexToHash(p.SC_ID)})
	}

	/*
		 // if you need to send tx now mostly for testing purpose use this
		tx, err := w.wallet.TransferPayload0(p.Transfers, false, p.SC_RPC, false)
		if err != nil {
			rlog.Warnf("Error while building Transaction err %s\n", err)
			return err

		}

		err = w.wallet.SendTransaction(tx)
		if err != nil {
			return err
		}
	*/

	tx, err := w.wallet.TransferPayload0(p.Transfers, p.Ringsize, false, p.SC_RPC, false)
	if err != nil {
		w.logger.V(1).Error(err, "Error building tx")
		return result, err
	}

	err = w.wallet.SendTransaction(tx)
	if err != nil {
		return result, err
	}

	// we must return a txid if everything went alright
	result.TXID = tx.GetHash().String()

	/*
		uid, err := w.wallet.PoolTransfer(p.Transfers, p.SC_RPC)
		if err != nil {
			return err

		}
		_ = uid
	*/
	return result, nil
}
