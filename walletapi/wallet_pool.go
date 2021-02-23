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

package walletapi

// the objective of this file is to implememt a pool which sends and retries transactions until they are accepted by the chain

import "fmt"
import "runtime/debug"

//import "encoding/binary"
//import "encoding/hex"

//import "encoding/json"

import "github.com/romana/rlog"

//import "github.com/vmihailenco/msgpack"

//import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derohe/crypto/ringct"
//import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derohe/globals"
//import "github.com/deroproject/derohe/address"
import "github.com/deroproject/derohe/rpc"

//import "github.com/deroproject/derohe/rpc"
//import "github.com/deroproject/derohe/blockchain/inputmaturity"
//import "github.com/deroproject/derohe/crypto/bn256"

type Wallet_Pool []Wallet_Pool_Entry

/// since wallet may try n number of times, it logs all these entries and keeps them until they are accepted
//
type Try struct {
	Height int64       `json:"height"`
	TXID   crypto.Hash `json:"txid"`
	Status string      `json:"status"` // currently what is happening with this tx
}

// should we keep these entries forever
type Wallet_Pool_Entry struct {
	Transfers []rpc.Transfer `json:"transfers"`
	SCDATA    rpc.Arguments  `json:"scdata"`

	Transfer_Everything bool  `json:"transfer_everything"`
	Trigger_Height      int64 `json:"trigger_height"`
	Tries               []Try `json:"tries"`
}

// deep copies an entry
func (w Wallet_Pool_Entry) DeepCopy() (r Wallet_Pool_Entry) {
	r.Transfers = append(r.Transfers, w.Transfers...)
	r.SCDATA = append(r.SCDATA, w.SCDATA...)

	r.Transfer_Everything = w.Transfer_Everything
	r.Trigger_Height = w.Trigger_Height
	r.Tries = append(r.Tries, w.Tries...)

	return r
}

func (w Wallet_Pool_Entry) Amount() (v uint64) {
	for i := range w.Transfers {
		v += w.Transfers[i].Amount
	}

	return v
}

// gives entire cop of wallet pool,
func (w *Wallet_Memory) GetPool() (mirror Wallet_Pool) {
	w.account.Lock()
	defer w.account.Unlock()

	for i := range w.account.Pool {
		mirror = append(mirror, w.account.Pool[i].DeepCopy())
	}
	return mirror
}

func (w *Wallet_Memory) save_if_disk() {
	if w == nil || w.wallet_disk == nil {
		return
	}
	w.wallet_disk.Save_Wallet()
}

// send amount to specific addresses if burn is need do that also
func (w *Wallet_Memory) PoolTransfer(transfers []rpc.Transfer, scdata rpc.Arguments) (uid string, err error) {

	var transfer_all bool

	if _, err = w.TransferPayload0(transfers, transfer_all, scdata, true); err != nil {
		return
	}
	var entry Wallet_Pool_Entry
	defer w.save_if_disk()

	entry.Transfers = append(entry.Transfers, transfers...)
	entry.SCDATA = append(entry.SCDATA, scdata...)

	entry.Transfer_Everything = transfer_all
	entry.Trigger_Height = int64(w.Daemon_Height)

	w.account.Lock()
	defer w.account.Unlock()
	w.account.Pool = append(w.account.Pool, entry)

	return
}

// total which is pending to be sent
func (w *Wallet_Memory) PoolBalance() (balance uint64) {
	w.account.Lock()
	defer w.account.Unlock()

	for i := range w.account.Pool {
		for j := range w.account.Pool[i].Transfers {
			balance += w.account.Pool[i].Transfers[j].Amount //+ w.account.Pool[i].Burn
		}
	}
	return
}

func (w *Wallet_Memory) PoolCount() int {
	w.account.Lock()
	defer w.account.Unlock()

	return len(w.account.Pool)
}

func (w *Wallet_Memory) PoolClear() int {
	defer w.save_if_disk()
	w.account.Lock()
	defer w.account.Unlock()

	count := len(w.account.Pool)
	w.account.Pool = w.account.Pool[:0]

	return count
}

func (w *Wallet_Memory) pool_loop() {
	for {
		WaitNewHeightBlock()
		if w == nil {
			break
		}
		w.processPool(false) // attempt to process  every height change
	}

}

// this function triggers on every height change on daemon
// checks and executes or reexecuts transactions
// a confirmed tx is one which is valid ( not in a side block )
func (w *Wallet_Memory) processPool(checkonly bool) error {
	defer w.save_if_disk()

	w.account.Lock()
	defer w.account.Unlock()

	if len(w.account.Pool) < 1 {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {
			rlog.Warnf("Stack trace  \n%s", debug.Stack())

		}
	}()

	if !w.GetMode() { // if wallet is in offline mode , we cannot do anything
		return fmt.Errorf("wallet is in offline mode")

	}

	if !IsDaemonOnline() {
		return fmt.Errorf("not connected")
	}

	var info rpc.GetInfo_Result
	if err := rpc_client.Call("DERO.GetInfo", nil, &info); err != nil {
		rlog.Warnf("DERO.GetInfo Call failed: %v", err)
		Connected = false
		return err
	}

	for i := 0; i < len(w.account.Pool); i++ {

		var try *Try = &Try{}
		if len(w.account.Pool[i].Tries) >= 1 {
			try = &w.account.Pool[i].Tries[len(w.account.Pool[i].Tries)-1]
		}

		if len(w.account.Pool[i].Tries) >= 1 { // we hav tried atleast once, keep monitoring it for stable blocks
			var tx_result rpc.GetTransaction_Result

			if err := rpc_client.Call("DERO.GetTransaction", rpc.GetTransaction_Params{Tx_Hashes: []string{try.TXID.String()}}, &tx_result); err != nil {
				rlog.Errorf("gettransa rpc failed err %s\n", err)
				return fmt.Errorf("gettransa rpc failed err %s", err)
			}

			if tx_result.Txs_as_hex[0] == "" {
				try.Status = "Lost (not in mempool/chain, Waiting for more"
				if try.Height > info.StableHeight { // we have attempted, lets wait some blocks, this  needs to be optimized, for instant transfer
					continue // try other txs
				}
			} else if tx_result.Txs[0].In_pool {
				try.Status = "TX in Mempool"
				continue
			} else if tx_result.Txs[0].ValidBlock != "" { // if the result is valid in one of the blocks
				try.Status = fmt.Sprintf("Mined in %s (%d confirmations)", tx_result.Txs[0].ValidBlock, info.TopoHeight-tx_result.Txs[0].Block_Height)
				if try.Height < (info.StableHeight + 1) { // successful confirmation
					w.account.PoolHistory = append(w.account.PoolHistory, w.account.Pool[i])
					rlog.Infof("tx %s confirmed successfully  at stableheight %d  height %d trigger_height %d\n", try.TXID.String(), info.StableHeight, try.Height, w.account.Pool[i].Trigger_Height)
					w.account.Pool = append(w.account.Pool[:i], w.account.Pool[i+1:]...)
					i-- // so another element at same place gets used

				}
				continue
			} else {
				try.Status = fmt.Sprintf("Mined in sideblock (%d confirmations, waiting for more)", info.Height-try.Height)
				if try.Height < info.StableHeight { // we have attempted, lets wait some blocks, this  needs to be optimized, for instant transfer
					continue // try other txs
				}
			}

		}

		if !checkonly {

			// we are here means we have to dispatch tx first time or again or whatever the case
			rlog.Debugf("%d tries, sending\n", len(w.account.Pool[i].Tries))

			tx, err := w.TransferPayload0(w.account.Pool[i].Transfers, w.account.Pool[i].Transfer_Everything, w.account.Pool[i].SCDATA, false)
			//tx, err := w.Transfer_Simplified(w.account.Pool[i].Addr, w.account.Pool[i].Amount, w.account.Pool[i].Data)
			if err != nil {
				rlog.Errorf("err building tx %s\n", err)
				return err
			}

			if err = w.SendTransaction(tx); err != nil {
				rlog.Errorf("err sending tx %s\n", err)
				return err
			}
			rlog.Infof("dispatched tx %s at height %d trigger_height %d\n", tx.GetHash().String(), tx.Height, w.account.Pool[i].Trigger_Height)
			w.account.Pool[i].Tries = append(w.account.Pool[i].Tries, Try{int64(tx.Height), tx.GetHash(), "Dispatched to mempool"})
			break // we can only send one tx per height
		}
	}

	return nil
}
