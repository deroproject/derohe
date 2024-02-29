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

package explorerlib

// this file implements the explorer for DERO blockchain
// this needs only RPC access
// NOTE: Only use data exported from within the RPC interface, do direct use of exported variables  fom packages
// NOTE: we can use structs defined within the RPCserver package

// TODO: error handling is non-existant ( as this was built up in hrs ). Add proper error handling
//

import (
	"bytes"
	"context"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unsafe"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/deroproject/derohe/block"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/glue/rwc"
	"github.com/deroproject/derohe/proof"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
)

// need to avoid this, but only used by byteviewer

//import "encoding/json"
//import "io/ioutil"

//go:embed templates/*.tmpl
var tpls embed.FS

//go:embed static/*
var static embed.FS

type Client struct {
	WS  *websocket.Conn
	RPC *jrpc2.Client
}

var rpc_client = &Client{}
var Connected bool = false

var mainnet = true
var endpoint string
var replacer = strings.NewReplacer("h", ":", "m", ":", "s", "")
var logger logr.Logger

func (cli *Client) Call(method string, params interface{}, result interface{}) error {

	try := 0
try_again:
	if cli == nil || !cli.IsDaemonOnline() {
		go Connect()
		time.Sleep(time.Second)
		try++
		if try < 3 {
			goto try_again
		}
		return fmt.Errorf("client is offline or not connected")
	}
	return cli.RPC.CallResult(context.Background(), method, params, result)
}

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func (cli *Client) IsDaemonOnline() bool {
	if cli.WS == nil || cli.RPC == nil {
		return false
	}
	return true
}

func (cli *Client) onlinecheck_and_get_online() {

	for {
		if cli.IsDaemonOnline() {
			var result string
			if err := cli.Call("DERO.Ping", nil, &result); err != nil {
				logger.V(1).Error(err, "Ping failed:")
				cli.RPC.Close()
				cli.WS = nil
				cli.RPC = nil
				Connect() // try to connect again
			} else {
				//fmt.Printf("Ping Received %s\n", result)
			}
		}
		time.Sleep(time.Second)
	}
}

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func Connect() (err error) {
	// TODO enable socks support here

	//rpc_conn, err = rpcc.Dial("ws://"+ w.Daemon_Endpoint + "/ws")

	var daemon_uri string

	daemon_endpoint := endpoint
	//rpc_client.WS, _, err = websocket.DefaultDialer.Dial("ws://"+daemon_endpoint+"/ws", nil)

	// Trim off http, https, wss, ws to get endpoint to use for connecting
	if strings.HasPrefix(daemon_endpoint, "https") {
		ld := strings.TrimPrefix(strings.ToLower(daemon_endpoint), "https://")
		daemon_uri = "wss://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else if strings.HasPrefix(daemon_endpoint, "http") {
		ld := strings.TrimPrefix(strings.ToLower(daemon_endpoint), "http://")
		daemon_uri = "ws://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else if strings.HasPrefix(daemon_endpoint, "wss") {
		ld := strings.TrimPrefix(strings.ToLower(daemon_endpoint), "wss://")
		daemon_uri = "wss://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else if strings.HasPrefix(daemon_endpoint, "ws") {
		ld := strings.TrimPrefix(strings.ToLower(daemon_endpoint), "ws://")
		daemon_uri = "ws://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else {
		daemon_uri = "ws://" + daemon_endpoint + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	}

	// notify user of any state change
	// if daemon connection breaks or comes live again
	if err == nil {
		if !Connected {
			logger.V(1).Info("Connection to RPC server successful", "daemon_endpoint", daemon_uri)
			Connected = true
		}
	} else {
		logger.Error(err, "Error connecting to daemon")

		if Connected {
			logger.Error(err, "Connection to RPC server Failed ", "daemon_endpoint", daemon_uri)
		}
		Connected = false
		return
	}

	input_output := rwc.New(rpc_client.WS)
	rpc_client.RPC = jrpc2.NewClient(channel.RawJSON(input_output, input_output), nil)

	var result string
	if err := rpc_client.Call("DERO.Ping", nil, &result); err != nil {
		logger.V(1).Error(err, "Ping failed:")
	} else {
		//		fmt.Printf("Ping Received %s\n", result)
	}

	var info rpc.GetInfo_Result

	// collect all the data afresh,  execute rpc to service
	if err = rpc_client.Call("DERO.GetInfo", nil, &info); err != nil {
		logger.V(1).Error(err, "GetInfo failed:")
	} else {
		mainnet = !info.Testnet // inverse of testnet is mainnet
	}

	return nil
}

func StartServer(loggerb logr.Logger, daemon_endpoint string, listen_address string) (err error) {

	logger = globals.Logger.WithName("explorer")

	endpoint = daemon_endpoint
	logger.Info("Daemon RPC endpoint ", "endpoint", endpoint)
	logger.Info("Will listen ", "address", listen_address)

	// execute rpc to service
	err = Connect()

	if err == nil {
		logger.Info("Connection to RPC server successful")
	} else {
		logger.Error(err, "Connection to RPC server Failed")
		return
	}

	go rpc_client.onlinecheck_and_get_online() // keep connectingto server

	all_templates, err = template.ParseFS(tpls, "templates/*.tmpl")
	if err != nil {
		logger.Error(err, "error parsing templates.")
		return
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static)))) // include all static assets
	http.HandleFunc("/search", search_handler)
	http.HandleFunc("/page/", page_handler)
	http.HandleFunc("/block/", block_handler)
	http.HandleFunc("/txpool/", txpool_handler)
	http.HandleFunc("/tx/", tx_handler)
	http.HandleFunc("/", root_handler)

	go func() {
		logger.Info("Listening for requests")
		err = http.ListenAndServe(listen_address, nil)
		logger.Error(err, "ListenAndServe failed")

	}()

	time.Sleep(50 * time.Millisecond)

	return err
}

// all the tx info which ever needs to be printed
type txinfo struct {
	Hex             string // raw tx
	Height          string // height at which tx was mined
	HeightBuilt     uint64 // height at which tx was built
	BLID            string // BLID in whose reference this tx was built
	RootHash        string // roothash which forms the basis for balance tree
	TransactionType string // transaction type
	Depth           int64
	Timestamp       uint64 // timestamp
	Age             string //  time diff from current time
	Block_time      string // UTC time from block header
	Epoch           uint64 // Epoch time
	In_Pool         bool   // whether tx was in pool
	Hash            string // hash for hash
	PrefixHash      string // prefix hash
	Version         int    // version of tx
	Size            string // size of tx in KB
	Sizeuint64      uint64 // size of tx in bytes
	Burn_Value      string //  value of burned amount
	Fee             string // fee in TX
	Feeuint64       uint64 // fee in atomic units
	In              int    // inputs counts
	Out             int    // outputs counts
	Amount          string
	CoinBase        bool     // is tx coin base
	Extra           string   // extra within tx
	Keyimages       []string // key images within tx
	OutAddress      []string // contains output secret key
	OutOffset       []uint64 // contains index offsets
	Type            string   // ringct or ruffct ( bulletproof)
	ValidBlock      string   // the tx is valid in which block
	InvalidBlock    []string // the tx is invalid in which block
	Skipped         bool     // this is only valid, when a block is being listed
	Ring_size       int
	Ring            [][]string // contains entire ring  in string form

	TXpublickey string
	PayID32     string // 32 byte payment ID
	PayID8      string // 8 byte encrypted payment ID

	Proof_address     string // address agains which which the proving ran
	Proof_index       int64  // proof satisfied for which index
	Proof_amount      string // decoded amount
	Proof_Payload_raw string // payload raw bytes
	Proof_Payload     string //   if proof decoded, decoded , else decode error
	Proof_error       string // error if any while decoding proof

	SC_TX_Available    string            //bool   // whether this contains an SC TX
	SC_Signer          string            // whether SC signer
	SC_Signer_verified string            // whether SC signer  can be verified successfully
	SC_Balance         uint64            // SC SC_Balance in atomic units
	SC_Balance_string  string            // SC_Balance in DERO
	SC_Keys            map[string]string // SC key value of
	SC_Args            rpc.Arguments     // rpc.Arguments
	SC_Code            string            // install SC
	SC_State           rpc.GetSC_Result  // current SC state
	SC_Install         bool

	Assets []Asset
}

type Asset struct {
	SCID      string
	Fees      string
	Burn      string
	Ring      []string
	Ring_size int
}

// any information for block which needs to be printed
type block_info struct {
	Block         block.Block
	Major_Version uint64
	Minor_Version uint64
	Height        int64
	TopoHeight    int64
	Depth         int64
	Timestamp     uint64
	Hash          string
	Tips          []string
	Nonce         uint64
	Fees          string
	Reward        string
	Size          string
	Age           string //  time diff from current time
	Block_time    string // UTC time from block header
	Epoch         uint64 // Epoch time
	Outputs       string
	Mtx           txinfo
	Txs           []txinfo
	Orphan_Status bool
	SyncBlock     bool // whether the block is sync block
	Tx_Count      int
}

var all_templates *template.Template

// load and setup block_info from rpc
// if hash is less than 64 bytes then it is considered a height parameter
func load_block_from_rpc(info *block_info, block_hash string, recursive bool) (err error) {
	var bl block.Block
	var bresult rpc.GetBlock_Result

	var block_height int
	var block_bin []byte
	if len(block_hash) != 64 { // parameter is a height
		fmt.Sscanf(block_hash, "%d", &block_height)
		// user requested block height
		logger.V(1).Info("User requested block", "topoheight", block_height, "user input", block_hash)
		if err = rpc_client.Call("DERO.GetBlock", rpc.GetBlock_Params{Height: uint64(block_height)}, &bresult); err != nil {
			return fmt.Errorf("getblock rpc failed. err %s", err)
		}

	} else { // parameter is the hex blob
		logger.V(1).Info("User requested block using hash", "block_hash", block_hash)

		if err = rpc_client.Call("DERO.GetBlock", rpc.GetBlock_Params{Hash: block_hash}, &bresult); err != nil {
			return fmt.Errorf("getblock rpc failed")
		}
	}

	// fmt.Printf("block %d  %+v\n",i, bresult)
	info.TopoHeight = bresult.Block_Header.TopoHeight
	info.Height = bresult.Block_Header.Height
	info.Depth = bresult.Block_Header.Depth

	duration_millisecond := (uint64(time.Now().UTC().UnixMilli()) - bresult.Block_Header.Timestamp)
	info.Age = replacer.Replace((time.Duration(duration_millisecond) * time.Millisecond).String())
	info.Block_time = time.Unix(0, int64(bresult.Block_Header.Timestamp*uint64(time.Millisecond))).Format("2006-01-02 15:04:05")
	info.Epoch = bresult.Block_Header.Timestamp
	info.Outputs = fmt.Sprintf("%.03f", float32(bresult.Block_Header.Reward)/1000000000000.0)
	info.Size = "N/A"
	info.Hash = bresult.Block_Header.Hash
	//info.Prev_Hash = bresult.Block_Header.Prev_Hash
	info.Tips = bresult.Block_Header.Tips
	info.Orphan_Status = bresult.Block_Header.Orphan_Status
	info.SyncBlock = bresult.Block_Header.SyncBlock
	info.Nonce = bresult.Block_Header.Nonce
	info.Major_Version = bresult.Block_Header.Major_Version
	info.Minor_Version = bresult.Block_Header.Minor_Version
	info.Reward = fmt.Sprintf("%.05f", float32(bresult.Block_Header.Reward)/100000.0)

	block_bin, _ = hex.DecodeString(bresult.Blob)

	//log.Infof("block %+v bresult %+v ", bl, bresult)

	bl.Deserialize(block_bin)

	info.Block = bl

	if recursive {
		// fill in miner tx info

		//err = load_tx_from_rpc(&info.Mtx, bl.Miner_TX.GetHash().String()) //TODO handle error

		load_tx_info_from_tx(&info.Mtx, &bl.Miner_TX)

		// miner tx reward is calculated on runtime due to client protocol reasons in dero atlantis
		// feed what is calculated by the daemon
		reward := uint64(0)
		if bl.Miner_TX.TransactionType == transaction.PREMINE {
			reward += bl.Miner_TX.Value
		}
		info.Mtx.Amount = fmt.Sprintf("%.05f", float64(reward+bresult.Block_Header.Reward)/100000)

		//logger.Error(err,"loading miner tx from rpc ", "txid", bl.Miner_TX.GetHash().String())
		info.Tx_Count = len(bl.Tx_hashes)

		fees := uint64(0)
		size := uint64(len(bl.Serialize()))
		// if we have any other tx load them also
		for i := 0; i < len(bl.Tx_hashes); i++ {

			var tx txinfo
			err = load_tx_from_rpc(&tx, bl.Tx_hashes[i].String()) //TODO handle error
			if err != nil {
				logger.V(1).Error(err, "loading tx ", "txid", bl.Tx_hashes[i].String())
			}
			if tx.ValidBlock != bresult.Block_Header.Hash { // track skipped status
				tx.Skipped = true
			}
			info.Txs = append(info.Txs, tx)
			fees += tx.Feeuint64
			size += tx.Sizeuint64
		}

		info.Fees = fmt.Sprintf("%.03f", float32(fees)/100000.0)
		info.Size = fmt.Sprintf("%.03f", float32(size)/1024)

	}

	return
}

// this will fill up the info struct from the tx
func load_tx_info_from_tx(info *txinfo, tx *transaction.Transaction) (err error) {
	info.Hash = tx.GetHash().String()
	//info.PrefixHash = tx.GetPrefixHash().String()
	info.TransactionType = tx.TransactionType.String()
	info.Size = fmt.Sprintf("%.03f", float32(len(tx.Serialize()))/1024)
	info.Sizeuint64 = uint64(len(tx.Serialize()))
	info.Version = int(tx.Version)

	//info.Extra = fmt.Sprintf("%x", tx.Extra)

	if len(tx.Payloads) >= 1 {
		info.RootHash = fmt.Sprintf("%x", tx.Payloads[0].Statement.Roothash[:])
	}
	info.HeightBuilt = tx.Height
	info.BLID = fmt.Sprintf("%x", tx.BLID)
	//info.In = len(tx.Vin)
	//info.Out = len(tx.Vout)

	if tx.TransactionType == transaction.BURN_TX {
		info.Burn_Value = fmt.Sprintf(" %.05f", float64(tx.Value)/100000)
	}

	switch tx.TransactionType {

	case transaction.PREMINE:
		var acckey crypto.Point
		if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
			panic(err)
		}

		astring := rpc.NewAddressFromKeys(&acckey)
		astring.Mainnet = mainnet
		info.OutAddress = append(info.OutAddress, astring.String())
		info.Amount = globals.FormatMoney(tx.Value)

	case transaction.REGISTRATION:
		var acckey crypto.Point
		if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
			panic(err)
		}

		astring := rpc.NewAddressFromKeys(&acckey)
		astring.Mainnet = mainnet
		info.OutAddress = append(info.OutAddress, astring.String())

	case transaction.COINBASE:
		info.CoinBase = true
		info.In = 0
		var acckey crypto.Point
		if err := acckey.DecodeCompressed(tx.MinerAddress[:]); err != nil {
			panic(err)
		}
		astring := rpc.NewAddressFromKeys(&acckey)
		astring.Mainnet = mainnet
		info.OutAddress = append(info.OutAddress, astring.String())
	case transaction.NORMAL, transaction.BURN_TX, transaction.SC_TX:
		info.Fee = globals.FormatMoney(tx.Fees())
		info.Ring_size = int(tx.Payloads[0].Statement.RingSize)

	}

	if tx.TransactionType == transaction.SC_TX {
		info.SC_Args = tx.SCDATA

	}

	// if outputs cannot be located, do not panic
	// this will be the case for pool transactions
	if len(info.OutAddress) != len(info.OutOffset) {
		info.OutOffset = make([]uint64, len(info.OutAddress), len(info.OutAddress))
	}

	switch 0 {
	case 0:
		info.Type = "DERO_HOMOMORPHIC"
	default:
		panic("not implemented")
	}

	if !info.In_Pool && !info.CoinBase && (tx.TransactionType == transaction.NORMAL || tx.TransactionType == transaction.BURN_TX || tx.TransactionType == transaction.SC_TX) { // find the age of block and other meta
		var blinfo block_info
		err := load_block_from_rpc(&blinfo, fmt.Sprintf("%s", info.Height), false) // we only need block data and not data of txs
		if err != nil {
			return err
		}

		//    fmt.Printf("Blinfo %+v height %d", blinfo, info.Height);

		info.Age = blinfo.Age
		info.Block_time = blinfo.Block_time
		info.Epoch = blinfo.Epoch
		info.Timestamp = blinfo.Epoch
		info.Depth = blinfo.Depth

	}

	return nil
}

// load and setup txinfo from rpc
func load_tx_from_rpc(info *txinfo, txhash string) (err error) {
	var tx_params rpc.GetTransaction_Params
	var tx_result rpc.GetTransaction_Result

	//fmt.Printf("Requesting tx data %s", txhash);
	tx_params.Tx_Hashes = append(tx_params.Tx_Hashes, txhash)

	if err = rpc_client.Call("DERO.GetTransaction", tx_params, &tx_result); err != nil {
		return fmt.Errorf("gettransa rpc failed err %s", err)
	}
	//fmt.Printf("TX response %+v", tx_result)
	if tx_result.Status != "OK" {
		return fmt.Errorf("No Such TX RPC error status %s", tx_result.Status)
	}

	var tx transaction.Transaction

	if len(tx_result.Txs_as_hex[0]) < 50 {
		return
	}

	info.Hex = tx_result.Txs_as_hex[0]

	tx_bin, _ := hex.DecodeString(tx_result.Txs_as_hex[0])
	tx.Deserialize(tx_bin)

	// fill as much info required from headers
	if tx_result.Txs[0].In_pool {
		info.In_Pool = true
	} else {
		info.Height = fmt.Sprintf("%d", tx_result.Txs[0].Block_Height)
	}

	for x := range tx_result.Txs[0].Output_Indices {
		info.OutOffset = append(info.OutOffset, tx_result.Txs[0].Output_Indices[x])
	}

	if tx.IsCoinbase() { // fill miner tx reward from what the chain tells us
		info.Amount = fmt.Sprintf("%.05f", float64(uint64(tx_result.Txs[0].Reward))/100000)
	}

	info.ValidBlock = tx_result.Txs[0].ValidBlock
	info.InvalidBlock = tx_result.Txs[0].InvalidBlock

	info.Ring = tx_result.Txs[0].Ring

	if tx.TransactionType == transaction.SC_TX {
		info.SC_Signer = "Unknown"
	}

	if tx.TransactionType == transaction.NORMAL || tx.TransactionType == transaction.BURN_TX || tx.TransactionType == transaction.SC_TX {

		for t := range tx.Payloads {
			var a Asset
			a.SCID = tx.Payloads[t].SCID.String()
			a.Fees = fmt.Sprintf("%.05f", float64(tx.Payloads[t].Statement.Fees)/100000)
			a.Burn = fmt.Sprintf("%.05f", float64(tx.Payloads[t].BurnValue)/100000)

			if len(tx_result.Txs[0].Ring) == 0 {
				continue
			}

			a.Ring_size = len(tx_result.Txs[0].Ring[t])
			a.Ring = tx_result.Txs[0].Ring[t]
			info.SC_Signer = tx_result.Txs[0].Signer

			info.Assets = append(info.Assets, a)

		}
		//fmt.Printf("assets  now %+v\n", info.Assets)
	}

	info.SC_Balance = tx_result.Txs[0].Balance
	info.SC_Balance_string = fmt.Sprintf("%.05f", float64(uint64(info.SC_Balance)/100000))
	info.SC_Code = tx_result.Txs[0].Code

	if tx.TransactionType == transaction.SC_TX && len(info.SC_Code) >= 1 {

		if len(info.SC_Code) >= 1 {
			info.SC_Install = true
		}
		var p = rpc.GetSC_Params{SCID: txhash, Variables: true}
		var r rpc.GetSC_Result

		if err = rpc_client.Call("DERO.GetSC", p, &r); err != nil {
			logger.V(1).Error(err, "DERO.GetSC failed")
		} else {
			info.SC_State = r
		}
	}

	//info.Ring = strings.Join(info.OutAddress, " ")

	//fmt.Printf("tx_result %+v\n",tx_result.Txs)
	// fmt.Printf("response contained tx %s \n", tx.GetHash())

	return load_tx_info_from_tx(info, &tx)
}

func block_handler(w http.ResponseWriter, r *http.Request) {
	param := ""
	fmt.Sscanf(r.URL.EscapedPath(), "/block/%s", &param)

	var blinfo block_info
	err := load_block_from_rpc(&blinfo, param, true)
	_ = err

	// execute template now
	data := map[string]interface{}{}

	fill_common_info(data, false)
	data["block"] = blinfo

	err = all_templates.ExecuteTemplate(w, "block", data)
	if err != nil {
		return
	}

	return

	//     fmt.Fprint(w, "This is a valid block")

}

func tx_handler(w http.ResponseWriter, r *http.Request) {
	var info txinfo
	tx_hex := ""
	fmt.Sscanf(r.URL.EscapedPath(), "/tx/%s", &tx_hex)
	txhash := crypto.HashHexToHash(tx_hex)
	logger.V(1).Info("user requested ", "txid", tx_hex)

	err := load_tx_from_rpc(&info, txhash.String()) //TODO handle error
	_ = err

	// check whether user requested proof

	tx_proof := r.PostFormValue("txproof")
	raw_tx_data := r.PostFormValue("raw_tx_data")

	if raw_tx_data != "" { // gives ability to prove transactions not in the blockchain
		info.Hex = raw_tx_data
	}

	if tx_proof != "" {
		logger.V(1).Info("Proving TX", "proof", tx_proof, "tx_hex", info.Hex, "ring", info.Ring)

		// there may be more than 1 amounts, only first one is shown
		addresses, amounts, raw, decoded, err := proof.Prove(tx_proof, info.Hex, info.Ring, mainnet)
		if err == nil { //&& len(amounts) > 0 && len(indexes) > 0{
			logger.V(1).Info("Successfully proved transaction", "txid", tx_hex, "payload_count", len(decoded))
			info.Proof_address = addresses[0]
			info.Proof_amount = globals.FormatMoney(amounts[0])
			info.Proof_Payload_raw = BytesViewer(raw[0]).String() // raw payload
			info.Proof_Payload = decoded[0]

		} else {
			logger.V(1).Error(err, "err while proving")
			if err != nil {
				info.Proof_error = err.Error()
			}

		}
	}

	// execute template now
	data := map[string]interface{}{}

	fill_common_info(data, false)
	data["info"] = info

	err = all_templates.ExecuteTemplate(w, "tx", data)
	if err != nil {
		return
	}

	return

}

func pool_handler(w http.ResponseWriter, r *http.Request) {

	fmt.Fprint(w, "This is a valid pool")

}

// if there is any error, we return back empty
// if pos is wrong we return back
// pos is descending order
func fill_tx_structure(pos int, size_in_blocks int) (data []block_info) {

	i := pos
	for ; i > (pos-size_in_blocks) && i >= 0; i-- { // query blocks by topo height
		var blinfo block_info
		if err := load_block_from_rpc(&blinfo, fmt.Sprintf("%d", i), true); err == nil {
			data = append(data, blinfo)
		} else {
			logger.V(2).Error(err, "error loading block", "i", i)
		}
	}
	if i == 0 {
		var blinfo block_info
		if err := load_block_from_rpc(&blinfo, fmt.Sprintf("%d", i), true); err == nil {
			data = append(data, blinfo)
		} else {
			logger.V(2).Error(err, "error loading block", "i", i)
		}
	}
	return
}

func show_page(w http.ResponseWriter, page int) {
	data := map[string]interface{}{}
	var info rpc.GetInfo_Result
	var err error

	if err = rpc_client.Call("DERO.GetInfo", nil, &info); err != nil {
		goto exit_error
	}

	fill_common_info(data, true)

	if page == 0 { // use requested invalid page, give current page
		page = int(info.TopoHeight) / 10
	}

	data["previous_page"] = page - 1
	if page <= 1 {
		data["previous_page"] = 1
	}
	data["current_page"] = page
	if (int(info.TopoHeight) % 10) == 0 {
		data["total_page"] = (int(info.TopoHeight) / 10)
	} else {
		data["total_page"] = (int(info.TopoHeight) / 10)
	}

	data["next_page"] = page + 1
	if (page + 1) > data["total_page"].(int) {
		data["next_page"] = page
	}

	fill_tx_pool_info(data, 25)

	if page == 1 { // page 1 has 11 blocks, it does not show genesis block
		data["block_array"] = fill_tx_structure(int(page*10), 12)
	} else {
		if int(info.TopoHeight)-int(page*10) > 10 {
			data["block_array"] = fill_tx_structure(int(page*10), 10)
		} else {
			data["block_array"] = fill_tx_structure(int(info.TopoHeight), int(info.TopoHeight)-int((page-1)*10))
		}

	}

	//fmt.Printf("page %+v\n", data)

	err = all_templates.ExecuteTemplate(w, "main", data)
	if err != nil {
		goto exit_error
	}

	return

exit_error:
	fmt.Fprintf(w, "Error occurred err %s", err)

}

func txpool_handler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{}
	fill_common_info(data, true)
	fill_tx_pool_info(data, 500) // show only 500 txs

	var err error
	if err = all_templates.ExecuteTemplate(w, "txpool_page", data); err != nil {
		goto exit_error
	}

	return

exit_error:
	fmt.Fprintf(w, "Error occurred err %s", err)

}

// shows a page
func page_handler(w http.ResponseWriter, r *http.Request) {
	page := 0
	page_string := r.URL.EscapedPath()
	fmt.Sscanf(page_string, "/page/%d", &page)
	logger.V(1).Info("user requested page", "page", page)
	show_page(w, page)
}

// root shows page 0
func root_handler(w http.ResponseWriter, r *http.Request) {
	logger.V(1).Info("Showing main page")
	show_page(w, 0)
}

// search handler, finds the items using rpc bruteforce
func search_handler(w http.ResponseWriter, r *http.Request) {
	var info rpc.GetInfo_Result
	var err error

	logger.V(1).Info("Showing search page")

	values, ok := r.URL.Query()["value"]

	if !ok || len(values) < 1 {
		show_page(w, 0)
		return
	}

	// Query()["key"] will return an array of items,
	// we only want the single item.
	value := strings.TrimSpace(values[0])
	good := false

	// collect all the data afresh,  execute rpc to service
	if err = rpc_client.Call("DERO.GetInfo", nil, &info); err != nil {
		goto exit_error
	}

	if len(value) != 64 {
		if s, err := strconv.ParseInt(value, 10, 64); err == nil && s >= 0 && s <= info.TopoHeight {
			good = true
		}
	} else { // check whether the string can be hex decoded
		t, err := hex.DecodeString(value)
		if err != nil || len(t) != 32 {

		} else {
			good = true
		}
	}

	// value should be either 64 hex chars or a topoheight which should be less than current topoheight

	if good {
		// check whether the page is block or tx or height
		var blinfo block_info
		var tx txinfo
		err := load_block_from_rpc(&blinfo, value, false)
		if err == nil {
			logger.V(1).Info("Redirecting user to block page")
			http.Redirect(w, r, "/block/"+value, 302)
			return
		}

		err = load_tx_from_rpc(&tx, value) //TODO handle error
		if err == nil {
			logger.V(1).Info("Redirecting user to tx page")
			http.Redirect(w, r, "/tx/"+value, 302)

			return
		}
	}

	{
		data := map[string]interface{}{}
		fill_common_info(data, true)
		if err = all_templates.ExecuteTemplate(w, "notfound_page", data); err == nil {
			return
		}
	}

exit_error:
	show_page(w, 0)
	return

}

func fill_common_info(data map[string]interface{}, extra_data bool) error {

	var info rpc.GetInfo_Result

	data["title"] = "DERO HE BlockChain Explorer(v1)"
	data["servertime"] = time.Now().UTC().Format("2006-01-02 15:04:05")

	if !extra_data {
		return nil
	}
	// collect all the data afresh,  execute rpc to service
	if err := rpc_client.Call("DERO.GetInfo", nil, &info); err != nil {
		return err
	}

	//fmt.Printf("get info %+v", info)

	data["Network_Difficulty"] = info.Difficulty
	data["hash_rate"] = fmt.Sprintf("%.03f", float32(info.Difficulty/1000000))
	data["txpool_size"] = info.Tx_pool_size
	data["testnet"] = info.Testnet
	data["network"] = info.Network
	data["fee_per_kb"] = float64(info.Dynamic_fee_per_kb) / 1000000000000
	data["median_block_size"] = fmt.Sprintf("%.02f", float32(info.Median_Block_Size)/1024)
	data["total_supply"] = info.Total_Supply
	data["averageblocktime50"] = info.AverageBlockTime50

	return nil
}

// fill all the tx pool info as per requested
func fill_tx_pool_info(data map[string]interface{}, max_count int) error {

	var err error
	var txs []txinfo
	var txpool rpc.GetTxPool_Result

	data["mempool"] = txs // initialize with empty data
	if err = rpc_client.Call("DERO.GetTxPool", nil, &txpool); err != nil {
		return fmt.Errorf("gettxpool rpc failed")
	}

	for i := range txpool.Tx_list {
		var info txinfo
		err := load_tx_from_rpc(&info, txpool.Tx_list[i]) //TODO handle error
		if err != nil {
			continue
		}
		txs = append(txs, info)

		if len(txs) >= max_count {
			break
		}
	}

	data["mempool"] = txs
	return nil
}

// BytesViewer bytes viewer
type BytesViewer []byte

// String returns view in hexadecimal
func (b BytesViewer) String() string {
	if len(b) == 0 {
		return "invlaid string"
	}
	const head = `
| Address  | Hex                                             | Text             |
| -------: | :---------------------------------------------- | :--------------- |
`
	const row = 16
	result := make([]byte, 0, len(head)/2*(len(b)/16+3))
	result = append(result, head...)
	for i := 0; i < len(b); i += row {
		result = append(result, "| "...)
		result = append(result, fmt.Sprintf("%08x", i)...)
		result = append(result, " | "...)

		k := i + row
		more := 0
		if k >= len(b) {
			more = k - len(b)
			k = len(b)
		}
		for j := i; j != k; j++ {
			if b[j] < 16 {
				result = append(result, '0')
			}
			result = strconv.AppendUint(result, uint64(b[j]), 16)
			result = append(result, ' ')
		}
		for j := 0; j != more; j++ {
			result = append(result, "   "...)
		}
		result = append(result, "| "...)
		buf := bytes.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return ' '
			}
			return r
		}, b[i:k])
		result = append(result, buf...)
		for j := 0; j != more; j++ {
			result = append(result, ' ')
		}
		result = append(result, " |\n"...)
	}
	return *(*string)(unsafe.Pointer(&result))
}
