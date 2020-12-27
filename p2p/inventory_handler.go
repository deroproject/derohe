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

package p2p

//import "fmt"
//import "net"
import "sync/atomic"
import "time"

//import "container/list"

import "github.com/romana/rlog"
import "github.com/vmihailenco/msgpack"

//import "github.com/deroproject/derosuite/crypto"
//import "github.com/deroproject/derosuite/globals"

import "github.com/deroproject/derohe/crypto"

//import "github.com/deroproject/derosuite/globals"

//import "github.com/deroproject/derosuite/blockchain"

//  we are sending object request
// right now we only send block ids
func (connection *Connection) Send_Inventory(blids []crypto.Hash, txids []crypto.Hash) {

	var request Object_Request_Struct
	fill_common(&request.Common) // fill common info
	request.Command = V2_NOTIFY_INVENTORY

	for i := range blids {
		request.Block_list = append(request.Block_list, blids[i])
	}

	for i := range txids {
		request.Tx_list = append(request.Tx_list, txids[i])
	}

	if len(blids) > 0 || len(txids) > 0 {
		serialized, err := msgpack.Marshal(&request) // serialize and send
		if err != nil {
			panic(err)
		}

		// use first object

		command := Queued_Command{Command: V2_COMMAND_OBJECTS_RESPONSE, BLID: blids, TXID: txids}

		connection.Objects <- command
		atomic.StoreInt64(&connection.LastObjectRequestTime, time.Now().Unix())

		// we should add to queue that we are waiting for object response
		//command := Queued_Command{Command: V2_COMMAND_OBJECTS_RESPONSE, BLID: blids, TXID: txids, Started: time.Now()}

		connection.Lock()
		//connection.Command_queue.PushBack(command) // queue command
		connection.Send_Message_prelocked(serialized)
		connection.Unlock()
		rlog.Tracef(3, "object request sent contains %d blids %d txids %s ", len(blids), connection.logid)
	}
}

// peer has given his list of inventory
// if certain object is not in our list we request with the inventory
// if everything is already in our inventory, do nothing ignore
func (connection *Connection) Handle_Incoming_Inventory(buf []byte) {
	var request Object_Request_Struct
	var response Object_Request_Struct

	var blids, txids []crypto.Hash

	var dirty = false

	err := msgpack.Unmarshal(buf, &request)
	if err != nil {
		rlog.Warnf("Error while decoding incoming object request err %s %s", err, connection.logid)
		connection.Exit()
	}

	if len(request.Block_list) >= 1 { //  handle incoming blocks list
		for i := range request.Block_list { //
			if !chain.Is_Block_Topological_order(request.Block_list[i]) { // block is not in our chain
				if !chain.Block_Exists(request.Block_list[i]) { // check whether the block can be loaded from disk
					response.Block_list = append(response.Block_list, request.Block_list[i])
					blids = append(blids, request.Block_list[i])
					dirty = true
				}
			}
		}
	}

	if len(request.Tx_list) >= 1 { // handle incoming tx list and see whether it exists in mempoolor regpool
		for i := range request.Tx_list { //
			if !(chain.Mempool.Mempool_TX_Exist(request.Tx_list[i]) || chain.Regpool.Regpool_TX_Exist(request.Tx_list[i])) { // check if is already in mempool skip it
				if _, err = chain.Store.Block_tx_store.ReadTX(request.Tx_list[i]); err != nil { // check whether the tx can be loaded from disk
					response.Tx_list = append(response.Tx_list, request.Tx_list[i])
					txids = append(txids, request.Tx_list[i])
					dirty = true
				}
			}
		}
	}

	if dirty { //  request inventory only if we want it
		fill_common(&response.Common) // fill common info
		response.Command = V2_COMMAND_OBJECTS_REQUEST
		serialized, err := msgpack.Marshal(&response) // serialize and send
		if err != nil {
			panic(err)
		}

		command := Queued_Command{Command: V2_COMMAND_OBJECTS_RESPONSE, BLID: blids, TXID: txids}
		connection.Objects <- command
		atomic.StoreInt64(&connection.LastObjectRequestTime, time.Now().Unix())

		rlog.Tracef(3, "OBJECT REQUEST SENT  sent size %d %s", len(serialized), connection.logid)
		connection.Send_Message(serialized)
	}

}
