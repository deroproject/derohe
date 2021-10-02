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

package main

import "io"
import "net"
import "fmt"
import "net/http"
import "time"
import "sync"
import "sync/atomic"
import "context"
import "strings"
import "runtime/debug"
import "net/http/pprof"

import "github.com/romana/rlog"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/metrics"
import "github.com/deroproject/derohe/blockchain"
import "github.com/deroproject/derohe/glue/rwc"

import log "github.com/sirupsen/logrus"
import "github.com/gorilla/websocket"

import "github.com/creachadair/jrpc2"
import "github.com/creachadair/jrpc2/handler"
import "github.com/creachadair/jrpc2/channel"

import "github.com/creachadair/jrpc2/jhttp"

/* this file implements the rpcserver api, so as wallet and block explorer tools can work without migration */

// all components requiring access to blockchain must use , this struct to communicate
// this structure must be update while mutex
type RPCServer struct {
	srv        *http.Server
	mux        *http.ServeMux
	Exit_Event chan bool // blockchain is shutting down and we must quit ASAP
	sync.RWMutex
}

//var Exit_In_Progress bool
var chain *blockchain.Blockchain
var logger *log.Entry

var client_connections sync.Map

var options = &jrpc2.ServerOptions{AllowPush: true}

// this function triggers notification to all clients that they should repoll
func Notify_Block_Addition() {

	for {
		chain.RPC_NotifyNewBlock.L.Lock()
		chain.RPC_NotifyNewBlock.Wait()
		chain.RPC_NotifyNewBlock.L.Unlock()
		client_connections.Range(func(key, value interface{}) bool {

			key.(*jrpc2.Server).Notify(context.Background(), "Repoll", nil)
			return true
		})
	}
}

func Notify_Height_Changes() {

	for {
		chain.RPC_NotifyNewBlock.L.Lock()
		chain.RPC_NotifyNewBlock.Wait()
		chain.RPC_NotifyNewBlock.L.Unlock()
		client_connections.Range(func(key, value interface{}) bool {

			key.(*jrpc2.Server).Notify(context.Background(), "HRepoll", nil)
			return true
		})
	}
}

func RPCServer_Start(params map[string]interface{}) (*RPCServer, error) {

	var err error
	var r RPCServer

	_ = err

	r.Exit_Event = make(chan bool)

	logger = globals.Logger.WithFields(log.Fields{"com": "RPC"}) // all components must use this logger
	chain = params["chain"].(*blockchain.Blockchain)

	/*
		// test whether chain is okay
		if chain.Get_Height() == 0 {
			return nil, fmt.Errorf("Chain DOES NOT have genesis block")
		}
	*/

	go r.Run()
	logger.Infof("RPC/Websocket server started")
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
	logger.Infof("RPC Shutdown")
	atomic.AddUint32(&globals.Subsystem_Active, ^uint32(0)) // this decrement 1 fom subsystem
}

// setup handlers
func (r *RPCServer) Run() {

	// create a new mux
	r.mux = http.NewServeMux()

	default_address := "127.0.0.1:" + fmt.Sprintf("%d", config.Mainnet.RPC_Default_Port)
	if !globals.IsMainnet() {
		default_address = "127.0.0.1:" + fmt.Sprintf("%d", config.Testnet.RPC_Default_Port)
	}

	if _, ok := globals.Arguments["--rpc-bind"]; ok && globals.Arguments["--rpc-bind"] != nil {
		addr, err := net.ResolveTCPAddr("tcp", globals.Arguments["--rpc-bind"].(string))
		if err != nil {
			logger.Warnf("--rpc-bind address is invalid, err = %s", err)
		} else {
			if addr.Port == 0 {
				logger.Infof("RPC server is disabled, No ports will be opened for RPC")
				return
			} else {
				default_address = addr.String()
			}
		}
	}

	logger.Infof("RPC  will listen on %s", default_address)
	r.Lock()
	r.srv = &http.Server{Addr: default_address, Handler: r.mux}
	r.Unlock()

	r.mux.HandleFunc("/json_rpc", translate_http_to_jsonrpc_and_vice_versa)
    r.mux.HandleFunc("/metrics", metrics.WritePrometheus)  // write all the metrics

	r.mux.HandleFunc("/ws", ws_handler)
	r.mux.HandleFunc("/", hello)
    r.mux.HandleFunc("/debug/pprof/", pprof.Index)
    r.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	//if DEBUG_MODE {
	// r.mux.HandleFunc("/debug/pprof/", pprof.Index)

	// Register pprof handlers individually if required
	/*		r.mux.HandleFunc("/debug/pprof/", pprof.Index)
			r.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			r.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			r.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			r.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	*/




	go Notify_Block_Addition() // process all blocks
	go Notify_Height_Changes() // gives notification of changed height
	if err := r.srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Warnf("ERR listening to address err %s", err)
	}

}

func hello(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "DERO BLOCKCHAIN Hello world!")
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }} // use default options

func ws_handler(w http.ResponseWriter, r *http.Request) {

	var ws_server *jrpc2.Server
	defer func() {

		// safety so if anything wrong happens, verification fails
		if r := recover(); r != nil {
			logger.Warnf("Recovered while processing websocket request, Stack trace below ")
			logger.Warnf("Stack trace  \n%s", debug.Stack())
		}
		if ws_server != nil {
			client_connections.Delete(ws_server)
		}

	}()

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		rlog.Warnf("upgrade:", err)
		return
	}

	defer c.Close()
	input_output := rwc.New(c)
	ws_server = jrpc2.NewServer(assigner, options).Start(channel.RawJSON(input_output, input_output))
	client_connections.Store(ws_server, 1)
	ws_server.Wait()

}

var assigner = handler.ServiceMap{
	"DAEMON": handler.Map{
		"Echo": handler.New(DAEMON_RPC_APIS{}.Echo),
	},
	"DERO": handler.Map{
		"Ping": handler.New(DERO_RPC_APIS{}.Ping),
		"Echo": handler.New(DERO_RPC_APIS{}.Echo),
	},
}

type DAEMON_RPC_APIS struct{} // exports daemon status and other RPC apis

func (DAEMON_RPC_APIS) Echo(ctx context.Context, args []string) string {
	return "DAEMON " + strings.Join(args, " ")
}

type DERO_RPC_APIS struct{} // exports DERO specific apis, such as transaction

// used to verify whether the connection is alive
func (DERO_RPC_APIS) Ping(ctx context.Context) string {
	return "Pong "
}

func (DERO_RPC_APIS) Echo(ctx context.Context, args []string) string {
	return "DERO " + strings.Join(args, " ")
}

// Bridge HTTP to the JSON-RPC server.
// uses traditional "getinfo" for compatibility reasons
// Use DERO.GetInfo names
var bridge = jhttp.NewBridge(historical_apis, nil)

var dero_apis DERO_RPC_APIS

var historical_apis = handler.Map{"getinfo": handler.New(dero_apis.GetInfo),
	"get_info":                   handler.New(dero_apis.GetInfo), // this is just an alias to above
	"getblock":                   handler.New(dero_apis.GetBlock),
	"getblockheaderbytopoheight": handler.New(dero_apis.GetBlockHeaderByTopoHeight),
	"getblockheaderbyhash":       handler.New(dero_apis.GetBlockHeaderByHash),
	"gettxpool":                  handler.New(dero_apis.GetTxPool),
	"getrandomaddress":           handler.New(dero_apis.GetRandomAddress),
	"gettransactions":            handler.New(dero_apis.GetTransaction),
	"sendrawtransaction":         handler.New(dero_apis.SendRawTransaction),
	"submitblock":                handler.New(dero_apis.SubmitBlock),
	"getheight":                  handler.New(dero_apis.GetHeight),
	"getblockcount":              handler.New(dero_apis.GetBlockCount),
	"getlastblockheader":         handler.New(dero_apis.GetLastBlockHeader),
	"getblocktemplate":           handler.New(dero_apis.GetBlockTemplate),
	"getencryptedbalance":        handler.New(dero_apis.GetEncryptedBalance),
	"getsc":                      handler.New(dero_apis.GetSC)}

func translate_http_to_jsonrpc_and_vice_versa(w http.ResponseWriter, r *http.Request) {
	bridge.ServeHTTP(w, r)
}
