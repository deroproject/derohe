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
import "fmt"
import "net"
import "time"
import "sync"
import "bytes"
import "math/big"

//import "net/url"
import "net/http"

//import "bufio"
import "strings"
import "context"

//import "runtime"
//import "compress/gzip"
import "encoding/hex"
import "encoding/binary"

import "runtime/debug"

import "github.com/romana/rlog"

//import "github.com/pierrec/lz4"
import "github.com/ybbus/jsonrpc"

//import "github.com/vmihailenco/msgpack"

//import "github.com/gorilla/websocket"
//import "github.com/mafredri/cdp/rpcc"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/address"
import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/structures"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/crypto/bn256"
import "github.com/deroproject/derohe/glue/rwc"

import "github.com/creachadair/jrpc2"
import "github.com/creachadair/jrpc2/channel"
import "github.com/gorilla/websocket"

// this global variable should be within wallet structure
var Connected bool = false

// there should be no global variables, so multiple wallets can run at the same time with different assset
var rpcClient *jsonrpc.RPCClient
var netClient *http.Client
var endpoint string

var output_lock sync.Mutex

type Client struct {
	WS  *websocket.Conn
	RPC *jrpc2.Client
}

var rpc_client = &Client{}

func (cli *Client) Call(method string, params interface{}, result interface{}) error {
	return cli.RPC.CallResult(context.Background(), method, params, result)
}

// returns whether wallet was online some time ago
func (w *Wallet) IsDaemonOnlineCached() bool {
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
func (w *Wallet) IsDaemonOnline() bool {
	if rpc_client.WS == nil || rpc_client.RPC == nil {
		return false
	}
	return true
}

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func (w *Wallet) Connect() (err error) {

	if globals.Arguments["--remote"] == true && globals.IsMainnet() {
		w.Daemon_Endpoint = config.REMOTE_DAEMON
	}

	// if user provided endpoint has error, use default
	if w.Daemon_Endpoint == "" {
		w.Daemon_Endpoint = "127.0.0.1:" + fmt.Sprintf("%d", config.Mainnet.RPC_Default_Port)
		if !globals.IsMainnet() {
			w.Daemon_Endpoint = "127.0.0.1:" + fmt.Sprintf("%d", config.Testnet.RPC_Default_Port)
		}
	}

	if globals.Arguments["--daemon-address"] != nil {
		w.Daemon_Endpoint = globals.Arguments["--daemon-address"].(string)
	}

	rlog.Infof("Daemon endpoint %s", w.Daemon_Endpoint)

	// TODO enable socks support here
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second, // 5 second timeout
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	netClient = &http.Client{
		Timeout:   time.Second * 10,
		Transport: netTransport,
	}

	//rpc_conn, err = rpcc.Dial("ws://"+ w.Daemon_Endpoint + "/ws")

	rpc_client.WS, _, err = websocket.DefaultDialer.Dial("ws://"+w.Daemon_Endpoint+"/ws", nil)

	// notify user of any state change
	// if daemon connection breaks or comes live again
	if err == nil {
		if !Connected {
			rlog.Infof("Connection to RPC server successful %s", "ws://"+w.Daemon_Endpoint+"/ws")
			Connected = true
		}
	} else {
		rlog.Errorf("Error executing getinfo_rpc err %s", err)

		if Connected {
			rlog.Warnf("Connection to RPC server Failed err %s endpoint %s ", err, "ws://"+w.Daemon_Endpoint+"/ws")
		}
		Connected = false

		return
	}

	input_output := rwc.New(rpc_client.WS)
	rpc_client.RPC = jrpc2.NewClient(channel.RawJSON(input_output, input_output), nil)

	var result string

	// Issue a call with a response.
	if err = rpc_client.Call("DERO.Echo", []string{"hello", "world"}, &result); err != nil {
		rlog.Warnf("DERO.Echo Call failed: %v", err)
		Connected = false
		return
	}
	//fmt.Println(result)

	var info structures.GetInfo_Result
	// Issue a call with a response.
	if err = rpc_client.Call("DERO.GetInfo", nil, &info); err != nil {
		rlog.Warnf("DERO.GetInfo Call failed: %v", err)
		Connected = false
		return
	}

	// detect whether both are in different modes
	//  daemon is in testnet and wallet in mainnet or
	// daemon
	if info.Testnet != !globals.IsMainnet() {
		err = fmt.Errorf("Mainnet/TestNet  is different between wallet/daemon.Please run daemon/wallet without --testnet")
		rlog.Criticalf("%s", err)
		return
	}

	w.random_ring_members()

	w.Lock()
	defer w.Unlock()

	if info.Height >= 0 {
		w.Daemon_Height = uint64(info.Height)
		w.Daemon_TopoHeight = info.TopoHeight
		w.Merkle_Balance_TreeHash = info.Merkle_Balance_TreeHash
	}
	w.dynamic_fees_per_kb = info.Dynamic_fee_per_kb // set fee rate, it can work for quite some time,

	//  fmt.Printf("merkle tree %+v\n", info);

	return nil
}

/*
func (cli *Client)onlinecheck_and_get_online(){
    for {
        if cli.IsDaemonOnline() {
            var result string
	        if err := cli.Call( "DERO.Ping", nil, &result); err != nil {
		       // fmt.Printf("Ping failed: %v", err)
                cli.RPC.Close()
                cli.WS = nil
                cli.RPC = nil
                Connect() // try to connect again
	        }else{
		        //fmt.Printf("Ping Received %s\n", result)
	        }
        }
        time.Sleep(time.Second)
    }
}
*/

// get the outputs from the daemon, requesting specfic outputs
// the range can be anything
// if stop is zero,
// the daemon will flush out everything it has ASAP
// the stream can be saved and used later on

func (w *Wallet) Sync_Wallet_With_Daemon() {

	//fmt.Printf("syncing with wallet started\n")

	if !w.IsDaemonOnline() {
		return
	}
	output_lock.Lock()
	defer output_lock.Unlock()

	//	fmt.Printf("account %+v\n", w.account)

	// only sync if both height are different
	//if w.Daemon_TopoHeight == w.account.TopoHeight && w.account.TopoHeight != 0 { // wallet is already synced
	//	return
	//}

	//w.Daemon_State_Version = ""

	w.random_ring_members()

	rlog.Infof("wallet topo height %d daemon online topo height %d\n", w.account.TopoHeight, w.Daemon_TopoHeight)

	previous := w.account.Balance_Result.Data

	if _, err := w.GetEncryptedBalance("", w.GetAddress().String()); err != nil {
		return
	}

	if w.account.Balance_Result.Data != previous || (len(w.account.Entries) >= 1 && strings.ToLower(w.account.Balance_Result.Data) != strings.ToLower(w.account.Entries[len(w.account.Entries)-1].EWData)) {
		w.DecodeEncryptedBalance() // try to decode balance
		w.SyncHistory()            // also update statement
	}

	return
}

// triggers syncing with wallet every 5 seconds
func (w *Wallet) sync_loop() {

	for {

		if !Connected {
			w.Connect()
		} else {
			if w.IsDaemonOnline() {
				var result string
				if err := rpc_client.Call("DERO.Ping", nil, &result); err != nil {
					// fmt.Printf("Ping failed: %v", err)
					rpc_client.RPC.Close()
					rpc_client.WS = nil
					rpc_client.RPC = nil
					w.Connect() // try to connect again

				} else {
					//fmt.Printf("Ping Received %s\n", result)
				}
			}
		}

		if w.IsDaemonOnline() { // could not connect try again after 5 secs
			w.Sync_Wallet_With_Daemon() // sync with the daemon
		}

		select { // quit midway if required
		case <-w.quit:
			return
		case <-time.After(5 * time.Second):
		}

		if !w.wallet_online_mode { // wallet requested to be in offline mode
			return
		}
	}
}

func (w *Wallet) Rescan_From_Height(startheight uint64) {
	panic("not implemented")

}

// this is as simple as it gets
// single threaded communication to relay TX to daemon
// if this is successful, then daemon is in control

func (w *Wallet) SendTransaction(tx *transaction.Transaction) (err error) {

	if tx == nil {
		return fmt.Errorf("Can not send nil transaction")
	}

	if !w.IsDaemonOnline() {
		return fmt.Errorf("offline or not connected. cannot send transaction.")
	}

	params := structures.SendRawTransaction_Params{Tx_as_hex: hex.EncodeToString(tx.Serialize())}
	var result structures.SendRawTransaction_Result

	// Issue a call with a response.
	if err := rpc_client.Call("DERO.SendRawTransaction", params, &result); err != nil {
		return err
	}

	//fmt.Printf("raw transaction result %+v\n", result)

	if result.Status == "OK" {
		return nil
	} else {
		err = fmt.Errorf("Err %s", result.Status)
	}

	//fmt.Printf("err in response %+v", result)

	return
}

// this is as simple as it gets
// single threaded communication  gets whether the the key image is spent in pool or in blockchain
// this can leak informtion which keyimage belongs to us
// TODO in order to stop privacy leaks we must guess this information somehow on client side itself
// maybe the server can broadcast a bloomfilter or something else from the mempool keyimages
//
func (w *Wallet) GetEncryptedBalance(treehash string, accountaddr string) (e *crypto.ElGamal, err error) {

	defer func() {
		if r := recover(); r != nil {
			rlog.Warnf("Stack trace  \n%s", debug.Stack())

		}
	}()

	if !w.GetMode() { // if wallet is in offline mode , we cannot do anything
		err = fmt.Errorf("wallet is in offline mode")
		return
	}

	if !w.IsDaemonOnline() {
		err = fmt.Errorf("offline or not connected")
		return
	}

	//var params structures.GetEncryptedBalance_Params
	var result structures.GetEncryptedBalance_Result

	// Issue a call with a response.
	if err = rpc_client.Call("DERO.GetEncryptedBalance", structures.GetEncryptedBalance_Params{Address: accountaddr, TopoHeight: -1}, &result); err != nil {

		rlog.Warnf("GetEncryptedBalance err %s", err)

		if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(errormsg.ErrAccountUnregistered.Error())) && accountaddr == w.GetAddress().String() {
			w.Error = errormsg.ErrAccountUnregistered
		}
		return
	}

	//fmt.Printf("result %+v\n", result)
	w.Daemon_Height = uint64(result.DHeight)
	w.Daemon_TopoHeight = result.DTopoheight
	w.Merkle_Balance_TreeHash = result.DMerkle_Balance_TreeHash

	if accountaddr == w.GetAddress().String() {
		w.account.Balance_Result = result
		w.account.TopoHeight = result.Topoheight
	}

	//fmt.Printf("status '%s' err '%s'  %+v  %+v \n", result.Status , w.Error , result.Status == errormsg.ErrAccountUnregistered.Error()  , accountaddr == w.account.GetAddress().String())

	if result.Status != "OK" {
		err = fmt.Errorf("%s", result.Status)
		return
	}

	hexdecoded, err := hex.DecodeString(result.Data)
	if err != nil {
		return
	}

	if accountaddr == w.GetAddress().String() {
		w.Error = nil
	}

	el := new(crypto.ElGamal).Deserialize(hexdecoded)
	return el, nil
}

func (w *Wallet) DecodeEncryptedBalance() (err error) {

	var el crypto.ElGamal
	var balance_point bn256.G1

	hexdecoded, err := hex.DecodeString(w.account.Balance_Result.Data)
	if err != nil {
		return
	}

	el = *el.Deserialize(hexdecoded)
	if err != nil {
		panic(err)
		return
	}

	balance_point.Add(el.Left, new(bn256.G1).Neg(new(bn256.G1).ScalarMult(el.Right, w.account.Keys.Secret.BigInt())))

	w.account.Balance_Mature = Balance_lookup_table.Lookup(&balance_point, w.account.Balance_Mature)

	return nil
}

// this is as simple as it gets
// single threaded communication  gets whether the the key image is spent in pool or in blockchain
// this can leak informtion which keyimage belongs to us
// TODO in order to stop privacy leaks we must guess this information somehow on client side itself
// maybe the server can broadcast a bloomfilter or something else from the mempool keyimages
//
func (w *Wallet) GetEncryptedBalanceAtTopoHeight(topoheight int64, accountaddr string) (e *crypto.ElGamal, err error) {

	defer func() {
		if r := recover(); r != nil {
			rlog.Warnf("Stack trace  \n%s", debug.Stack())

		}
	}()

	if !w.GetMode() { // if wallet is in offline mode , we cannot do anything
		err = fmt.Errorf("wallet is in offline mode")
		return
	}

	if !w.IsDaemonOnline() {
		err = fmt.Errorf("offline or not connected")
		return
	}

	//var params structures.GetEncryptedBalance_Params
	var result structures.GetEncryptedBalance_Result

	// Issue a call with a response.
	if err := rpc_client.Call("DERO.GetEncryptedBalance", structures.GetEncryptedBalance_Params{Address: accountaddr, TopoHeight: topoheight}, &result); err != nil {
		fmt.Printf("Call failed: %v", err)
	}

	//	fmt.Printf("encrypted_balance %+v\n", result)
	/*

		response, err := rpcClient.CallNamed("getencryptedbalance", map[string]interface{}{"address": accountaddr, "treehash":treehash,})
			if err != nil {
				rlog.Errorf("getencryptedbalance call Failed err %s", err)
				return
			}


			// parse response
			if response.Error != nil {
				rlog.Errorf("getencryptedbalance Failed err %s", response.Error)
				return
			}

			err = response.GetObject(&result)
			if err != nil {
				return // err
			}
	*/
	if result.Status == errormsg.ErrAccountUnregistered.Error() && accountaddr == w.GetAddress().String() {
		w.Error = errormsg.ErrAccountUnregistered
	}

	//	fmt.Printf("status '%s' err '%s'  %+v  %+v \n", result.Status , w.Error , result.Status == errormsg.ErrAccountUnregistered.Error()  , accountaddr == w.account.GetAddress().String())

	if result.Status == errormsg.ErrAccountUnregistered.Error() {
		err = fmt.Errorf("%s", result.Status)
		return
	}

	if result.Status != "OK" {
		err = fmt.Errorf("%s", result.Status)
		return
	}

	hexdecoded, err := hex.DecodeString(result.Data)
	if err != nil {
		return
	}

	if accountaddr == w.GetAddress().String() {
		w.Error = nil
	}

	el := new(crypto.ElGamal).Deserialize(hexdecoded)
	return el, nil
}

func (w *Wallet) DecodeEncryptedBalance_Memory(el *crypto.ElGamal, hint uint64) (balance uint64) {

	var balance_point bn256.G1

	balance_point.Add(el.Left, new(bn256.G1).Neg(new(bn256.G1).ScalarMult(el.Right, w.account.Keys.Secret.BigInt())))

	return Balance_lookup_table.Lookup(&balance_point, hint)
}

func (w *Wallet) GetDecryptedBalanceAtTopoHeight(topoheight int64, accountaddr string) (balance uint64, err error) {
	encrypted_balance, err := w.GetEncryptedBalanceAtTopoHeight(topoheight, accountaddr)
	if err != nil {
		return 0, err
	}

	return w.DecodeEncryptedBalance_Memory(encrypted_balance, 0), nil
}

// sync history of wallet from blockchain
func (w *Wallet) random_ring_members() {

	//fmt.Printf("getting random_ring_members\n")

	if len(w.account.RingMembers) > 300 { // unregistered so skip
		return
	}

	var result structures.GetRandomAddress_Result

	// Issue a call with a response.
	if err := rpc_client.Call("DERO.GetRandomAddress", nil, &result); err != nil {
		fmt.Printf("Call failed: %v", err)
		return
	}
	//fmt.Printf("ring members %+v\n", result)

	// we have found a matching block hash, start syncing from here
	if w.account.RingMembers == nil {
		w.account.RingMembers = map[string]int64{}
	}

	for _, k := range result.Address {
		if k != w.GetAddress().String() {
			w.account.RingMembers[k] = 1
		}
	}
	return
}

// sync history of wallet from blockchain
func (w *Wallet) SyncHistory() (balance uint64) {
	if w.account.Balance_Result.Registration < 0 { // unregistered so skip
		return
	}

	last_topo_height := int64(-1)

	//fmt.Printf("finding sync point  ( Registration point %d)\n", w.account.Balance_Result.Registration)

	// we need to find a sync point, to minimize traffic
	for i := len(w.account.Entries) - 1; i >= 0; {

		// below condition will trigger if chain got pruned on server
		if w.account.Balance_Result.Registration >= w.account.Entries[i].TopoHeight { // keep old history if chain got pruned
			break
		}
		if last_topo_height == w.account.Entries[i].TopoHeight {
			i--
		} else {

			last_topo_height = w.account.Entries[i].TopoHeight

			var result structures.GetBlockHeaderByHeight_Result

			// Issue a call with a response.
			if err := rpc_client.Call("DERO.GetBlockHeaderByTopoHeight", structures.GetBlockHeaderByTopoHeight_Params{TopoHeight: uint64(w.account.Entries[i].TopoHeight)}, &result); err != nil {
				fmt.Printf("Call failed: %v", err)
				return 0
			}

			if i >= 1 && last_topo_height == w.account.Entries[i-1].TopoHeight { // skipping any entries withing same block
				for ; i >= 1; i-- {
					if last_topo_height != w.account.Entries[i-1].TopoHeight {
						w.account.Entries = w.account.Entries[:i]
					}
				}
			}

			if i == 0 {
				w.account.Entries = w.account.Entries[:0] // discard all entries
				break
			}

			// we have found a matching block hash, start syncing from here
			if result.Status == "OK" && result.Block_Header.Hash == w.account.Entries[i].BlockHash {
				w.synchistory_internal(w.account.Entries[i].TopoHeight+1, w.account.Balance_Result.Topoheight)
				return
			}

		}

	}

	//fmt.Printf("syncing loop using Registration %d\n", w.account.Balance_Result.Registration)

	// if we reached here, means we should sync from scratch
	w.synchistory_internal(w.account.Balance_Result.Registration, w.account.Balance_Result.Topoheight)

	//if w.account.Registration >= 0 {
	// err :=
	// err =  w.synchistory_internal(w.account.Registration,6)

	// }
	// fmt.Printf("syncing err %s\n",err)
	// fmt.Printf("entries %+v\n", w.account.Entries)

	return 0
}

// sync history
func (w *Wallet) synchistory_internal(start_topo, end_topo int64) error {

	var err error
	var start_balance_e *crypto.ElGamal
	if start_topo == w.account.Balance_Result.Registration {
		start_balance_e = crypto.ConstructElGamal(w.account.Keys.Public.G1(), crypto.ElGamal_BASE_G)
	} else {
		start_balance_e, err = w.GetEncryptedBalanceAtTopoHeight(start_topo, w.GetAddress().String())
		if err != nil {
			return err
		}
	}

	end_balance_e, err := w.GetEncryptedBalanceAtTopoHeight(end_topo, w.GetAddress().String())
	if err != nil {
		return err
	}

	return w.synchistory_internal_binary_search(start_topo, start_balance_e, end_topo, end_balance_e)

}

func (w *Wallet) synchistory_internal_binary_search(start_topo int64, start_balance_e *crypto.ElGamal, end_topo int64, end_balance_e *crypto.ElGamal) error {

	//fmt.Printf("end %d start %d\n", end_topo, start_topo)

	if end_topo < 0 {
		return fmt.Errorf("done")
	}

	/*	if bytes.Compare(start_balance_e.Serialize(), end_balance_e.Serialize()) == 0 {
		    return nil
		}
	*/

	//for start_topo <= end_topo{
	{
		median := (start_topo + end_topo) / 2

		//fmt.Printf("low %d high %d median %d\n", start_topo,end_topo,median)

		if start_topo == median {
			//fmt.Printf("syncing block %d\n", start_topo)
			err := w.synchistory_block(start_topo)
			if err != nil {
				return err
			}
		}

		if end_topo-start_topo <= 1 {
			return w.synchistory_block(end_topo)
		}

		median_balance_e, err := w.GetEncryptedBalanceAtTopoHeight(median, w.GetAddress().String())
		if err != nil {
			return err
		}

		// check if there is a change in lower section, if yes process more
		if start_topo == w.account.Balance_Result.Registration || bytes.Compare(start_balance_e.Serialize(), median_balance_e.Serialize()) != 0 {
			//fmt.Printf("lower\n")
			err = w.synchistory_internal_binary_search(start_topo, start_balance_e, median, median_balance_e)
			if err != nil {
				return err
			}
		}

		// check if there is a change in higher section, if yes process more
		if bytes.Compare(median_balance_e.Serialize(), end_balance_e.Serialize()) != 0 {
			//fmt.Printf("higher\n")
			err = w.synchistory_internal_binary_search(median, median_balance_e, end_topo, end_balance_e)
			if err != nil {
				return err
			}
		}

		/*if IsRegisteredAtTopoHeight (addr,median) {
		            high = median - 1
				}else{
					low = median + 1
				}*/

	}

	/*
	       if end_topo - start_topo <= 1 {
	   		err :=  w.synchistory_block(start_topo)
	   		if err != nil {
	   			return err
	   		}
	   		return w.synchistory_block(end_topo)
	   	}


	       // this means the address is either a ring member or a sender or a receiver in atleast one of the blocks
	       middle :=  start_topo +  (end_topo-start_topo)/2
	       middle_balance_e, err := w.GetEncryptedBalanceAtTopoHeight( middle ,w.account.GetAddress().String())
	   	if err != nil {
	   		return err
	   	}

	   	// check if there is a change in lower section, if yes process more
	   	if bytes.Compare(start_balance_e.Serialize(), middle_balance_e.Serialize()) != 0 {
	   		fmt.Printf("lower\n")
	           err = w.synchistory_internal_binary_search(start_topo,start_balance_e, middle, middle_balance_e )
	           if err != nil {
	           	return err
	           }
	       }

	       // check if there is a change in lower section, if yes process more
	       if bytes.Compare(middle_balance_e.Serialize(), end_balance_e.Serialize()) != 0 {
	       	fmt.Printf("higher\n")
	           err = w.synchistory_internal_binary_search(middle, middle_balance_e, end_topo,end_balance_e )
	           if err != nil {
	           	return err
	           }
	       }
	*/

	return nil
}

// extract history from a single block
// first get a block, then get all the txs
// Todo we should expose an API to get all txs which have the specific address as ring member
// for a particular block
// for the entire chain
func (w *Wallet) synchistory_block(topo int64) (err error) {

	var local_entries []Entry

	compressed_address := w.account.Keys.Public.EncodeCompressed()

	var previous_balance_e, current_balance_e *crypto.ElGamal
	var previous_balance, current_balance, total_sent, total_received uint64

	if topo <= 0 || w.account.Balance_Result.Registration == topo {
		previous_balance_e = crypto.ConstructElGamal(w.account.Keys.Public.G1(), crypto.ElGamal_BASE_G)
	} else {
		previous_balance_e, err = w.GetEncryptedBalanceAtTopoHeight(topo-1, w.GetAddress().String())
		if err != nil {
			return err
		}
	}

	current_balance_e, err = w.GetEncryptedBalanceAtTopoHeight(topo, w.GetAddress().String())
	if err != nil {
		return err
	}

	EWData := fmt.Sprintf("%x", current_balance_e.Serialize())

	previous_balance = w.DecodeEncryptedBalance_Memory(previous_balance_e, 0)
	current_balance = w.DecodeEncryptedBalance_Memory(current_balance_e, 0)

	// we can skip some check if both balances are equal ( means we are ring members in this block)
	// this check will also fail if we total spend == total receivein the block
	// currently it is not implmented, and we bruteforce everything

	_ = current_balance

	var bl block.Block
	var bresult structures.GetBlock_Result
	if err = rpc_client.Call("DERO.GetBlock", structures.GetBlock_Params{Height: uint64(topo)}, &bresult); err != nil {
		return fmt.Errorf("getblock rpc failed")
	}

	block_bin, _ := hex.DecodeString(bresult.Blob)
	bl.Deserialize(block_bin)

	if len(bl.Tx_hashes) >= 1 {

		//fmt.Printf("Requesting tx data %s", txhash);

		for i := range bl.Tx_hashes {
			var tx transaction.Transaction

			var tx_params structures.GetTransaction_Params
			var tx_result structures.GetTransaction_Result

			tx_params.Tx_Hashes = append(tx_params.Tx_Hashes, bl.Tx_hashes[i].String())

			if err = rpc_client.Call("DERO.GetTransaction", tx_params, &tx_result); err != nil {
				return fmt.Errorf("gettransa rpc failed %s", err)
			}

			tx_bin, err := hex.DecodeString(tx_result.Txs_as_hex[0])
			if err != nil {
				return err
			}
			tx.DeserializeHeader(tx_bin)

			for j := range tx.Statement.Publickeylist_compressed { // check whether statement has public key

				// check whether our address is a ring member if yes, process it as ours
				if bytes.Compare(compressed_address, tx.Statement.Publickeylist_compressed[j][:]) == 0 {

					// this tx contains us either as a ring member, or sender or receiver, so add all  members as ring members for future
					// keep collecting ring members to make things exponentially complex
					for k := range tx.Statement.Publickeylist_compressed {
						if j != k {
							ringmember := address.NewAddressFromKeys((*crypto.Point)(tx.Statement.Publickeylist[k]))
							ringmember.Mainnet = w.GetNetwork()
							w.account.RingMembers[ringmember.String()] = 1
						}
					}

					changes := crypto.ConstructElGamal(tx.Statement.C[j], tx.Statement.D)
					changed_balance_e := previous_balance_e.Add(changes)

					changed_balance := w.DecodeEncryptedBalance_Memory(changed_balance_e, previous_balance)

					entry := Entry{Height: bl.Height, TopoHeight: topo, BlockHash: bl.GetHash().String(), TransactionPos: i, TXID: tx.GetHash(), Time: time.Unix(int64(bl.Timestamp), 0)}

					entry.EWData = EWData
					ring_member := false

					switch {
					case previous_balance == changed_balance: //ring member/* handle 0 value tx but fees is deducted */
						//fmt.Printf("Anon Ring Member in TX %s\n", bl.Tx_hashes[i].String())
						ring_member = true
					case previous_balance > changed_balance: // we generated this tx
						entry.Amount = previous_balance - changed_balance - tx.Statement.Fees
						entry.Fees = tx.Statement.Fees
						entry.Status = 1 // mark it as spend
						total_sent += (previous_balance - changed_balance) + tx.Statement.Fees

						rinputs := append([]byte{}, tx.Statement.Roothash[:]...)
						for l := range tx.Statement.Publickeylist_compressed {
							rinputs = append(rinputs, tx.Statement.Publickeylist_compressed[l][:]...)
						}
						rencrypted := new(bn256.G1).ScalarMult(crypto.HashToPoint(crypto.HashtoNumber(append([]byte(crypto.PROTOCOL_CONSTANT), rinputs...))), w.account.Keys.Secret.BigInt())
						r := crypto.ReducedHash(rencrypted.EncodeCompressed())

						//	fmt.Printf("r  calculated %s\n", r.Text(16))
						// lets separate ring members

						for k := range tx.Statement.C {
							// skip self address, this can be optimized way more
							if tx.Statement.Publickeylist[k].String() != w.account.Keys.Public.G1().String() {
								var x bn256.G1
								x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(entry.Amount)))
								x.Add(new(bn256.G1).Set(&x), new(bn256.G1).ScalarMult(tx.Statement.Publickeylist[k], r))
								if x.String() == tx.Statement.C[k].String() {

									// lets encrypt the payment id, it's simple, we XOR the paymentID
									blinder := new(bn256.G1).ScalarMult(tx.Statement.Publickeylist[k], r)

									// proof is blinder + amount transferred, it will recover the encrypted payment id also
									proof := address.NewAddressFromKeys((*crypto.Point)(blinder))
									proof.PaymentID = make([]byte, 8, 8)
									proof.Proof = true
									binary.LittleEndian.PutUint64(proof.PaymentID, entry.Amount)
									entry.Proof = proof.String()

									entry.PaymentID = crypto.EncryptDecryptPaymentID(blinder, tx.PaymentID[:])
									//paymentID := binary.BigEndian.Uint64(payment_id_encrypted_bytes[:]) // get decrypted payment id

									addr := address.NewAddressFromKeys((*crypto.Point)(tx.Statement.Publickeylist[k]))
									addr.Mainnet = w.GetNetwork()
									//fmt.Printf("%d Sent funds to %s paymentid %x  \n", tx.Height, addr.String(), entry.PaymentID)

									entry.Details.TXID = fmt.Sprintf("%x", entry.TXID)
									entry.Details.PaymentID = fmt.Sprintf("%x", entry.PaymentID)
									entry.Details.Fees = tx.Statement.Fees
									entry.Details.Amount = append(entry.Details.Amount, entry.Amount)

									entry.Details.Daddress = append(entry.Details.Daddress, addr.String())
									break

								}

							}
						}

					case previous_balance < changed_balance: // someone sentus this amount
						entry.Amount = changed_balance - previous_balance
						entry.Incoming = true

						// we should decode the payment id
						var x bn256.G1
						x.ScalarMult(crypto.G, new(big.Int).SetInt64(0-int64(entry.Amount))) // increase receiver's balance
						x.Add(new(bn256.G1).Set(&x), tx.Statement.C[j])                      // get the blinder

						entry.PaymentID = crypto.EncryptDecryptPaymentID(&x, tx.PaymentID[:])

						//fmt.Printf("Received %s amount in TX(%d) %s payment id %x\n", globals.FormatMoney(changed_balance-previous_balance), tx.Height, bl.Tx_hashes[i].String(),  entry.PaymentID)
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

	if bytes.Compare(compressed_address, bl.Miner_TX.MinerAddress[:]) == 0 { // wallet user  has minted a block
		entry := Entry{Height: bl.Height, TopoHeight: topo, BlockHash: bl.GetHash().String(), TransactionPos: -1, Time: time.Unix(int64(bl.Timestamp), 0)}

		entry.EWData = EWData
		entry.Amount = current_balance - (previous_balance - total_sent + total_received)
		entry.Coinbase = true
		local_entries = append([]Entry{entry}, local_entries...)

		//fmt.Printf("Coinbase Reward %s for block %d\n", globals.FormatMoney(current_balance-(previous_balance-total_sent+total_received)), topo)
	}

	for _, e := range local_entries {
		w.InsertReplace(e)
	}

	if len(local_entries) >= 1 {
		w.Save_Wallet()
		//	w.db.Sync()
	}

	return nil

}
