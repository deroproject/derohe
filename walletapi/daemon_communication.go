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

// this file needs  serious improvements but have extremely limited time
/* this file handles communication with the daemon
 * this includes receiving output information
 *
 * *
 */
//import "io"
//import "os"
import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/deroproject/derohe/block"
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/errormsg"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
)

//import "bufio"

//import "runtime"
//import "compress/gzip"

//import "github.com/vmihailenco/msgpack"

//import "github.com/gorilla/websocket"
//import "github.com/mafredri/cdp/rpcc"

// this global variable should be within wallet structure
var Connected bool = false

var daemon_height int64
var daemon_topoheight int64

// return daemon height
func Get_Daemon_Height() int64 {
	return daemon_height
}

// return topoheight of daemon
func Get_Daemon_TopoHeight() int64 {
	return daemon_topoheight
}

var simulator bool // turns on simulator, which has 0 fees

// there should be no global variables, so multiple wallets can run at the same time with different assset

var endpoint string

var output_lock sync.Mutex

var NotifyNewBlock *sync.Cond = sync.NewCond(&sync.Mutex{})
var NotifyHeightChange *sync.Cond = sync.NewCond(&sync.Mutex{})

// this function will wait n goroutines to wait for new block
func WaitNewBlock() {
	NotifyNewBlock.L.Lock()
	NotifyNewBlock.Wait()
	NotifyNewBlock.L.Unlock()
}

// this function will wait n goroutines to wait  till height changes
func WaitNewHeightBlock() {
	NotifyHeightChange.L.Lock()
	NotifyHeightChange.Wait()
	NotifyHeightChange.L.Unlock()
}

func Notify_broadcaster(req *jrpc2.Request) {
	timer.Reset(timeout) // connection is alive
	switch req.Method() {
	case "Block":
		NotifyNewBlock.L.Lock()
		NotifyNewBlock.Broadcast()
		NotifyNewBlock.L.Unlock()
	case "Height":
		NotifyHeightChange.L.Lock()
		NotifyHeightChange.Broadcast()
		NotifyHeightChange.L.Unlock()
		go test_connectivity()
	case "MiniBlock": // we can skip this
	default:
		logger.V(1).Info("Notification received", "method", req.Method())
	}

}

var Daemon_Endpoint string
var Daemon_Endpoint_Active string

func get_daemon_address() string {
	if globals.Arguments["--remote"] == true && globals.IsMainnet() {
		Daemon_Endpoint_Active = config.REMOTE_DAEMON + fmt.Sprintf(":%d", config.Mainnet.RPC_Default_Port)
	}

	// if user provided endpoint has error, use default
	if Daemon_Endpoint_Active == "" {
		Daemon_Endpoint_Active = "127.0.0.1:" + fmt.Sprintf("%d", config.Mainnet.RPC_Default_Port)
		if !globals.IsMainnet() {
			Daemon_Endpoint_Active = "127.0.0.1:" + fmt.Sprintf("%d", config.Testnet.RPC_Default_Port)
		}
	}

	if globals.Arguments["--daemon-address"] != nil {
		Daemon_Endpoint_Active = globals.Arguments["--daemon-address"].(string)
	}

	return Daemon_Endpoint_Active
}

// tests connectivity when connectivity to daemon
func test_connectivity() (err error) {
	var result string

	// Issue a call with a response.
	if err = rpc_client.Call("DERO.Echo", []string{"hello", "world"}, &result); err != nil {
		logger.V(1).Error(err, "DERO.Echo Call failed:")
		Connected = false
		return
	}
	//fmt.Println(result)

	var info rpc.GetInfo_Result
	// Issue a call with a response.
	if err = rpc_client.Call("DERO.GetInfo", nil, &info); err != nil {
		logger.V(1).Error(err, "DERO.GetInfo Call failed:")
		Connected = false
		return
	}

	// detect whether both are in different modes
	//  daemon is in testnet and wallet in mainnet or
	// daemon
	if info.Testnet != !globals.IsMainnet() {
		err = fmt.Errorf("Mainnet/TestNet  is different between wallet/daemon.Please run daemon/wallet without --testnet")
		logger.Error(err, "Mainnet/Testnet mismatch")
		return
	}

	if strings.ToLower(info.Network) == "simulator" {
		simulator = true
	}
	daemon_height = info.Height
	daemon_topoheight = info.TopoHeight
	//	logger.Info("connection is maintained")
	return nil
}

// triggers syncing with wallet every 5 seconds
func (w *Wallet_Memory) sync_loop() {
	//logger = globals.Logger
	for {
		select {
		case <-w.Quit:
			break
		default:
		}

		if w.account.lastsaved.IsZero() || time.Since(w.account.lastsaved) > w.account.SaveChangesEvery {
			w.save_if_disk() // save wallet()
			w.account.lastsaved = time.Now()
			//	w.db.Sync()
		}

		if IsDaemonOnline() && test_connectivity() != nil {
			time.Sleep(timeout) // wait 5 seconds
			continue
		}

		if len(w.account.EntriesNative) == 0 {
			if err := w.Sync_Wallet_Memory_With_Daemon(); err != nil {
				logger.Error(err, "wallet syncing err")
			}
		} else {
			for k := range w.account.EntriesNative {
				err := w.Sync_Wallet_Memory_With_Daemon_internal(k)
				if err != nil {
					globals.Logger.V(3).Error(err, "Error while syncing SCID", "scid", k)
				}
			}
		}

		time.Sleep(timeout) // wait 5 seconds
	}
}

func (cli *Client) Call(method string, params interface{}, result interface{}) error {
	return cli.RPC.CallResult(context.Background(), method, params, result)
}

// returns whether wallet was online some time ago
func (w *Wallet_Memory) IsDaemonOnlineCached() bool {
	return Connected
}

// currently process url  with compatibility for older ip address
func buildurl(endpoint string) string {
	if strings.IndexAny(endpoint, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") >= 0 { // url is already complete
		return strings.TrimSuffix(endpoint, "/")
	} else {
		return "http://" + endpoint
	}
}

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func IsDaemonOnline() bool {
	if rpc_client.WS == nil || rpc_client.RPC == nil {
		return false
	}
	return true
}

// sync the wallet with daemon, this is instantaneous and can be done with a single call
// we have now the apis to avoid polling
func (w *Wallet_Memory) Sync_Wallet_Memory_With_Daemon_internal(scid crypto.Hash) (err error) {

	if !IsDaemonOnline() {
		daemon_height = 0
		daemon_topoheight = 0
		return fmt.Errorf("Daemon is offline")
	} else {
		//w.random_ring_members()
		//rlog.Debugf("wallet topo height %d daemon online topo height %d\n", w.account.TopoHeight, w.Daemon_TopoHeight)
		previous := w.getEncryptedBalanceresult(scid).Data

		if _, _, _, e, err := w.GetEncryptedBalanceAtTopoHeight(scid, -1, w.GetAddress().String()); err == nil {

			//fmt.Printf("data '%s' previous '%s' scid %s\n", w.account.Balance_Result[scid].Data, previous, scid)
			if w.getEncryptedBalanceresult(scid).Data != previous {
				b := w.DecodeEncryptedBalanceNow(e) // try to decode balance

				if scid.IsZero() {
					w.account.Balance_Mature = b
				}
				w.Lock()
				w.account.Balance[scid] = b
				w.Unlock()
				w.SyncHistory(scid) // also update statement
			}

			w.save_if_disk() // save wallet
		} else {
			return err
		}
	}

	return
}

func (w *Wallet_Memory) Sync_Wallet_Memory_With_Daemon() (err error) {
	var scid crypto.Hash

	return w.Sync_Wallet_Memory_With_Daemon_internal(scid)
}

func (w *Wallet_Memory) NameToAddress(name string) (addr string, err error) {
	if name == "" {
		return addr, fmt.Errorf("empty string is not a valid address")
	}

	if !IsDaemonOnline() {
		err = fmt.Errorf("offline or not connected. cannot translate name to address")
		return
	}

	var result rpc.NameToAddress_Result
	if err = rpc_client.Call("DERO.NameToAddress", rpc.NameToAddress_Params{Name: name, TopoHeight: -1}, &result); err != nil {
		return
	}

	if result.Status == "OK" {
		addr = result.Address
		return
	} else {
		err = fmt.Errorf("Err %s", result.Status)
		return
	}
}

// this is as simple as it gets
// single threaded communication to relay TX to daemon
// if this is successful, then daemon is in control

func (w *Wallet_Memory) SendTransaction(tx *transaction.Transaction) (err error) {
	if tx == nil {
		return fmt.Errorf("Can not send nil transaction")
	}

	if !IsDaemonOnline() {
		return fmt.Errorf("offline or not connected. cannot send transaction.")
	}

	params := rpc.SendRawTransaction_Params{Tx_as_hex: hex.EncodeToString(tx.Serialize())}
	var result rpc.SendRawTransaction_Result

	if err := rpc_client.Call("DERO.SendRawTransaction", params, &result); err != nil {
		return err
	}

	if result.Status == "OK" {
		return nil
	} else {
		err = fmt.Errorf("Err %s", result.Status)
	}

	return
}

// decode encrypted balance now
// it may take a long time, its currently sing threaded, need to parallelize
func (w *Wallet_Memory) DecodeEncryptedBalanceNow(el *crypto.ElGamal) uint64 {

	balance_point := new(bn256.G1).Add(el.Left, new(bn256.G1).Neg(new(bn256.G1).ScalarMult(el.Right, w.account.Keys.Secret.BigInt())))
	return Balance_lookup_table.Lookup(balance_point, w.account.Balance_Mature)
}

func (w *Wallet_Memory) GetSelfEncryptedBalanceAtTopoHeight(scid crypto.Hash, topoheight int64) (r rpc.GetEncryptedBalance_Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recovered while GetSelfEncryptedBalanceAtTopoHeight", "r", r, "stack", debug.Stack())
			err = fmt.Errorf("Recovered while GetSelfEncryptedBalanceAtTopoHeight r %s stack %s", r, string(debug.Stack()))
		}
	}()

	err = rpc_client.Call("DERO.GetEncryptedBalance", rpc.GetEncryptedBalance_Params{SCID: scid, Address: w.GetAddress().String(), TopoHeight: topoheight}, &r)
	return
}

// this is as simple as it gets
// single threaded communication  gets whether the the key image is spent in pool or in blockchain
// this can leak informtion which keyimage belongs to us
// TODO in order to stop privacy leaks we must guess this information somehow on client side itself
// maybe the server can broadcast a bloomfilter or something else from the mempool keyimages
func (w *Wallet_Memory) GetEncryptedBalanceAtTopoHeight(scid crypto.Hash, topoheight int64, accountaddr string) (bits int, lastused uint64, blid crypto.Hash, e *crypto.ElGamal, err error) {

	defer func() {
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recovered while GetEncryptedBalanceAtTopoHeight", "r", r, "stack", debug.Stack())
			err = fmt.Errorf("Recovered while GetEncryptedBalanceAtTopoHeight  r %s stack %s", r, debug.Stack())
		}
	}()

	if !w.GetMode() { // if wallet is in offline mode , we cannot do anything
		err = fmt.Errorf("wallet is in offline mode")
		return
	}

	if !IsDaemonOnline() {
		err = fmt.Errorf("offline or not connected")
		return
	}

	//var params rpc.GetEncryptedBalance_Params
	var result rpc.GetEncryptedBalance_Result

	// Issue a call with a response.
	if err = rpc_client.Call("DERO.GetEncryptedBalance", rpc.GetEncryptedBalance_Params{SCID: scid, Address: accountaddr, TopoHeight: topoheight}, &result); err != nil {
		logger.Error(err, "DERO.GetEncryptedBalance Call failed:")

		if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(errormsg.ErrAccountUnregistered.Error())) && accountaddr == w.GetAddress().String() && scid.IsZero() {
			w.Error = errormsg.ErrAccountUnregistered
			//fmt.Printf("setting unregisterd111 err %s scid %s topoheight %d\n",err,scid, topoheight)
			//fmt.Printf("debug stack %s\n",debug.Stack())

			return
		}

		// all SCID users are considered registered and their balance is assumed zero
		if !scid.IsZero() {
			if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(errormsg.ErrAccountUnregistered.Error())) {
				if addr, err1 := rpc.NewAddress(accountaddr); err1 != nil {
					err = err1
					return
				} else {
					e = crypto.ConstructElGamal(addr.PublicKey.G1(), crypto.ElGamal_BASE_G) // init zero balance
					bits = 0                                                                // since this is an SC we can
					err = nil
					return
				}
			}
		}
		return
	}

	//		fmt.Printf("GetEncryptedBalance result  %+v\n", result)
	if scid.IsZero() && accountaddr == w.GetAddress().String() {
		if result.Status == errormsg.ErrAccountUnregistered.Error() {
			w.Error = errormsg.ErrAccountUnregistered
			w.account.Registered = false
		} else {
			w.account.Registered = true
		}
	}

	//	fmt.Printf("status '%s' err '%s'  %+v  %+v \n", result.Status , w.Error , result.Status == errormsg.ErrAccountUnregistered.Error()  , accountaddr == w.account.GetAddress().String())

	if scid.IsZero() && result.Status == errormsg.ErrAccountUnregistered.Error() {
		err = fmt.Errorf("%s", result.Status)
		return
	}

	if topoheight == -1 {
		daemon_height = result.DHeight
		daemon_topoheight = result.DTopoheight
		w.Merkle_Balance_TreeHash = result.DMerkle_Balance_TreeHash
	}

	if topoheight == -1 && accountaddr == w.GetAddress().String() {
		//fmt.Printf("topoheight %d accountaddr '%s' waddress '%s'\n ",topoheight,accountaddr,w.GetAddress().String())

		w.setEncryptedBalanceresult(scid, result)
		w.account.TopoHeight = result.Topoheight
	}

	if scid.IsZero() && result.Status != "OK" {
		err = fmt.Errorf("%s", result.Status)
		return
	}

	hexdecoded, err := hex.DecodeString(result.Data)
	if err != nil {
		return
	}

	if accountaddr == w.GetAddress().String() && scid.IsZero() {
		w.Error = nil
	}

	var nb crypto.NonceBalance
	nb.Unmarshal(hexdecoded)

	return result.Bits, nb.NonceHeight, result.BlockHash, nb.Balance, nil
}

func (w *Wallet_Memory) DecodeEncryptedBalance_Memory(el *crypto.ElGamal, hint uint64) (balance uint64) {
	var balance_point bn256.G1
	balance_point.Add(el.Left, new(bn256.G1).Neg(new(bn256.G1).ScalarMult(el.Right, w.account.Keys.Secret.BigInt())))
	return Balance_lookup_table.Lookup(&balance_point, hint)
}

func (w *Wallet_Memory) GetDecryptedBalanceAtTopoHeight(scid crypto.Hash, topoheight int64, accountaddr string) (balance uint64, noncetopo uint64, err error) {
	_, noncetopo, _, encrypted_balance, err := w.GetEncryptedBalanceAtTopoHeight(scid, topoheight, accountaddr)
	if err != nil {
		return 0, 0, err
	}

	return w.DecodeEncryptedBalance_Memory(encrypted_balance, 0), noncetopo, nil
}

// sync history of wallet from blockchain
func (w *Wallet_Memory) Random_ring_members(scid crypto.Hash) (alist []string) {
	var result rpc.GetRandomAddress_Result

	//fmt.Printf("getting ring members %s  %s\n",scid.String(), debug.Stack())

	// Issue a call with a response.
	if err := rpc_client.Call("DERO.GetRandomAddress", rpc.GetRandomAddress_Params{SCID: scid}, &result); err != nil {
		logger.V(1).Error(err, "DERO.GetRandomAddress Call failed:")
		return
	}

	//fmt.Printf("getting ring members %d\n",len(result.Address))
	for _, k := range result.Address {
		if k != w.GetAddress().String() {
			alist = append(alist, k)
		}
	}
	//fmt.Printf("got ring members %d\n",len(result.Address))
	return
}

// sync history of wallet from blockchain
var sync_multilock sync.Mutex // make sync history single threaded
func (w *Wallet_Memory) SyncHistory(scid crypto.Hash) (balance uint64) {
	sync_multilock.Lock()
	defer sync_multilock.Unlock()

	defer func() {
		if r := recover(); r != nil {
			logger.V(1).Error(nil, "Recovered while syncing connecting", "r", r, "stack", debug.Stack())
		}
	}()

	if w.getEncryptedBalanceresult(scid).Registration < 0 { // unregistered so skip
		return
	}

	last_topo_height := int64(-1)

	//fmt.Printf("finding sync point  ( Registration point %d)\n", w.getEncryptedBalanceresult(scid).Registration)

	entries := w.account.EntriesNative[scid]

	logger.Info("syncing loop ", "total_entries", len(entries))

	defer func() { logger.Info("syncing loop completed", "total_entries", len(w.account.EntriesNative[scid])) }()

	// we need to find a sync point, to minimize traffic
	for i := len(entries) - 1; i >= 0; {

		// below condition will trigger if chain got pruned on server
		if w.getEncryptedBalanceresult(scid).Registration >= entries[i].TopoHeight { // keep old history if chain got pruned
			break
		}

		last_topo_height = entries[i].TopoHeight

		var result rpc.GetBlockHeaderByHeight_Result

		// Issue a call with a response.
		if err := rpc_client.Call("DERO.GetBlockHeaderByTopoHeight", rpc.GetBlockHeaderByTopoHeight_Params{TopoHeight: uint64(entries[i].TopoHeight)}, &result); err != nil {
			logger.V(1).Error(err, "DERO.GetBlockHeaderByTopoHeight Call failed:")
			return 0
		}

		if result.Status != "OK" {
			logger.Error(nil, "syncing loop status failed", "Status", result.Status, "topo", entries[i].TopoHeight)
			return
		}

		// wallet previous synced onto a side block chain / revert
		if entries[i].BlockHash != result.Block_Header.Hash {
			logger.Info("syncing loop header mismatch ", "i", i, "block_hash", entries[i].BlockHash)
			skip := 1
			if i >= 1 && last_topo_height == entries[i-1].TopoHeight { // skipping any entries withing same block
				for ; i >= 1; i-- {
					if last_topo_height == entries[i-1].TopoHeight {
						skip++
					} else {
						break
					}
				}
			}
			entries = entries[:i-skip]
			w.account.EntriesNative[scid] = entries
			logger.Info("syncing loop skipped ", "i", i, "skip", skip)
			continue
		}

		if i <= 0 {
			w.account.EntriesNative[scid] = entries[:0] // discard all entries
			logger.Info("syncing loop discarding all entries", "i", i)
			break
		}

		// we have found a matching block hash, start syncing from here
		if result.Block_Header.Hash == entries[i].BlockHash {
			logger.Info("syncing loop from pos", "i", i, "start_topo", entries[i].TopoHeight+1, "end_topo", w.getEncryptedBalanceresult(scid).Topoheight)

			w.synchistory_internal(scid, entries[i].TopoHeight+1, w.getEncryptedBalanceresult(scid).Topoheight)
			return
		}

		return

	}

	logger.Info("syncing loop using Registration", "registraion", w.getEncryptedBalanceresult(scid).Registration)

	// if we reached here, means we should sync from scratch
	w.synchistory_internal(scid, w.getEncryptedBalanceresult(scid).Registration, w.getEncryptedBalanceresult(scid).Topoheight)

	//if w.account.Registration >= 0 {
	// err :=
	// err =  w.synchistory_internal(w.account.Registration,6)

	// }
	// fmt.Printf("syncing err %s\n",err)
	// fmt.Printf("entries %+v\n", w.account.Entries)

	return 0
}

// sync history
func (w *Wallet_Memory) synchistory_internal(scid crypto.Hash, start_topo, end_topo int64) error {
	var err error
	var start_balance_e *crypto.ElGamal

	logger.Info("syncing loop  starting internal ", "start_topo", start_topo, "end_topo", end_topo)

	if w.account.TrackRecentBlocks > 0 && daemon_topoheight >= w.account.TrackRecentBlocks {
		start_topo = daemon_topoheight - w.account.TrackRecentBlocks
	}
	if start_topo == w.getEncryptedBalanceresult(scid).Registration {
		start_balance_e = crypto.ConstructElGamal(w.account.Keys.Public.G1(), crypto.ElGamal_BASE_G)
	} else {
		_, _, _, start_balance_e, err = w.GetEncryptedBalanceAtTopoHeight(scid, start_topo, w.GetAddress().String())
		if err != nil {
			logger.Error(err, "syncing info failed", "start_topo", start_topo)
			return err
		}
	}

	_, _, _, end_balance_e, err := w.GetEncryptedBalanceAtTopoHeight(scid, end_topo, w.GetAddress().String())
	if err != nil {
		logger.Error(err, "syncing info failed", "end_topo", end_topo)
		return err
	}

	return w.synchistory_internal_binary_search(0, scid, start_topo, start_balance_e, end_topo, end_balance_e)

}

func (w *Wallet_Memory) synchistory_internal_binary_search(level int, scid crypto.Hash, start_topo int64, start_balance_e *crypto.ElGamal, end_topo int64, end_balance_e *crypto.ElGamal) error {

	var err error

	//defer fmt.Printf("end %d start %d err %s\n", end_topo, start_topo, err)

	if end_topo < 0 {
		return fmt.Errorf("done")
	}

	//if bytes.Compare(start_balance_e.Serialize(), end_balance_e.Serialize()) == 0 {
	//	logger.Info("syncing loop  same encrypted data so skipping ","start_topo",start_topo, "end_topo",end_topo)
	//    return nil
	//}

	defer globals.Recover(0)

	//for start_topo <= end_topo{
	{
		median := (start_topo + end_topo) / 2
		//	fmt.Printf("%slevel %d low %d high %d median %d\n", strings.Repeat("\t", level), level, start_topo, end_topo, median)
		if start_topo == median {
			if err = w.synchistory_block(scid, start_topo); err != nil {
				logger.Error(err, "syncing block failed", "start_topo", start_topo)
				return err
			}
		}

		if end_topo-start_topo <= 1 {
			if err = w.synchistory_block(scid, end_topo); err != nil {
				logger.Error(err, "syncing block failed", "end_topo", end_topo)
				return err
			}
			return nil
		}

		_, _, _, median_balance_e, err := w.GetEncryptedBalanceAtTopoHeight(scid, median, w.GetAddress().String())
		if err != nil {
			logger.Error(err, "syncing block getting balance failed", "median", median)
			return err
		}

		// check if there is a change in lower section, if yes process more
		//fmt.Printf("%slevel %d checking lower\n", strings.Repeat("\t", level), level)
		if start_topo == w.getEncryptedBalanceresult(scid).Registration || bytes.Compare(start_balance_e.Serialize(), median_balance_e.Serialize()) != 0 {
			err = w.synchistory_internal_binary_search(level+1, scid, start_topo, start_balance_e, median, median_balance_e)
			if err != nil {
				logger.Error(err, "syncing block synchistory_internal_binary_search failed", "level+1", level+1, "start_topo", start_topo, "median", median)
				return err
			}
		}

		// check if there is a change in higher section, if yes process more
		//fmt.Printf("%slevel %d checking higher\n", strings.Repeat("\t", level), level)
		if bytes.Compare(median_balance_e.Serialize(), end_balance_e.Serialize()) != 0 {
			err = w.synchistory_internal_binary_search(level+1, scid, median, median_balance_e, end_topo, end_balance_e)
			if err != nil {
				logger.Error(err, "syncing block synchistory_internal_binary_search failed", "level+1", level+1, "median", median, "end_topo", end_topo)
				return err
			}
		}
	}

	return nil
}

// extract history from a single block
// first get a block, then get all the txs
// Todo we should expose an API to get all txs which have the specific address as ring member
// for a particular block
// for the entire chain
func (w *Wallet_Memory) synchistory_block(scid crypto.Hash, topo int64) (err error) {

	var local_entries []rpc.Entry

	compressed_address := w.account.Keys.Public.EncodeCompressed()

	var previous_balance_e, current_balance_e *crypto.ElGamal
	var previous_balance, current_balance, total_sent, total_received uint64

	if topo <= 0 || w.getEncryptedBalanceresult(scid).Registration == topo {
		previous_balance_e = crypto.ConstructElGamal(w.account.Keys.Public.G1(), crypto.ElGamal_BASE_G)
	} else {
		_, _, _, previous_balance_e, err = w.GetEncryptedBalanceAtTopoHeight(scid, topo-1, w.GetAddress().String())
		if err != nil {
			return err
		}
	}

	//logger.Info("syncing block", "topo", topo)
	_, _, _, current_balance_e, err = w.GetEncryptedBalanceAtTopoHeight(scid, topo, w.GetAddress().String())
	if err != nil {
		return err
	}

	var bl block.Block
	var bresult rpc.GetBlock_Result
	if err = rpc_client.Call("DERO.GetBlock", rpc.GetBlock_Params{Height: uint64(topo)}, &bresult); err != nil {
		return fmt.Errorf("getblock rpc failed")
	}

	if bresult.Block_Header.SideBlock && w.getEncryptedBalanceresult(scid).Registration != topo {
		return nil
	}

	EWData := fmt.Sprintf("%x", current_balance_e.Serialize())

	previous_balance = w.DecodeEncryptedBalance_Memory(previous_balance_e, 0)
	current_balance = w.DecodeEncryptedBalance_Memory(current_balance_e, 0)

	// we can skip some check if both balances are equal ( means we are ring members in this block)
	// this check will also fail if we total spend == total receivein the block
	// currently it is not implmented, and we bruteforce everything

	block_bin, err := hex.DecodeString(bresult.Blob)
	if err != nil {
		return err
	}
	if err = bl.Deserialize(block_bin); err != nil {
		return err
	}

	if !bresult.Block_Header.SideBlock && len(bl.Tx_hashes) >= 1 {

		for i := range bl.Tx_hashes {
			var tx transaction.Transaction

			var tx_params rpc.GetTransaction_Params
			var tx_result rpc.GetTransaction_Result

			tx_params.Tx_Hashes = append(tx_params.Tx_Hashes, bl.Tx_hashes[i].String())

			//fmt.Printf("Requesting tx data %s\n", bl.Tx_hashes[i].String())

			if err = rpc_client.Call("DERO.GetTransaction", tx_params, &tx_result); err != nil {
				return fmt.Errorf("gettransa rpc failed %s", err)
			}

			tx_bin, err := hex.DecodeString(tx_result.Txs_as_hex[0])
			if err != nil {
				return err
			}
			if err = tx.Deserialize(tx_bin); err != nil {
				logger.V(1).Error(err, "Error deserialing tx", "txid", bl.Tx_hashes[i].String(), "incoming bytes", tx_result.Txs_as_hex[0])
				continue
			}

			if tx.TransactionType == transaction.REGISTRATION {
				continue
			}

			// if daemon was syncing/or disk corrupption, it may not give data, so skip
			if len(tx_result.Txs) == 0 {
				return fmt.Errorf("Daemon did not expandd tx %s", bl.Tx_hashes[i].String())
			}

			// since balance might change with tx, we track within tx using this
			previous_balance_e_tx := new(crypto.ElGamal).Deserialize(previous_balance_e.Serialize())

			for t := range tx.Payloads {
				if len(tx_result.Txs) == 0 {
					continue
				}
				if len(tx_result.Txs[0].Ring) == 0 {
					continue
				}
				_ = int(tx.Payloads[t].Statement.RingSize)
				_ = tx_result.Txs[0]
				_ = tx_result.Txs[0].Ring
				_ = tx_result.Txs[0].Ring[t]
				if int(tx.Payloads[t].Statement.RingSize) != len(tx_result.Txs[0].Ring[t]) {
					logger.V(1).Error(fmt.Errorf("missing ring members"), "missing ring members", "txid", bl.Tx_hashes[i].String(), "expected", int(tx.Payloads[t].Statement.RingSize), "got", len(tx_result.Txs[t].Ring))
					continue
				}

				if tx.Payloads[t].SCID != scid { // skip tokens in which we are not interested
					continue
				}

				previous_balance = w.DecodeEncryptedBalanceNow(previous_balance_e_tx)

				for j := 0; j < int(tx.Payloads[t].Statement.RingSize); j++ { // first fill in all the ring members

					var addr *rpc.Address
					if addr, err = rpc.NewAddress(tx_result.Txs[0].Ring[t][j]); err != nil {
						panic(err)
					}
					var buf [33]byte
					copy(buf[:], addr.PublicKey.EncodeCompressed())
					tx.Payloads[t].Statement.Publickeylist_compressed = append(tx.Payloads[t].Statement.Publickeylist_compressed, buf)
				}

				for j := 0; j < int(tx.Payloads[t].Statement.RingSize); j++ { // check whether statement has public key

					// check whether our address is a ring member if yes, process it as ours
					if bytes.Compare(compressed_address, tx.Payloads[t].Statement.Publickeylist_compressed[j][:]) == 0 {

						// this tx contains us either as a ring member, or sender or receiver, so add all  members as ring members for future
						// keep collecting ring members to make things exponentially complex

						for k := range tx.Payloads[t].Statement.Publickeylist_compressed {
							var p bn256.G1
							if err = p.DecodeCompressed(tx.Payloads[t].Statement.Publickeylist_compressed[k][:]); err != nil {
								fmt.Printf("key could not be decompressed")

							} else {
								tx.Payloads[t].Statement.Publickeylist = append(tx.Payloads[t].Statement.Publickeylist, &p)
							}
						}

						/*for k := range tx.Statement.Publickeylist_compressed {
							if j != k {
								ringmember := address.NewAddressFromKeys((*crypto.Point)(tx.Statement.Publickeylist[k]))
								ringmember.Mainnet = w.GetNetwork()
								w.account.RingMembers[ringmember.String()] = 1
							}
						}*/

						changes := crypto.ConstructElGamal(tx.Payloads[t].Statement.C[j], tx.Payloads[t].Statement.D)
						changed_balance_e := previous_balance_e_tx.Add(changes)

						previous_balance_e_tx = new(crypto.ElGamal).Deserialize(changed_balance_e.Serialize())

						changed_balance := w.DecodeEncryptedBalance_Memory(changed_balance_e, previous_balance)

						//fmt.Printf("%d changed_balance %d previous_balance %d len payload %d\n", t, changed_balance, previous_balance, len(tx.Payloads[t].RPCPayload))

						entry := rpc.Entry{Height: bl.Height, Pos: t, TopoHeight: topo, BlockHash: bl.GetHash().String(), TransactionPos: i, TXID: tx.GetHash().String(), Time: time.UnixMilli(int64(bl.Timestamp)), Fees: tx.Fees()}

						entry.EWData = EWData
						ring_member := false

						switch {
						case previous_balance == changed_balance: //ring member/* handle 0 value tx but fees is deducted */
							//fmt.Printf("Anon Ring Member in TX %s\n", bl.Tx_hashes[i].String())
							ring_member = true
						case previous_balance > changed_balance: // we generated this tx
							entry.Burn = tx.Payloads[t].BurnValue
							entry.Amount = previous_balance - changed_balance - (tx.Payloads[t].Statement.Fees)
							entry.Fees = tx.Payloads[t].Statement.Fees
							entry.Status = 1                        // mark it as spend
							total_sent += entry.Amount + entry.Fees // burn is in amount

							rinputs := append([]byte{}, tx.Payloads[t].Statement.Roothash[:]...)
							for l := range tx.Payloads[t].Statement.Publickeylist_compressed {
								rinputs = append(rinputs, tx.Payloads[t].Statement.Publickeylist_compressed[l][:]...)
							}
							rencrypted := new(bn256.G1).ScalarMult(crypto.HashToPoint(crypto.HashtoNumber(append([]byte(crypto.PROTOCOL_CONSTANT), rinputs...))), w.account.Keys.Secret.BigInt())
							r := crypto.ReducedHash(rencrypted.EncodeCompressed())

							//fmt.Printf("t %d r  calculated %s value amount %d burn %d\n", t, r.Text(16), entry.Amount,entry.Burn)

							parity := tx.Payloads[t].Proof.Parity()

							// lets separate ring members
							for k := range tx.Payloads[t].Statement.C {
								if (k%2 == 0) == parity { // ignore senders self,this condition is well thought out and works good enough
									continue
								}

								// we need to brute force receiver in this case, if amount sent is zero
								if entry.Amount == entry.Burn && tx.Payloads[t].RPCType == transaction.ENCRYPTED_DEFAULT_PAYLOAD_CBOR {
									shared_key := crypto.GenerateSharedSecret(r, tx.Payloads[t].Statement.Publickeylist[k])

									var data_copy []byte
									data_copy = append(data_copy, tx.Payloads[t].RPCPayload...)
									crypto.EncryptDecryptUserData(crypto.Keccak256(shared_key[:], tx.Payloads[t].Statement.Publickeylist[k].EncodeCompressed()), data_copy)

									var args rpc.Arguments
									if err = args.UnmarshalBinary(data_copy[1:]); err != nil {
										//fmt.Printf("k %d len(data_copy) %d err %s data_copy %x\n",k, len(data_copy),err, data_copy)
										continue
									}

									// we have found one which could be decoded, fall through
								}

								var x bn256.G1
								x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(entry.Amount-entry.Burn)))
								x.Add(new(bn256.G1).Set(&x), new(bn256.G1).ScalarMult(tx.Payloads[t].Statement.Publickeylist[k], r))

								if x.String() == tx.Payloads[t].Statement.C[k].String() {

									var x bn256.G1
									x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(0-entry.Amount)))
									x.Add(new(bn256.G1).Set(&x), tx.Payloads[t].Statement.C[k]) // get the blinder
									blinder := &x

									shared_key := crypto.GenerateSharedSecret(r, tx.Payloads[t].Statement.Publickeylist[k])

									// proof is blinder + amount transferred, it will recover the encrypted rpc payload also
									// enable sender side proofs
									proof := rpc.NewAddressFromKeys((*crypto.Point)(blinder))
									proof.Proof = true
									proof.Arguments = rpc.Arguments{{Name: "H", DataType: rpc.DataHash, Value: crypto.Hash(shared_key)}, {Name: rpc.RPC_VALUE_TRANSFER, DataType: rpc.DataUint64, Value: uint64(entry.Amount - entry.Burn)}}
									entry.Proof = proof.String()
									entry.PayloadType = tx.Payloads[t].RPCType
									switch tx.Payloads[t].RPCType {

									case transaction.ENCRYPTED_DEFAULT_PAYLOAD_CBOR:

										crypto.EncryptDecryptUserData(crypto.Keccak256(shared_key[:], tx.Payloads[t].Statement.Publickeylist[k].EncodeCompressed()), tx.Payloads[t].RPCPayload)
										//fmt.Printf("decoded plaintext payload t %d  %x\n",t,tx.Payloads[t].RPCPayload)
										//sender_idx := uint(tx.Payloads[t].RPCPayload[0])

										addr := rpc.NewAddressFromKeys((*crypto.Point)(w.account.Keys.Public.G1()))
										addr.Mainnet = w.GetNetwork()
										entry.Sender = addr.String()

										entry.Payload = append(entry.Payload, tx.Payloads[t].RPCPayload[1:]...)
										entry.Data = append(entry.Data, tx.Payloads[t].RPCPayload[:]...)

										args, _ := entry.ProcessPayload()
										_ = args

									//	fmt.Printf("data received %s idx %d arguments %s\n", string(entry.Payload), sender_idx, args)

									default:
										entry.PayloadError = fmt.Sprintf("unknown payload type %d", tx.Payloads[t].RPCType)
										entry.Payload = tx.Payloads[t].RPCPayload
									}

									//paymentID := binary.BigEndian.Uint64(payment_id_encrypted_bytes[:]) // get decrypted payment id

									addr := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[k]))
									addr.Mainnet = w.GetNetwork()

									entry.Destination = addr.String()

									//fmt.Printf("t %d height %d Sent funds to %s entry %+v\n",t, tx.Height, addr.String(), entry)
									break

								}

							}

						case previous_balance < changed_balance: // someone sentus this amount
							entry.Amount = changed_balance - previous_balance
							entry.Incoming = true

							// we should decode the payment id
							var x bn256.G1
							x.ScalarMult(crypto.G, new(big.Int).SetInt64(0-int64(entry.Amount))) // decrease amounts
							x.Add(new(bn256.G1).Set(&x), tx.Payloads[t].Statement.C[j])          // get the blinder

							blinder := &x

							shared_key := crypto.GenerateSharedSecret(w.account.Keys.Secret.BigInt(), tx.Payloads[t].Statement.D)

							// enable receiver side proofs
							proof := rpc.NewAddressFromKeys((*crypto.Point)(blinder))
							proof.Proof = true
							proof.Arguments = rpc.Arguments{{Name: "H", DataType: rpc.DataHash, Value: crypto.Hash(shared_key)}, {Name: rpc.RPC_VALUE_TRANSFER, DataType: rpc.DataUint64, Value: uint64(entry.Amount)}}
							entry.Proof = proof.String()

							entry.PayloadType = tx.Payloads[t].RPCType
							switch tx.Payloads[t].RPCType {

							case 0:

								//fmt.Printf("decoding encrypted payload %x\n",tx.Payloads[t].RPCPayload)
								crypto.EncryptDecryptUserData(crypto.Keccak256(shared_key[:], w.GetAddress().PublicKey.EncodeCompressed()), tx.Payloads[t].RPCPayload)
								//fmt.Printf("decoded plaintext payload %x\n",tx.Payloads[t].RPCPayload)
								sender_idx := uint(tx.Payloads[t].RPCPayload[0])
								// if ring size is 2, the other party is the sender so mark it so
								if uint(tx.Payloads[t].Statement.RingSize) == 2 {
									sender_idx = 0
									if j == 0 {
										sender_idx = 1
									}
								}

								if sender_idx <= uint(tx.Payloads[t].Statement.RingSize) {
									addr := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[sender_idx]))
									addr.Mainnet = w.GetNetwork()
									entry.Sender = addr.String()
								}

								entry.Payload = append(entry.Payload, tx.Payloads[t].RPCPayload[1:]...)
								entry.Data = append(entry.Data, tx.Payloads[t].RPCPayload[:]...)

								args, _ := entry.ProcessPayload()
								_ = args

							//	fmt.Printf("data received %s idx %d arguments %s\n", string(entry.Payload), sender_idx, args)

							default:
								entry.PayloadError = fmt.Sprintf("unknown payload type %d", tx.Payloads[t].RPCType)
								entry.Payload = tx.Payloads[t].RPCPayload
							}

							//fmt.Printf("Received %s amount in TX(%d) %s payment id %x Src_ID %s data %s\n", globals.FormatMoney(changed_balance-previous_balance), tx.Height, bl.Tx_hashes[i].String(),  entry.PaymentID, tx.Src_ID, tx.Data)
							//fmt.Printf("Received  amount in TX(%d) %s payment id %x Src_ID %s data %s\n",  tx.Height, bl.Tx_hashes[i].String(),  entry.PaymentID, tx.SrcID, tx.Data)
							total_received += (changed_balance - previous_balance)
						}

						if !ring_member { // do not book keep ring members
							local_entries = append(local_entries, entry)
						}

						//break // this tx has been processed so skip it

					}
				}
			}

			//fmt.Printf("block %d   %+v\n", topo, tx_result)
		}
	}

	previous_balance = w.DecodeEncryptedBalance_Memory(previous_balance_e, 0)
	coinbase_reward := current_balance - (previous_balance - total_sent + total_received)

	//fmt.Printf("ht %d coinbase_reward %d   curent balance %d previous_balance %d sent %d received %d\n", bl.Height, coinbase_reward, current_balance, previous_balance, total_sent, total_received)

	if bytes.Compare(compressed_address, bl.Miner_TX.MinerAddress[:]) == 0 || coinbase_reward > 0 { // wallet user  has minted a block
		entry := rpc.Entry{Height: bl.Height, TopoHeight: topo, BlockHash: bl.GetHash().String(), TransactionPos: -1, Time: time.UnixMilli(int64(bl.Timestamp))}

		entry.EWData = EWData
		entry.Amount = current_balance - (previous_balance - total_sent + total_received)
		entry.Coinbase = true
		local_entries = append([]rpc.Entry{entry}, local_entries...)

		//fmt.Printf("Coinbase Reward %s for block %d\n", globals.FormatMoney(current_balance-(previous_balance-total_sent+total_received)), topo)
	}

	for _, e := range local_entries {
		w.InsertReplace(scid, e)
	}

	if len(local_entries) >= 1 {
		w.save_if_disk() // save wallet()
		//	w.db.Sync()
	}

	return nil

}
