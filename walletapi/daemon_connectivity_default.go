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

import (
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/deroproject/derohe/glue/rwc"
	"github.com/gorilla/websocket"
)

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func (c *Client) Connect() (err error) {

	logger.V(1).Info("Daemon endpoint ", "address", c.endpoint)

	c.WS, _, err = websocket.DefaultDialer.Dial("ws://"+c.endpoint+"/ws", nil)

	// notify user of any state change
	// if daemon connection breaks or comes live again
	if err == nil {
		if !c.IsConnected() {
			logger.V(1).Info("Connection to RPC server successful", "address", "ws://"+c.endpoint+"/ws")
			c.SetConnected(true)
		}
	} else {

		if c.IsConnected() {
			logger.V(1).Error(err, "Connection to RPC server Failed", "endpoint", "ws://"+c.endpoint+"/ws")
		}
		c.SetConnected(false)
		return
	}

	input_output := rwc.New(c.WS)
	c.RPC = jrpc2.NewClient(channel.RawJSON(input_output, input_output), &jrpc2.ClientOptions{OnNotify: c.Notify_broadcaster})

	return c.test_connectivity()
}
