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

import "io"
import "os"
import "net"
import "fmt"
import "net/http"
import "net/http/pprof"
import "time"
import "sort"
import "sync"
import "sync/atomic"
import "context"
import "strings"
import "runtime/debug"
import "encoding/json"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/blockchain"
import "github.com/deroproject/derohe/glue/rwc"
import "github.com/deroproject/derohe/metrics"

import "github.com/go-logr/logr"
import "github.com/gorilla/websocket"

import "github.com/creachadair/jrpc2"
import "github.com/creachadair/jrpc2/handler"
import "github.com/creachadair/jrpc2/channel"

//import "github.com/creachadair/jrpc2/server"
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

// var Exit_In_Progress bool
var chain *blockchain.Blockchain
var logger logr.Logger

var client_connections sync.Map

var options = &jrpc2.ServerOptions{AllowPush: true, RPCLog: metrics_generator{}, DecodeContext: func(ctx context.Context, method string, param json.RawMessage) (context.Context, json.RawMessage, error) {
	t := time.Now()
	return context.WithValue(ctx, "start_time", &t), param, nil
}}

type metrics_generator struct{}

func (metrics_generator) LogRequest(ctx context.Context, req *jrpc2.Request) {}
func (metrics_generator) LogResponse(ctx context.Context, resp *jrpc2.Response) {
	defer globals.Recover(2)
	req := jrpc2.InboundRequest(ctx) // we cannot do anything here
	if req == nil {
		return
	}
	start_time, ok := ctx.Value("start_time").(*time.Time)
	if !ok {
		return //panic("cannot find time in context")
	}

	method := req.Method()
	metrics.Set.GetOrCreateHistogram(method + "_duration_histogram_seconds").UpdateDuration(*start_time)
	metrics.Set.GetOrCreateCounter(method + "_total").Inc()

	if output, err := resp.MarshalJSON(); err == nil {
		metrics.Set.GetOrCreateCounter(method + "_total_out_bytes").Add(len(output))
	}
}

// this function triggers notification to all clients that they should repoll
func Notify_Block_Addition() {

	for {
		chain.RPC_NotifyNewBlock.L.Lock()
		chain.RPC_NotifyNewBlock.Wait()
		chain.RPC_NotifyNewBlock.L.Unlock()
		go func() {
			defer globals.Recover(2)
			client_connections.Range(func(key, value interface{}) bool {
				key.(*jrpc2.Server).Notify(context.Background(), "Block", nil)
				return true
			})
		}()
	}
}

// this function triggers notification to all clients that they should repoll
func Notify_MiniBlock_Addition() {

	for {
		chain.RPC_NotifyNewMiniBlock.L.Lock()
		chain.RPC_NotifyNewMiniBlock.Wait()
		chain.RPC_NotifyNewMiniBlock.L.Unlock()

		if globals.Arguments["--simulator"] == nil || (globals.Arguments["--simulator"] != nil && globals.Arguments["--simulator"].(bool) == false) {
			go func() {
				defer globals.Recover(2)
				SendJob()
			}()
		}
	}
}

func Notify_Height_Changes() {

	for {
		chain.RPC_NotifyNewBlock.L.Lock()
		chain.RPC_NotifyNewBlock.Wait()
		chain.RPC_NotifyNewBlock.L.Unlock()

		go func() {
			defer globals.Recover(2)
			client_connections.Range(func(key, value interface{}) bool {
				key.(*jrpc2.Server).Notify(context.Background(), "Height", nil)
				return true
			})
		}()
	}
}

func RPCServer_Start(params map[string]interface{}) (*RPCServer, error) {
	var r RPCServer

	metrics.Set.GetOrCreateGauge("rpc_client_count", func() float64 { // set a new gauge
		count := float64(0)
		client_connections.Range(func(k, value interface{}) bool {
			count++
			return true
		})
		return count
	})

	r.Exit_Event = make(chan bool)

	logger = globals.Logger.WithName("RPC") // all components must use this logger
	chain = params["chain"].(*blockchain.Blockchain)

	go r.Run()
	logger.Info("RPC/Websocket server started")
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
	logger.Info("RPC Shutdown")
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
			logger.Error(err, "--rpc-bind address is invalid")
		} else {
			if addr.Port == 0 {
				logger.Info("RPC server is disabled, No ports will be opened for RPC")
				return
			} else {
				default_address = addr.String()
			}
		}
	}

	logger.Info("RPC will listen", "address", default_address)
	r.Lock()
	r.srv = &http.Server{Addr: default_address, Handler: r.mux}
	r.Unlock()

	r.mux.HandleFunc("/json_rpc", translate_http_to_jsonrpc_and_vice_versa)
	r.mux.HandleFunc("/ws", ws_handler)
	r.mux.HandleFunc("/", hello)
	r.mux.HandleFunc("/metrics", metrics.WritePrometheus) // register metrics handler

	//if DEBUG_MODE {
	// r.mux.HandleFunc("/debug/pprof/", pprof.Index)

	// Register pprof handlers individually if required
	// we should provide a way to disable these

	if os.Getenv("DISABLE_RUNTIME_PROFILE") == "1" { // daemon must have been started with DISABLE_RUNTIME_PROFILE=1
		logger.Info("runtime profiling is disabled")
	} else { // Register pprof handlers individually if required

		r.mux.HandleFunc("/debug/pprof/", pprof.Index)
		r.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	go Notify_Block_Addition()     // process all blocks
	go Notify_MiniBlock_Addition() // process all blocks
	go Notify_Height_Changes()     // gives notification of changed height
	if err := r.srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error(err, "ListenAndServe failed")
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
			logger.V(2).Error(nil, "Recovered while processing websocket request", "r", r, "stack", debug.Stack())
		}
		if ws_server != nil {
			client_connections.Delete(ws_server)
		}

	}()

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	defer c.Close()
	input_output := rwc.New(c)
	ws_server = jrpc2.NewServer(d, options).Start(channel.RawJSON(input_output, input_output))
	client_connections.Store(ws_server, 1)
	ws_server.Wait()

}

func DAEMON_Echo(ctx context.Context, args []string) string {
	return "DAEMON " + strings.Join(args, " ")
}

// used to verify whether the connection is alive
func Ping(ctx context.Context) string {
	return "Pong "
}

func Echo(ctx context.Context, args []string) string {
	return "DERO " + strings.Join(args, " ")
}

/*
//var internal_server = server.NewLocal(assigner,nil) // Use DERO.GetInfo names
var internal_server = server.NewLocal(historical_apis, nil) // uses traditional "getinfo" for compatibility reasons
// Bridge HTTP to the JSON-RPC server.
var bridge = jhttp.NewBridge(internal_server.Client)
*/
var historical_apis = handler.Map{"getinfo": handler.New(GetInfo),
	"get_info":                   handler.New(GetInfo), // this is just an alias to above
	"getblock":                   handler.New(GetBlock),
	"getblockheaderbytopoheight": handler.New(GetBlockHeaderByTopoHeight),
	"getblockheaderbyhash":       handler.New(GetBlockHeaderByHash),
	"gettxpool":                  handler.New(GetTxPool),
	"getrandomaddress":           handler.New(GetRandomAddress),
	"gettransactions":            handler.New(GetTransaction),
	"sendrawtransaction":         handler.New(SendRawTransaction),
	"submitblock":                handler.New(SubmitBlock),
	"getheight":                  handler.New(GetHeight),
	"getblockcount":              handler.New(GetBlockCount),
	"getlastblockheader":         handler.New(GetLastBlockHeader),
	"getblocktemplate":           handler.New(GetBlockTemplate),
	"getencryptedbalance":        handler.New(GetEncryptedBalance),
	"getsc":                      handler.New(GetSC),
	"getgasestimate":             handler.New(GetGasEstimate),
	"nametoaddress":              handler.New(NameToAddress)}

var servicemux = handler.ServiceMap{
	"DERO": handler.Map{
		"Echo":                       handler.New(Echo),
		"Ping":                       handler.New(Ping),
		"GetInfo":                    handler.New(GetInfo),
		"GetBlock":                   handler.New(GetBlock),
		"GetBlockHeaderByTopoHeight": handler.New(GetBlockHeaderByTopoHeight),
		"GetBlockHeaderByHash":       handler.New(GetBlockHeaderByHash),
		"GetTxPool":                  handler.New(GetTxPool),
		"GetRandomAddress":           handler.New(GetRandomAddress),
		"GetTransaction":             handler.New(GetTransaction),
		"SendRawTransaction":         handler.New(SendRawTransaction),
		"SubmitBlock":                handler.New(SubmitBlock),
		"GetHeight":                  handler.New(GetHeight),
		"GetBlockCount":              handler.New(GetBlockCount),
		"GetLastBlockHeader":         handler.New(GetLastBlockHeader),
		"GetBlockTemplate":           handler.New(GetBlockTemplate),
		"GetEncryptedBalance":        handler.New(GetEncryptedBalance),
		"GetSC":                      handler.New(GetSC),
		"GetGasEstimate":             handler.New(GetGasEstimate),
		"NameToAddress":              handler.New(NameToAddress),
	},
	"DAEMON": handler.Map{
		"Echo": handler.New(DAEMON_Echo),
	},
}

type dummyassigner int

var d dummyassigner

func (d dummyassigner) Assign(ctx context.Context, method string) (handler jrpc2.Handler) {
	if handler = servicemux.Assign(ctx, method); handler != nil {
		return
	}
	if handler = historical_apis.Assign(ctx, method); handler != nil {
		return
	}
	return nil
}

func (d dummyassigner) Names() []string {
	names := servicemux.Names()
	hist_names := historical_apis.Names()

	names = append(names, hist_names...)
	sort.Strings(names)
	return names
}

var bridge = jhttp.NewBridge(d, nil)

func translate_http_to_jsonrpc_and_vice_versa(w http.ResponseWriter, r *http.Request) {
	bridge.ServeHTTP(w, r)
}
