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

import "time"

var timeout = 5 * time.Second
var timer = time.NewTimer(time.Millisecond)

// this function continously turns connectivity online/offline
// avoid connectivity calls when possible
func Keep_Connectivity() {
	Connect("")
	for {
		select {
		//case <- w.quit:
		//	return
		case <-timer.C: // we disconnected and did not connect, this timer fires every 5 secs,

			timer.Reset(timeout)
			if !Connected {
				Connect("")
			} else {
				if IsDaemonOnline() {
					var result string
					if err := rpc_client.Call("DERO.Ping", nil, &result); err != nil {
						// fmt.Printf("Ping failed: %v", err)
						rpc_client.RPC.Close()
						rpc_client.WS = nil
						rpc_client.RPC = nil
						Connected = false
						Connect("") // try to connect again

					} else {
						//fmt.Printf("Pong Received %s\n", result)
					}
				}
			}
		}

	}

}
