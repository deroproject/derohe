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

import "github.com/romana/rlog"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/structures"

import "github.com/deroproject/derohe/glue/rwc"

import "github.com/creachadair/jrpc2"
import "github.com/creachadair/jrpc2/channel"
import "nhooyr.io/websocket"

// there should be no global variables, so multiple wallets can run at the same time with different assset

var netClient *http.Client

type Client struct {
	WS  *websocket.Conn
	RPC *jrpc2.Client
}

var rpc_client = &Client{}

var Daemon_Endpoint string

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func Connect(endpoint string) (err error) {

	if globals.Arguments["--remote"] == true && globals.IsMainnet() {
		Daemon_Endpoint = config.REMOTE_DAEMON
	}

	// if user provided endpoint has error, use default
	if Daemon_Endpoint == "" {
		Daemon_Endpoint = "127.0.0.1:" + fmt.Sprintf("%d", config.Mainnet.RPC_Default_Port)
		if !globals.IsMainnet() {
			Daemon_Endpoint = "127.0.0.1:" + fmt.Sprintf("%d", config.Testnet.RPC_Default_Port)
		}
	}

	if globals.Arguments["--daemon-address"] != nil {
		Daemon_Endpoint = globals.Arguments["--daemon-address"].(string)
	}

	Daemon_Endpoint = "127.0.0.1:8080"
	rlog.Infof("Daemon endpoint %s will connect ", Daemon_Endpoint)

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

	rpc_client.WS, _, err = websocket.Dial(context.Background(), "ws://"+Daemon_Endpoint+"/ws", nil)

	// notify user of any state change
	// if daemon connection breaks or comes live again
	if err == nil {
		if !Connected {
			rlog.Infof("Connection to RPC server successful %s", "ws://"+Daemon_Endpoint+"/ws")
			Connected = true
		}
	} else {
		rlog.Errorf("Error executing getinfo_rpc err %s", err)

		if Connected {
			rlog.Warnf("Connection to RPC server Failed err %s endpoint %s ", err, "ws://"+Daemon_Endpoint+"/ws")
		}
		Connected = false

		return
	}

	input_output := rwc.NewNhooyr(rpc_client.WS)
	rpc_client.RPC = jrpc2.NewClient(channel.RawJSON(input_output, input_output), &jrpc2.ClientOptions{OnNotify: Notify_broadcaster})

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
	return nil
}
