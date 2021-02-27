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
import "time"
import "sync/atomic"

//import "container/list"

//import log "github.com/sirupsen/logrus"
import "github.com/romana/rlog"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/transaction"

//import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derosuite/blockchain"

// we are expecting other side to have a heavier PoW chain, try to sync now
func (connection *Connection) sync_chain() {

	var request Chain_Request_Struct
	var response Chain_Response_Struct

try_again:

	// send our blocks, first 10 blocks directly, then decreasing in powers of 2
	start_point := chain.Load_TOPO_HEIGHT()
	for i := int64(0); i < start_point; {

		tr, err := chain.Store.Topo_store.Read(start_point - i) // commit everything
		if err != nil {
			continue
		}
		if tr.IsClean() {
			break
		}

		request.Block_list = append(request.Block_list, tr.BLOCK_ID)
		request.TopoHeights = append(request.TopoHeights, start_point-i)
		switch {
		case len(request.Block_list) < 10:
			i++
		default:
			i = i * 2
		}
	}
	// add genesis block at the end
	request.Block_list = append(request.Block_list, globals.Config.Genesis_Block_Hash)
	request.TopoHeights = append(request.TopoHeights, 0)
	fill_common(&request.Common) // fill common info
	if err := connection.RConn.Client.Call("Peer.Chain", request, &response); err != nil {
		rlog.Debugf("Call failed  Peer.Chain : %v \n", err)
		return
	}
	// we have a response, see if its valid and try to add to get the blocks

	rlog.Infof("Peer wants to give chain from topoheight %d ", response.Start_height)
	_ = config.STABLE_LIMIT

	// we do not need reorganisation if deviation is less than  or equak to 7 blocks
	// only pop blocks if the system has somehow deviated more than 7 blocks
	// if the deviation is less than 7 blocks, we internally reorganise everything
	if chain.Get_Height()-response.Start_height > config.STABLE_LIMIT && connection.SyncNode {
		// get our top block
		rlog.Infof("rewinding status our %d  peer %d", chain.Load_TOPO_HEIGHT(), response.Start_topoheight)
		pop_count := chain.Load_TOPO_HEIGHT() - response.Start_topoheight
		chain.Rewind_Chain(int(pop_count)) // pop as many blocks as necessary

		// we should NOT queue blocks, instead we sent our chain request again
		goto try_again

	} else if chain.Get_Height()-response.Start_height <= config.STABLE_LIMIT {
		pop_count := chain.Load_TOPO_HEIGHT() - response.Start_topoheight
		chain.Rewind_Chain(int(pop_count)) // pop as many blocks as necessary, assumming peer has given us good chain
	} else { // we must somehow notify that deviation is way too much and manual interaction is necessary, so as any bug for chain deviationmay be detected
		return
	}

	// response only 128 blocks at a time
	max_blocks_to_queue := 128
	// check whether the objects are in our db or not
	// until we put in place a parallel object tracker, do it one at a time

	rlog.Infof("response block list %d\n", len(response.Block_list))
	for i := range response.Block_list {
		our_topo_order := chain.Load_Block_Topological_order(response.Block_list[i])
		if our_topo_order != (int64(i)+response.Start_topoheight) || chain.Load_Block_Topological_order(response.Block_list[i]) == -1 { // if block is not in our chain, add it to request list
			//queue_block(request.Block_list[i])
			if max_blocks_to_queue >= 0 {
				max_blocks_to_queue--
				//connection.Send_ObjectRequest([]crypto.Hash{response.Block_list[i]}, []crypto.Hash{})
				var orequest ObjectList
				var oresponse Objects

				orequest.Block_list = append(orequest.Block_list, response.Block_list[i])
				fill_common(&orequest.Common)
				if err := connection.RConn.Client.Call("Peer.GetObject", orequest, &oresponse); err != nil {
					rlog.Debugf("Call faileda Peer.GetObject: %v\n", err)
					return
				} else { // process the response
					if err = connection.process_object_response(oresponse); err != nil {
						return
					}
				}

				//fmt.Printf("Queuing block %x height %d  %s", response.Block_list[i], response.Start_height+int64(i), connection.logid)
			}
		} else {
			rlog.Tracef(3, "We must have queued %x, but we skipped it at height %d", response.Block_list[i], response.Start_height+int64(i))
		}
	}

	// request alt-tips ( blocks if we are nearing the main tip )
	if (response.Common.TopoHeight - chain.Load_TOPO_HEIGHT()) <= 5 {
		for i := range response.TopBlocks {
			if chain.Load_Block_Topological_order(response.TopBlocks[i]) == -1 {
				//connection.Send_ObjectRequest([]crypto.Hash{response.TopBlocks[i]}, []crypto.Hash{})
				rlog.Tracef(2, "Queuing ALT-TIP  block %x %s", response.TopBlocks[i], connection.logid)

			}

		}
	}

}

func (connection *Connection) process_object_response(response Objects) error {
	var err error

	// make sure connection does not timeout and be killed while processing huge blocks
	processing_complete := make(chan bool)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-processing_complete:
				return // complete the loop
			case <-ticker.C: // give the chain some more time to respond
				atomic.StoreInt64(&connection.LastObjectRequestTime, time.Now().Unix())
			}
		}
	}()

	defer func() {
		processing_complete <- true
	}()

	for i := 0; i < len(response.CBlocks); i++ { // process incoming full blocks
		var cbl block.Complete_Block // parse incoming block and deserialize it
		var bl block.Block
		// lets deserialize block first and see whether it is the requested object
		cbl.Bl = &bl
		err := bl.Deserialize(response.CBlocks[i].Block)
		if err != nil { // we have a block which could not be deserialized ban peer
			rlog.Warnf("Error Incoming block could not be deserilised err %s %s", err, connection.logid)
			connection.exit()
			return nil
		}

		// give the chain some more time to respond
		atomic.StoreInt64(&connection.LastObjectRequestTime, time.Now().Unix())

		// check whether the object was requested one

		// complete the txs
		for j := range response.CBlocks[i].Txs {
			var tx transaction.Transaction
			err = tx.DeserializeHeader(response.CBlocks[i].Txs[j])
			if err != nil { // we have a tx which could not be deserialized ban peer
				rlog.Warnf("Error Incoming TX could not be deserialized err %s %s", err, connection.logid)
				connection.exit()

				return nil
			}
			cbl.Txs = append(cbl.Txs, &tx)
		}

		// check if we can add ourselves to chain
		err, ok := chain.Add_Complete_Block(&cbl)
		if !ok && err == errormsg.ErrInvalidPoW {
			connection.logger.Warnf("This peer should be banned")
			connection.exit()
			return nil
		}

		if !ok && err == errormsg.ErrPastMissing {
			rlog.Warnf("Error Incoming Block coould not be added due to missing past, so skipping future block err %s %s", err, connection.logid)
			return nil
		}

		// add the object to object pool from where it will be consume
		// queue_block_received(bl.GetHash(),&cbl)

	}

	for i := range response.Txs { // process incoming txs for mempool
		var tx transaction.Transaction
		err = tx.DeserializeHeader(response.Txs[i])
		if err != nil { // we have a tx which could not be deserialized ban peer
			rlog.Warnf("Error Incoming TX could not be deserialized err %s %s", err, connection.logid)
			connection.exit()

			return nil
		}
		if !chain.Mempool.Mempool_TX_Exist(tx.GetHash()) { // we still donot have it, so try to process it

			chain.Add_TX_To_Pool(&tx) // currently we are ignoring error
		}
	}
	return nil
}
