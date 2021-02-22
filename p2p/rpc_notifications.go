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

import "fmt"
import "sync/atomic"
import "encoding/binary"
import "time"
import "github.com/romana/rlog"

import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/transaction"

// notifies inventory
func (c *Connection) NotifyINV(request ObjectList, response *Dummy) (err error) {
	var need ObjectList
	var dirty = false

	c.update(&request.Common) // update common information

	if len(request.Block_list) >= 1 { //  handle incoming blocks list
		for i := range request.Block_list { //
			if !chain.Is_Block_Topological_order(request.Block_list[i]) { // block is not in our chain
				if !chain.Block_Exists(request.Block_list[i]) { // check whether the block can be loaded from disk
					need.Block_list = append(need.Block_list, request.Block_list[i])
					dirty = true
				}
			}
		}
	}

	if len(request.Tx_list) >= 1 { // handle incoming tx list and see whether it exists in mempoolor regpool
		for i := range request.Tx_list { //
			if !(chain.Mempool.Mempool_TX_Exist(request.Tx_list[i]) || chain.Regpool.Regpool_TX_Exist(request.Tx_list[i])) { // check if is already in mempool skip it
				if _, err = chain.Store.Block_tx_store.ReadTX(request.Tx_list[i]); err != nil { // check whether the tx can be loaded from disk
					need.Tx_list = append(need.Tx_list, request.Tx_list[i])
					dirty = true
				}
			}
		}
	}

	if dirty { //  request inventory only if we want it
		var oresponse Objects
		fill_common(&need.Common) // fill common info
		if err = c.RConn.Client.Call("Peer.GetObject", need, &oresponse); err != nil {
			fmt.Printf("Call faileda: %v\n", err)
			c.exit()
			return
		} else { // process the response
			if err = c.process_object_response(oresponse); err != nil {
				return
			}
		}
	}

	fill_common(&response.Common) // fill common info

	return nil

}

// Peer has notified us of a new transaction
func (c *Connection) NotifyTx(request Objects, response *Dummy) error {
	var err error
	var tx transaction.Transaction

	c.update(&request.Common) // update common information

	if len(request.CBlocks) != 0 {
		rlog.Warnf("Error while decoding incoming tx notifcation request err  %s", globals.CTXString(c.logger))
		c.exit()
		return fmt.Errorf("Notify TX cannot notify blocks")
	}

	if len(request.Txs) != 1 {
		rlog.Warnf("Error while decoding incoming tx notification request err %s ", globals.CTXString(c.logger))
		c.exit()
		return fmt.Errorf("Notify TX can only notify 1 tx")
	}

	err = tx.DeserializeHeader(request.Txs[0])
	if err != nil { // we have a tx which could not be deserialized ban peer
		rlog.Warnf("Error Incoming TX could not be deserialized err %s %s", err, globals.CTXString(c.logger))
		c.exit()
		return err
	}

	// track transaction propagation
	if first_time, ok := tx_propagation_map.Load(tx.GetHash()); ok {
		// block already has a reference, take the time and observe the value
		diff := time.Now().Sub(first_time.(time.Time)).Round(time.Millisecond)
		transaction_propagation.Observe(float64(diff / 1000000))
	} else {
		tx_propagation_map.Store(tx.GetHash(), time.Now()) // if this is the first time, store the tx time
	}

	// try adding tx to pool
	if err = chain.Add_TX_To_Pool(&tx); err == nil {
		// add tx to cache  of the peer who sent us this tx
		txhash := tx.GetHash()
		c.TXpool_cache_lock.Lock()
		c.TXpool_cache[binary.LittleEndian.Uint64(txhash[:])] = uint32(time.Now().Unix())
		c.TXpool_cache_lock.Unlock()
	}

	fill_common(&response.Common) // fill common info
	// broadcasting of tx is controlled by mempool
	// broadcast how ???

	return nil

}

func (c *Connection) NotifyBlock(request Objects, response *Dummy) error {
	var err error
	if len(request.CBlocks) != 1 {
		rlog.Warnf("Error while decoding incoming Block notifcation request err %s %s", err, globals.CTXString(c.logger))
		c.exit()
		return fmt.Errorf("Notify Block cannot only notify single block")
	}
	c.update(&request.Common) // update common information

	var cbl block.Complete_Block // parse incoming block and deserialize it
	var bl block.Block
	// lets deserialize block first and see whether it is the requested object
	cbl.Bl = &bl
	err = bl.Deserialize(request.CBlocks[0].Block)
	if err != nil { // we have a block which could not be deserialized ban peer
		rlog.Warnf("Error Incoming block could not be deserilised err %s %s", err, globals.CTXString(c.logger))
		c.exit()
		return err
	}

	blid := bl.GetHash()

	rlog.Infof("Incoming block Notification hash %s %s ", blid, globals.CTXString(c.logger))

	// track block propagation
	if first_time, ok := block_propagation_map.Load(blid); ok {
		// block already has a reference, take the time and observe the value
		diff := time.Now().Sub(first_time.(time.Time)).Round(time.Millisecond)
		block_propagation.Observe(float64(diff / 1000000))
	} else {
		block_propagation_map.Store(blid, time.Now()) // if this is the first time, store the block
	}

	// object is already is in our chain, we need not relay it
	if chain.Is_Block_Topological_order(blid) {
		return nil
	}

	// the block is not in our db,  parse entire block, complete the txs and try to add it
	if len(bl.Tx_hashes) == len(request.CBlocks[0].Txs) {
		c.logger.Debugf("Received a complete block %s with %d transactions", blid, len(bl.Tx_hashes))
		for j := range request.CBlocks[0].Txs {
			var tx transaction.Transaction
			err = tx.DeserializeHeader(request.CBlocks[0].Txs[j])
			if err != nil { // we have a tx which could not be deserialized ban peer
				rlog.Warnf("Error Incoming TX could not be deserialized err %s %s", err, globals.CTXString(c.logger))
				c.exit()
				return err
			}
			cbl.Txs = append(cbl.Txs, &tx)
		}
	} else { // the block is NOT complete, we consider it as an ultra compact block

		c.logger.Debugf("Received an ultra compact block %s, total %d contains %d skipped %d transactions", blid, len(bl.Tx_hashes), len(request.CBlocks[0].Txs), len(bl.Tx_hashes)-len(request.CBlocks[0].Txs))
		for j := range request.CBlocks[0].Txs {
			var tx transaction.Transaction
			err = tx.DeserializeHeader(request.CBlocks[0].Txs[j])
			if err != nil { // we have a tx which could not be deserialized ban peer
				rlog.Warnf("Error Incoming TX could not be deserialized err %s %s", err, globals.CTXString(c.logger))
				c.exit()
				return err
			}
			chain.Add_TX_To_Pool(&tx) // add tx to pool
		}

		// lets build a complete block ( tx from db or mempool )
		for i := range bl.Tx_hashes {

			tx := chain.Mempool.Mempool_Get_TX(bl.Tx_hashes[i])
			if tx != nil {
				cbl.Txs = append(cbl.Txs, tx)
				continue
			} else {
				tx = chain.Regpool.Regpool_Get_TX(bl.Tx_hashes[i])
				if tx != nil {
					cbl.Txs = append(cbl.Txs, tx)
					continue
				}
			}

			var tx_bytes []byte
			if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(bl.Tx_hashes[i]); err != nil {
				// the tx mentioned in ultra compact block could not be found, request a full block
				//connection.Send_ObjectRequest([]crypto.Hash{blid}, []crypto.Hash{})
				logger.Debugf("Ultra compact block  %s missing TX %s, requesting full block", blid, bl.Tx_hashes[i])
				return err
			}

			tx = &transaction.Transaction{}
			if err = tx.DeserializeHeader(tx_bytes); err != nil {
				// the tx mentioned in ultra compact block could not be found, request a full block
				//connection.Send_ObjectRequest([]crypto.Hash{blid}, []crypto.Hash{})
				logger.Debugf("Ultra compact block  %s missing TX %s, requesting full block", blid, bl.Tx_hashes[i])
				return err
			}
			cbl.Txs = append(cbl.Txs, tx) // tx is from disk
		}
	}

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
				atomic.StoreInt64(&c.LastObjectRequestTime, time.Now().Unix())
			}
		}
	}()

	defer func() {
		processing_complete <- true
	}()

	// check if we can add ourselves to chain
	if err, ok := chain.Add_Complete_Block(&cbl); ok { // if block addition was successfil
		// notify all peers
		Broadcast_Block(&cbl, c.Peer_ID) // do not send back to the original peer
	} else { // ban the peer for sometime
		if err == errormsg.ErrInvalidPoW {
			c.logger.Warnf("This peer should be banned and terminated")
			c.exit()
			return err
		}
	}

	fill_common(&response.Common) // fill common info

	return nil
}
