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

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/glue/rwc"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/walletapi"
	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
)

//import "github.com/creachadair/jrpc2/server"

/* this file implements the rpcserver api, so as wallet and block explorer tools can work without migration */

// all components requiring access to blockchain must use , this struct to communicate
// this structure must be update while mutex
type RPCServer struct {
	srv        *http.Server
	mux        *http.ServeMux
	logger     logr.Logger
	user       string
	password   string
	Exit_Event chan bool // blockchain is shutting down and we must quit ASAP
	sync.RWMutex
}

var client_connections sync.Map

func RPCServer_Start(wallet *walletapi.Wallet_Disk, title string) (*RPCServer, error) {
	var r RPCServer

	r.logger = globals.Logger.WithName(title) // all components must use this logger

	r.Exit_Event = make(chan bool)

	if globals.Arguments["--rpc-login"] != nil { // this was verified at startup
		userpass := globals.Arguments["--rpc-login"].(string)
		parts := strings.SplitN(userpass, ":", 2)
		r.user = parts[0]
		r.password = parts[1]
	}

	go r.Run(wallet)
	atomic.AddUint32(&globals.Subsystem_Active, 1) // increment subsystem

	return &r, nil
}

// shutdown the rpc server component
func (r *RPCServer) RPCServer_Stop() {
	r.Lock()
	defer r.Unlock()

	close(r.Exit_Event) // send signal to all connections to exit

	if r.srv != nil {
		r.srv.Shutdown(context.Background()) // shutdown the server
	}
	// TODO we  must wait for connections to kill themselves
	time.Sleep(1 * time.Second)
	r.logger.Info("RPC Shutdown")
	atomic.AddUint32(&globals.Subsystem_Active, ^uint32(0)) // this decrement 1 fom subsystem
}

// check basic authrizaion
func hasbasicauthfailed(rpcserver *RPCServer, w http.ResponseWriter, r *http.Request) bool {
	if rpcserver.user == "" {
		return false
	}
	u, p, ok := r.BasicAuth()
	if !ok {
		w.WriteHeader(401)
		io.WriteString(w, "Authorization Required")
		return true
	}
	if u != rpcserver.user || p != rpcserver.password {
		w.WriteHeader(401)
		io.WriteString(w, "Authorization Required")
		return true
	}

	if globals.Arguments["--allow-rpc-password-change"] != nil && globals.Arguments["--allow-rpc-password-change"].(bool) == true {

		if r.Header.Get("Pass") != "" {
			rpcserver.password = r.Header.Get("Pass")
		}
	}

	return false

}

// setup handlers
func (rpcserver *RPCServer) Run(wallet *walletapi.Wallet_Disk) {

	var wallet_apis WalletContext

	wallet_apis.logger = rpcserver.logger
	wallet_apis.wallet = wallet

	var options = &jrpc2.ServerOptions{AllowPush: true, NewContext: func() context.Context { return context.WithValue(context.Background(), "wallet_context", &wallet_apis) }}
	// create a new mux
	rpcserver.mux = http.NewServeMux()

	default_address := "127.0.0.1:" + fmt.Sprintf("%d", config.Mainnet.Wallet_RPC_Default_Port)
	if !globals.IsMainnet() {
		default_address = "127.0.0.1:" + fmt.Sprintf("%d", config.Testnet.Wallet_RPC_Default_Port)
	}

	if _, ok := globals.Arguments["--rpc-bind"]; ok && globals.Arguments["--rpc-bind"] != nil {
		addr, err := net.ResolveTCPAddr("tcp", globals.Arguments["--rpc-bind"].(string))
		if err != nil {
			rpcserver.logger.Error(err, "--rpc-bind address is invalid")
		} else {
			if addr.Port == 0 {
				rpcserver.logger.Info("RPC server is disabled, No ports will be opened for RPC")
				return
			} else {
				default_address = addr.String()
			}
		}
	}

	rpcserver.logger.Info("Wallet RPC/Websocket server starting", "address", default_address)
	rpcserver.Lock()
	rpcserver.srv = &http.Server{Addr: default_address, Handler: rpcserver.mux}
	rpcserver.Unlock()

	// Bridge HTTP to the JSON-RPC server.
	var bridge = jhttp.NewBridge(WalletHandler, &jhttp.BridgeOptions{Server: options})

	translate_http_to_jsonrpc_and_vice_versa := func(w http.ResponseWriter, r *http.Request) {

		if hasbasicauthfailed(rpcserver, w, r) {
			return
		}
		bridge.ServeHTTP(w, r)
	}

	ws_handler := func(w http.ResponseWriter, r *http.Request) {
		var ws_server *jrpc2.Server
		defer func() {
			if r := recover(); r != nil { // safety so if anything wrong happens, verification fails
				rpcserver.logger.V(1).Error(nil, "Recovered while processing websocket request", "r", r, "stack", debug.Stack())
			}
			if ws_server != nil {
				client_connections.Delete(ws_server)
			}
		}()
		if hasbasicauthfailed(rpcserver, w, r) {
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			rpcserver.logger.V(1).Error(err, "upgrade:")
			return
		}
		defer c.Close()

		input_output := rwc.New(c)
		ws_server = jrpc2.NewServer(servicemux, options).Start(channel.RawJSON(input_output, input_output))
		client_connections.Store(ws_server, 1)
		ws_server.Wait()
	}

	rpcserver.mux.HandleFunc("/json_rpc", translate_http_to_jsonrpc_and_vice_versa)
	rpcserver.mux.HandleFunc("/ws", ws_handler)
	rpcserver.mux.HandleFunc("/", hello)

	// handle SC installer,        // this will install an sc an

	rpcserver.mux.HandleFunc("/install_sc", func(w http.ResponseWriter, req *http.Request) { // translate call internally,  how to do it using a single json request
		var p rpc.Transfer_Params

		if hasbasicauthfailed(rpcserver, w, req) {
			return
		}

		b, err := ioutil.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		p.SC_Code = string(b) // encode as base64
		p.Ringsize = 2        // experts need not use this, they have direct call to do it

		if result, err := Transfer(context.WithValue(context.Background(), "wallet_context", &wallet_apis), p); err != nil {
			fmt.Fprintf(w, err.Error())
			return
		} else {
			if err := json.NewEncoder(w).Encode(result); err != nil {
				fmt.Fprintf(w, err.Error())
				return
			}
		}
	})

	// handle nasty http requests
	//r.mux.HandleFunc("/getheight", getheight)

	//if DEBUG_MODE {
	// r.mux.HandleFunc("/debug/pprof/", pprof.Index)

	// Register pprof handlers individually if required
	/*		r.mux.HandleFunc("/debug/pprof/", pprof.Index)
			r.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			r.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			r.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			r.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	*/

	/*
	       // Register pprof handlers individually if required
	   r.mux.HandleFunc("/cdebug/pprof/", pprof.Index)
	   r.mux.HandleFunc("/cdebug/pprof/cmdline", pprof.Cmdline)
	   r.mux.HandleFunc("/cdebug/pprof/profile", pprof.Profile)
	   r.mux.HandleFunc("/cdebug/pprof/symbol", pprof.Symbol)
	   r.mux.HandleFunc("/cdebug/pprof/trace", pprof.Trace)
	*/

	// register metrics handler
	//	r.mux.HandleFunc("/metrics", prometheus.InstrumentHandler("dero", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))

	//}

	//r.mux.HandleFunc("/json_rpc/debug", mr.ServeDebug)

	if err := rpcserver.srv.ListenAndServe(); err != http.ErrServerClosed {
		rpcserver.logger.Error(err, "ListenAndServe failed")
	}

}

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "DERO BLOCKCHAIN Hello world!")
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }} // use default options

type WalletContext struct {
	logger logr.Logger
	wallet *walletapi.Wallet_Disk
	// Add any extra customizable data in context
	Extra map[string]interface{}
} // exports daemon status and other RPC apis

func WalletEcho(ctx context.Context, args []string) string {
	return "WALLET " + strings.Join(args, " ")
}

// used to verify whether the connection is alive
func Ping(ctx context.Context) string {
	return "Pong "
}

func Echo(ctx context.Context, args []string) string {
	return "DERO " + strings.Join(args, " ")
}

var WalletHandler = handler.Map{
	"Echo":                     handler.New(WalletEcho),
	"getaddress":               handler.New(GetAddress),
	"GetAddress":               handler.New(GetAddress),
	"getbalance":               handler.New(GetBalance),
	"GetBalance":               handler.New(GetBalance),
	"get_tracked_assets":       handler.New(GetTrackedAssets),
	"GetTrackedAssets":         handler.New(GetTrackedAssets),
	"getheight":                handler.New(GetHeight),
	"GetHeight":                handler.New(GetHeight),
	"get_transfer_by_txid":     handler.New(GetTransferbyTXID),
	"GetTransferbyTXID":        handler.New(GetTransferbyTXID),
	"get_transfers":            handler.New(GetTransfers),
	"GetTransfers":             handler.New(GetTransfers),
	"make_integrated_address":  handler.New(MakeIntegratedAddress),
	"MakeIntegratedAddress":    handler.New(MakeIntegratedAddress),
	"split_integrated_address": handler.New(SplitIntegratedAddress),
	"SplitIntegratedAddress":   handler.New(SplitIntegratedAddress),
	"query_key":                handler.New(QueryKey),
	"QueryKey":                 handler.New(QueryKey),
	"transfer":                 handler.New(Transfer),
	"Transfer":                 handler.New(Transfer),
	"transfer_split":           handler.New(Transfer),
	"scinvoke":                 handler.New(ScInvoke),
}

var servicemux = handler.ServiceMap{
	"DERO": handler.Map{
		"Echo": handler.New(Echo),
		"Ping": handler.New(Ping),
	},
	"WALLET": WalletHandler,
}

func FromContext(ctx context.Context) *WalletContext {
	u, ok := ctx.Value("wallet_context").(*WalletContext)
	if !ok {
		panic("cannot find wallet context")
	}
	return u
}

func NewWalletContext(logger logr.Logger, wallet *walletapi.Wallet_Disk) *WalletContext {
	return &WalletContext{
		logger: logger,
		wallet: wallet,
		Extra:  make(map[string]interface{}),
	}
}
