//go:build !wasm
// +build !wasm

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
//import "fmt"

//import "net/url"
import (
	"strings"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/deroproject/derohe/glue/rwc"
	"github.com/gorilla/websocket"
)

// there should be no global variables, so multiple wallets can run at the same time with different assset

type Client struct {
	WS  *websocket.Conn
	RPC *jrpc2.Client
}

var rpc_client = &Client{}

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func Connect(endpoint string) (err error) {

	var daemon_uri string

	// Take globals arg configured daemon if endpoint param is not defined
	if endpoint == "" {
		Daemon_Endpoint_Active = get_daemon_address()
	} else {
		Daemon_Endpoint_Active = endpoint
	}

	logger.V(1).Info("Daemon endpoint ", "address", Daemon_Endpoint_Active)

	// Trim off http, https, wss, ws to get endpoint to use for connecting
	if strings.HasPrefix(Daemon_Endpoint_Active, "https") {
		ld := strings.TrimPrefix(strings.ToLower(Daemon_Endpoint_Active), "https://")
		daemon_uri = "wss://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else if strings.HasPrefix(Daemon_Endpoint_Active, "http") {
		ld := strings.TrimPrefix(strings.ToLower(Daemon_Endpoint_Active), "http://")
		daemon_uri = "ws://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else if strings.HasPrefix(Daemon_Endpoint_Active, "wss") {
		ld := strings.TrimPrefix(strings.ToLower(Daemon_Endpoint_Active), "wss://")
		daemon_uri = "wss://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else if strings.HasPrefix(Daemon_Endpoint_Active, "ws") {
		ld := strings.TrimPrefix(strings.ToLower(Daemon_Endpoint_Active), "ws://")
		daemon_uri = "ws://" + ld + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	} else {
		daemon_uri = "ws://" + Daemon_Endpoint_Active + "/ws"

		rpc_client.WS, _, err = websocket.DefaultDialer.Dial(daemon_uri, nil)
	}

	// notify user of any state change
	// if daemon connection breaks or comes live again
	if err == nil {
		if !Connected {
			logger.V(1).Info("Connection to RPC server successful", "address", daemon_uri)
			Connected = true
		}
	} else {

		if Connected {
			logger.V(1).Error(err, "Connection to RPC server Failed", "endpoint", daemon_uri)
		}
		Connected = false
		return
	}

	input_output := rwc.New(rpc_client.WS)
	rpc_client.RPC = jrpc2.NewClient(channel.RawJSON(input_output, input_output), &jrpc2.ClientOptions{OnNotify: Notify_broadcaster})

	return test_connectivity()
}
