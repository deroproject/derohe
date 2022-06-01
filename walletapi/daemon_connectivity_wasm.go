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

import "context"
import "net/http"

import "github.com/stratumfarm/derohe/glue/rwc"

import "github.com/creachadair/jrpc2"
import "github.com/creachadair/jrpc2/channel"
import "nhooyr.io/websocket"
import "strings"

// there should be no global variables, so multiple wallets can run at the same time with different assset

var netClient *http.Client

type Client struct {
	WS  *websocket.Conn
	RPC *jrpc2.Client
}

var rpc_client = &Client{}

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func Connect(endpoint string) (err error) {

	Daemon_Endpoint_Active = get_daemon_address()

	logger.V(1).Info("Daemon endpoint ", "address", Daemon_Endpoint_Active)

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

	if strings.HasPrefix(Daemon_Endpoint, "https") {
		ld := strings.TrimPrefix(strings.ToLower(Daemon_Endpoint), "https://")
		fmt.Printf("will use endpoint %s\n", "wss://"+ld+"/ws")
		rpc_client.WS, _, err = websocket.Dial(context.Background(), "wss://"+ld+"/ws", nil)
	} else {
		fmt.Printf("will use endpoint %s\n", "ws://"+Daemon_Endpoint+"/ws")
		rpc_client.WS, _, err = websocket.Dial(context.Background(), "ws://"+Daemon_Endpoint+"/ws", nil)
	}

	// notify user of any state change
	// if daemon connection breaks or comes live again
	if err == nil {
		if !Connected {
			logger.V(1).Info("Connection to RPC server successful", "address", "ws://"+Daemon_Endpoint+"/ws")
			fmt.Printf("successfully connected\n")
			Connected = true
		}
	} else {
		if Connected {
			logger.V(1).Error(err, "Connection to RPC server Failed", "endpoint", "ws://"+Daemon_Endpoint+"/ws")
		}
		Connected = false
		fmt.Printf("connection to endpoint failed.err %s\n", err)
		return
	}

	input_output := rwc.NewNhooyr(rpc_client.WS)
	rpc_client.RPC = jrpc2.NewClient(channel.RawJSON(input_output, input_output), &jrpc2.ClientOptions{OnNotify: Notify_broadcaster})

	return test_connectivity()
}
