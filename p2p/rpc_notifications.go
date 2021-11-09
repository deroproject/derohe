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

import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/metrics"
import "github.com/deroproject/derohe/globals"

// handles notifications of inventory
func (c *Connection) NotifyINV(request ObjectList, response *Dummy) (err error) {
	defer handle_connection_panic(c)
	var need ObjectList
	var dirty = false

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

			// track transaction propagation
			if request.Sent != 0 && request.Sent < globals.Time().UTC().UnixMicro() {
				time_to_receive := float64(globals.Time().UTC().UnixMicro()-request.Sent) / 1000000
				metrics.Set.GetOrCreateHistogram("tx_propagation_duration_histogram_seconds").Update(time_to_receive)
			}
			if !(chain.Mempool.Mempool_TX_Exist(request.Tx_list[i]) || chain.Regpool.Regpool_TX_Exist(request.Tx_list[i])) { // check if is already in mempool skip it
				if _, err = chain.Store.Block_tx_store.ReadTX(request.Tx_list[i]); err != nil { // check whether the tx can be loaded from disk
					need.Tx_list = append(need.Tx_list, request.Tx_list[i])
					dirty = true
				}
			}
		}
	}

	// cchunk list ids are 65 bytes long
	if len(request.Chunk_list) >= 1 { // handle incoming chunks list and see whether we have the chunks
		for i := range request.Chunk_list { //
			var blid, hhash [32]byte
			copy(blid[:], request.Chunk_list[i][:])
			cid := uint8(request.Chunk_list[i][32])
			copy(hhash[:], request.Chunk_list[i][33:])

			// track chunk propagation
			if request.Sent != 0 && request.Sent < globals.Time().UTC().UnixMicro() {
				time_to_receive := float64(globals.Time().UTC().UnixMicro()-request.Sent) / 1000000
				metrics.Set.GetOrCreateHistogram("chunk_propagation_duration_histogram_seconds").Update(time_to_receive)
			}

			if !chain.Block_Exists(blid) { // check whether the block can be loaded from disk
				if nil == is_chunk_exist(hhash, cid) { // if chunk does not exist
					//					fmt.Printf("requesting chunk %d INV %x\n",cid, request.Chunk_list[i][:])
					need.Chunk_list = append(need.Chunk_list, request.Chunk_list[i])
					dirty = true
				}
			}
		}
	}

	if dirty { //  request inventory only if we want it
		var oresponse Objects
		fill_common(&need.Common) // fill common info
		//need.Sent = request.Sent // send time
		if err = c.RConn.Client.Call("Peer.GetObject", need, &oresponse); err != nil {
			c.logger.V(2).Error(err, "Call failed GetObject", err)
			c.exit()
			return
		} else { // process the response
			if err = c.process_object_response(oresponse, request.Sent, false); err != nil {
				return
			}
		}
	}

	c.update(&request.Common)     // update common information
	fill_common(&response.Common) // fill common info

	return nil

}

// Peer has notified us of a new transaction
func (c *Connection) NotifyTx(request Objects, response *Dummy) error {
	defer handle_connection_panic(c)
	var err error
	var tx transaction.Transaction

	c.update(&request.Common) // update common information

	if len(request.CBlocks) != 0 {
		err = fmt.Errorf("Notify TX cannot notify blocks")
		c.logger.V(3).Error(err, "Should be banned")
		c.exit()
		return err
	}

	if len(request.Txs) != 1 {
		err = fmt.Errorf("Notify TX can only notify 1 tx")
		c.logger.V(3).Error(err, "Should be banned")
		c.exit()
		return err
	}

	err = tx.Deserialize(request.Txs[0])
	if err != nil { // we have a tx which could not be deserialized ban peer
		c.logger.V(3).Error(err, "Should be banned")
		c.exit()
		return err
	}

	// track transaction propagation
	if request.Sent != 0 && request.Sent < globals.Time().UTC().UnixMicro() {
		time_to_receive := float64(globals.Time().UTC().UnixMicro()-request.Sent) / 1000000
		metrics.Set.GetOrCreateHistogram("tx_propagation_duration_histogram_seconds").Update(time_to_receive)
	}

	// try adding tx to pool
	if err = chain.Add_TX_To_Pool(&tx); err == nil {
		// add tx to cache  of the peer who sent us this tx
		txhash := tx.GetHash()
		c.TXpool_cache_lock.Lock()
		c.TXpool_cache[binary.LittleEndian.Uint64(txhash[:])] = uint32(time.Now().Unix())
		c.TXpool_cache_lock.Unlock()
		broadcast_Tx(&tx, 0, request.Sent)
	}

	fill_common(&response.Common) // fill common info

	return nil
}

// only miniblocks carry extra info, which leads to better time tracking
func (c *Connection) NotifyMiniBlock(request Objects, response *Dummy) (err error) {
	defer handle_connection_panic(c)

	if len(request.MiniBlocks) != 1 {
		err = fmt.Errorf("Notify Block can notify single block")
		c.logger.V(3).Error(err, "Should be banned")
		c.exit()
		return err
	}
	fill_common_T1(&request.Common)
	c.update(&request.Common) // update common information

	for i := range request.MiniBlocks {
		var mbl block.MiniBlock
		var ok bool

		if err = mbl.Deserialize(request.MiniBlocks[i]); err != nil {
			return err
		}

		if mbl.Timestamp > uint64(globals.Time().UTC().UnixMilli())+50 { // 50 ms passing allowed
			return errormsg.ErrInvalidTimestamp
		}

		// track miniblock propagation
		if request.Sent != 0 && request.Sent < globals.Time().UTC().UnixMicro() {
			time_to_receive := float64(globals.Time().UTC().UnixMicro()-request.Sent) / 1000000
			metrics.Set.GetOrCreateHistogram("miniblock_propagation_duration_histogram_seconds").Update(time_to_receive)
		}

		// first check whether it is already in the chain
		if chain.MiniBlocks.IsCollision(mbl) {
			continue // miniblock already in chain, so skip it
		}

		// first check whether the incoming minblock can be added to sub chains
		if !chain.MiniBlocks.IsConnected(mbl) {
			c.logger.V(3).Error(err, "Disconnected miniblock")
			return fmt.Errorf("Disconnected miniblock")
		}

		var miner_hash crypto.Hash
		copy(miner_hash[:], mbl.KeyHash[:])
		if !chain.IsAddressHashValid(false, miner_hash) { // this will use cache
			c.logger.V(3).Error(err, "unregistered miner")
			return fmt.Errorf("unregistered miner")
		}

		// check whether the genesis blocks are all equal
		genesis_list := chain.MiniBlocks.GetGenesisFromMiniBlock(mbl)

		var bl block.Block
		if len(genesis_list) >= 1 {
			bl.Height = binary.BigEndian.Uint64(genesis_list[0].Check[:])

			var tip1, tip2 crypto.Hash
			copy(tip1[:], genesis_list[0].Check[8:8+12])
			bl.Tips = append(bl.Tips, tip1)

			if genesis_list[0].PastCount == 2 {
				copy(tip2[:], genesis_list[0].Check[8+12:])
				bl.Tips = append(bl.Tips, tip2)
			}

		} else {
			c.logger.V(3).Error(nil, "no genesis, we cannot do anything")
			continue
		}

		for i, tip := range bl.Tips { // tips are currently only partial,  lets expand tips
			if ehash, ok := chain.ExpandMiniBlockTip(tip); ok {
				bl.Tips[i] = ehash
			} else {
				return fmt.Errorf("tip could not be expanded")
			}
		}

		if bl.Height >= 2 && !chain.CheckDagStructure(bl.Tips) {
			return fmt.Errorf("Invalid DAG structure")
		}

		bl.MiniBlocks = append(bl.MiniBlocks, chain.MiniBlocks.GetEntireMiniBlockHistory(mbl)...)

		if err = chain.Verify_MiniBlocks(bl); err != nil { // whether the structure is okay
			return err
		}

		// lets get the difficulty at tips
		if !chain.VerifyMiniblockPoW(&bl, mbl) {
			return errormsg.ErrInvalidPoW
		}

		if err, ok = chain.InsertMiniBlock(mbl); !ok {
			return err
		} else { // rebroadcast miniblock
			broadcast_MiniBlock(mbl, c.Peer_ID, request.Sent) // do not send back to the original peer
		}
	}
	fill_common(&response.Common)                         // fill common info
	fill_common_T0T1T2(&request.Common, &response.Common) // fill time related information
	return nil
}

func (c *Connection) NotifyBlock(request Objects, response *Dummy) error {
	defer handle_connection_panic(c)
	var err error
	if len(request.CBlocks) != 1 {
		err = fmt.Errorf("Notify Block cannot only notify single block")
		c.logger.V(3).Error(err, "Should be banned")
		c.exit()
		return err
	}
	c.update(&request.Common) // update common information

	err = c.processChunkedBlock(request, true, false, 0, 0)
	if err != nil { // we have a block which could not be deserialized ban peer
		return err
	}

	fill_common(&response.Common) // fill common info
	return nil
}

func (c *Connection) processChunkedBlock(request Objects, isnotified bool, waschunked bool, data_shard_count, parity_shard_count int) error {
	var err error

	var cbl block.Complete_Block // parse incoming block and deserialize it
	var bl block.Block
	// lets deserialize block first and see whether it is the requested object
	cbl.Bl = &bl
	err = bl.Deserialize(request.CBlocks[0].Block)
	if err != nil { // we have a block which could not be deserialized ban peer
		c.logger.V(3).Error(err, "Block cannot be deserialized.Should be banned")
		c.exit()
		return err
	}

	blid := bl.GetHash()

	// track block propagation only if its notified
	if isnotified { // otherwise its tracked in chunks
		if request.Sent != 0 && request.Sent < globals.Time().UTC().UnixMicro() {
			time_to_receive := float64(globals.Time().UTC().UnixMicro()-request.Sent) / 1000000
			metrics.Set.GetOrCreateHistogram("block_propagation_duration_histogram_seconds").Update(time_to_receive)
		}
	}

	// object is already is in our chain, we need not relay it
	if chain.Is_Block_Topological_order(blid) {
		return nil
	}

	// the block is not in our db,  parse entire block, complete the txs and try to add it
	if len(bl.Tx_hashes) == len(request.CBlocks[0].Txs) {
		c.logger.V(2).Info("Received a complete block", "blid", blid, "txcount", len(bl.Tx_hashes))
		for j := range request.CBlocks[0].Txs {
			var tx transaction.Transaction
			err = tx.Deserialize(request.CBlocks[0].Txs[j])
			if err != nil { // we have a tx which could not be deserialized ban peer
				c.logger.V(3).Error(err, "tx cannot be deserialized.Should be banned")
				c.exit()
				return err
			}
			cbl.Txs = append(cbl.Txs, &tx)
		}

		// fill all shards which we might be missing but only we received it in chunk
		if waschunked {
			convert_block_to_chunks(&cbl, data_shard_count, parity_shard_count)
		}

	} else { // the block is NOT complete, we consider it as an ultra compact block
		c.logger.V(2).Info("Received an ultra compact  block", "blid", blid, "txcount", len(bl.Tx_hashes), "skipped", len(bl.Tx_hashes)-len(request.CBlocks[0].Txs))
		for j := range request.CBlocks[0].Txs {
			var tx transaction.Transaction
			err = tx.Deserialize(request.CBlocks[0].Txs[j])
			if err != nil { // we have a tx which could not be deserialized ban peer
				c.logger.V(3).Error(err, "tx cannot be deserialized.Should be banned")
				c.exit()
				return err
			}
			chain.Add_TX_To_Pool(&tx) // add tx to pool
		}

		// lets build a complete block ( tx from db or mempool )
		for i := range bl.Tx_hashes {

			retry_count := 0
		retry_tx:

			if retry_count > 10 {
				err = fmt.Errorf("TX %s could not be obtained after %d tries", bl.Tx_hashes[i], retry_count)
				c.logger.V(3).Error(err, "Missing TX")
			}
			retry_count++

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

				err = c.request_tx([][32]byte{bl.Tx_hashes[i]}, isnotified)
				if err == nil {
					goto retry_tx
				}
				//connection.Send_ObjectRequest([]crypto.Hash{blid}, []crypto.Hash{})
				//logger.Debugf("Ultra compact block  %s missing TX %s, requesting full block", blid, bl.Tx_hashes[i])
				return err
			}

			tx = &transaction.Transaction{}
			if err = tx.Deserialize(tx_bytes); err != nil {
				err = c.request_tx([][32]byte{bl.Tx_hashes[i]}, isnotified)
				if err == nil {
					goto retry_tx
				}
				// the tx mentioned in ultra compact block could not be found, request a full block
				//connection.Send_ObjectRequest([]crypto.Hash{blid}, []crypto.Hash{})
				//logger.Debugf("Ultra compact block  %s missing TX %s, requesting full block", blid, bl.Tx_hashes[i])
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
			c.logger.Error(err, "This peer should be banned and terminated")
			c.exit()
			return err
		}
	}

	return nil
}
