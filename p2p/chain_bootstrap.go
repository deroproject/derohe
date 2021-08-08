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

//import "net"
import "time"
import "math/big"
import "math/bits"
import "sync/atomic"
import "encoding/binary"

//import "container/list"

//import log "github.com/sirupsen/logrus"
import "github.com/romana/rlog"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"

//import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/transaction"

import "github.com/deroproject/graviton"

import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derosuite/blockchain"

// we are expecting other side to have a heavier PoW chain
// this is for the case when the chain only moves in pruned state
// if after bootstraping the chain can continousky sync for few minutes, this means we have got the job done
func (connection *Connection) bootstrap_chain() {

	var request ChangeList
	var response Changes
	// var err error
	var zerohash crypto.Hash

	// we will request top 60 blocks
	ctopo := connection.TopoHeight
	var topos []int64
	for i := ctopo - 200; i < ctopo; i++ {
		topos = append(topos, i)
	}

	connection.logger.Infof("Bootstrap Initiated")

	for i := range topos {
		request.TopoHeights = append(request.TopoHeights, topos[i])
	}

	fill_common(&request.Common) // fill common info
	if err := connection.RConn.Client.Call("Peer.ChangeSet", request, &response); err != nil {
		rlog.Errorf("Call faileda ChangeSet: %v\n", err)
		return
	}
	// we have a response, see if its valid and try to add to get the blocks
	rlog.Infof("changeset received, estimated keys : %d  SC Keys : %d \n", response.KeyCount, response.SCKeyCount)

	commit_version := uint64(0)

	{ // fetch and commit balance tree

		chunksize := int64(640)
		chunks_estm := response.KeyCount / chunksize
		chunks := int64(1) // chunks need to be in power of 2
		path_length := 0
		for chunks < chunks_estm {
			chunks = chunks * 2
			path_length++
		}

		if chunks < 2 {
			chunks = 2
			path_length = 1
		}

		var section [8]byte

		total_keys := 0

		for i := int64(0); i < chunks; i++ {
			binary.BigEndian.PutUint64(section[:], bits.Reverse64(uint64(i))) // place reverse path
			ts_request := Request_Tree_Section_Struct{Topo: request.TopoHeights[0], TreeName: []byte(config.BALANCE_TREE), Section: section[:], SectionLength: uint64(path_length)}
			var ts_response Response_Tree_Section_Struct
			fill_common(&ts_response.Common)
			if err := connection.RConn.Client.Call("Peer.TreeSection", ts_request, &ts_response); err != nil {
				connection.logger.Warnf("Call failed TreeSection: %v\n", err)
				return
			} else {
				// now we must write all the state changes to gravition
				var balance_tree *graviton.Tree
				if ss, err := chain.Store.Balance_store.LoadSnapshot(0); err != nil {
					panic(err)
				} else if balance_tree, err = ss.GetTree(config.BALANCE_TREE); err != nil {
					panic(err)
				}

				if len(ts_response.Keys) != len(ts_response.Values) {
					rlog.Warnf("Incoming Key count %d value count %d \"%s\" ", len(ts_response.Keys), len(ts_response.Values), globals.CTXString(connection.logger))
					connection.exit()
					return
				}
				rlog.Debugf("chunk %d Will write %d keys\n", i, len(ts_response.Keys))

				for j := range ts_response.Keys {
					balance_tree.Put(ts_response.Keys[j], ts_response.Values[j])
				}
				total_keys += len(ts_response.Keys)

				commit_version, err = graviton.Commit(balance_tree)
				if err != nil {
					panic(err)
				}

				h, err := balance_tree.Hash()
				rlog.Debugf("total keys %d hash %x err %s\n", total_keys, h, err)

			}
			connection.logger.Infof("Bootstrap %3.1f%% completed", float32(i*100)/float32(chunks))
		}
	}

	{ // fetch and commit SC tree

		chunksize := int64(640)
		chunks_estm := response.SCKeyCount / chunksize
		chunks := int64(1) // chunks need to be in power of 2
		path_length := 0
		for chunks < chunks_estm {
			chunks = chunks * 2
			path_length++
		}

		if chunks < 2 {
			chunks = 2
			path_length = 1
		}

		var section [8]byte

		total_keys := 0

		for i := int64(0); i < chunks; i++ {
			binary.BigEndian.PutUint64(section[:], bits.Reverse64(uint64(i))) // place reverse path
			ts_request := Request_Tree_Section_Struct{Topo: request.TopoHeights[0], TreeName: []byte(config.SC_META), Section: section[:], SectionLength: uint64(path_length)}
			var ts_response Response_Tree_Section_Struct
			fill_common(&ts_response.Common)
			if err := connection.RConn.Client.Call("Peer.TreeSection", ts_request, &ts_response); err != nil {
				connection.logger.Warnf("Call failed TreeSection: %v\n", err)
				return
			} else {
				// now we must write all the state changes to gravition
				var changed_trees []*graviton.Tree
				var sc_tree *graviton.Tree
				//var changed_trees []*graviton.Tree
				ss, err := chain.Store.Balance_store.LoadSnapshot(0)
				if err != nil {
					panic(err)
				} else if sc_tree, err = ss.GetTree(config.SC_META); err != nil {
					panic(err)
				}

				if len(ts_response.Keys) != len(ts_response.Values) {
					rlog.Warnf("Incoming Key count %d value count %d \"%s\" ", len(ts_response.Keys), len(ts_response.Values), globals.CTXString(connection.logger))
					connection.exit()
					return
				}
				rlog.Debugf("SC chunk %d Will write %d keys\n", i, len(ts_response.Keys))

				for j := range ts_response.Keys {
					sc_tree.Put(ts_response.Keys[j], ts_response.Values[j])

					// we must fetch each individual SC tree

					sc_request := Request_Tree_Section_Struct{Topo: request.TopoHeights[0], TreeName: ts_response.Keys[j], Section: section[:], SectionLength: uint64(0)}
					var sc_response Response_Tree_Section_Struct
					fill_common(&sc_response.Common)
					if err := connection.RConn.Client.Call("Peer.TreeSection", sc_request, &sc_response); err != nil {
						connection.logger.Warnf("Call failed TreeSection: %v\n", err)
						return
					} else {
						var sc_data_tree *graviton.Tree
						if sc_data_tree, err = ss.GetTree(string(ts_response.Keys[j])); err != nil {
							panic(err)
						} else {
							for j := range sc_response.Keys {
								sc_data_tree.Put(sc_response.Keys[j], sc_response.Values[j])
							}
							changed_trees = append(changed_trees, sc_data_tree)

						}

					}

				}
				total_keys += len(ts_response.Keys)
				changed_trees = append(changed_trees, sc_tree)
				commit_version, err = graviton.Commit(changed_trees...)
				if err != nil {
					panic(err)
				}

				h, err := sc_tree.Hash()
				rlog.Debugf("total SC keys %d hash %x err %s\n", total_keys, h, err)

			}
			connection.logger.Infof("Bootstrap %3.1f%% completed", float32(i*100)/float32(chunks))
		}
	}

	for i := int64(0); i <= request.TopoHeights[0]; i++ {
		chain.Store.Topo_store.Write(i, zerohash, commit_version, 0) // commit everything
	}

	for i := range response.CBlocks { // we must store the blocks

		var cbl block.Complete_Block // parse incoming block and deserialize it
		var bl block.Block
		// lets deserialize block first and see whether it is the requested object
		cbl.Bl = &bl
		err := bl.Deserialize(response.CBlocks[i].Block)
		if err != nil { // we have a block which could not be deserialized ban peer
			connection.logger.Warnf("Error Incoming block could not be deserilised err %s %s", err, connection.logid)
			connection.exit()
			return
		}

		// give the chain some more time to respond
		atomic.StoreInt64(&connection.LastObjectRequestTime, time.Now().Unix())

		if i == 0 { // whatever datastore we have written, its state hash must match
			// ToDo

		}

		// complete the txs
		for j := range response.CBlocks[i].Txs {
			var tx transaction.Transaction
			err = tx.DeserializeHeader(response.CBlocks[i].Txs[j])
			if err != nil { // we have a tx which could not be deserialized ban peer
				rlog.Warnf("Error Incoming TX could not be deserialized err %s %s", err, connection.logid)
				connection.exit()
				return
			}
			if bl.Tx_hashes[j] != tx.GetHash() {
				rlog.Warnf("Error Incoming TX has mismatch err %s %s", err, connection.logid)
				connection.exit()
				return
			}

			cbl.Txs = append(cbl.Txs, &tx)
		}

		{ // first lets save all the txs, together with their link to this block as height
			for i := 0; i < len(cbl.Txs); i++ {
				if err = chain.Store.Block_tx_store.WriteTX(bl.Tx_hashes[i], cbl.Txs[i].Serialize()); err != nil {
					panic(err)
				}
			}
		}

		diff := new(big.Int)
		if _, ok := diff.SetString(response.CBlocks[i].Difficulty, 10); !ok { // if Cumulative_Difficulty could not be parsed, kill connection
			rlog.Warnf("Could not Parse Difficulty in common %s \"%s\" ", connection.Cumulative_Difficulty, globals.CTXString(connection.logger))
			connection.exit()
			return
		}

		cdiff := new(big.Int)
		if _, ok := cdiff.SetString(response.CBlocks[i].Cumulative_Difficulty, 10); !ok { // if Cumulative_Difficulty could not be parsed, kill connection
			rlog.Warnf("Could not Parse Cumulative_Difficulty in common %s \"%s\" ", connection.Cumulative_Difficulty, globals.CTXString(connection.logger))
			connection.exit()
			return
		}

		if err = chain.Store.Block_tx_store.WriteBlock(bl.GetHash(), bl.Serialize(), diff, cdiff); err != nil {
			panic(fmt.Sprintf("error while writing block"))
		}

		// now we must write all the state changes to gravition

		var ss *graviton.Snapshot
		if ss, err = chain.Store.Balance_store.LoadSnapshot(0); err != nil {
			panic(err)
		}

		/*if len(response.CBlocks[i].Keys) != len(response.CBlocks[i].Values) {
			rlog.Warnf("Incoming Key count %d value count %d \"%s\" ", len(response.CBlocks[i].Keys), len(response.CBlocks[i].Values), globals.CTXString(connection.logger))
			connection.exit()
			return
		}*/

		write_count := 0
		commit_version := ss.GetVersion()
		if i != 0 {

			var changed_trees []*graviton.Tree

			for _, change := range response.CBlocks[i].Changes {
				var tree *graviton.Tree
				if tree, err = ss.GetTree(string(change.TreeName)); err != nil {
					panic(err)
				}

				for j := range change.Keys {
					tree.Put(change.Keys[j], change.Values[j])
					write_count++
				}
				changed_trees = append(changed_trees, tree)
			}
			commit_version, err = graviton.Commit(changed_trees...)
			if err != nil {
				panic(err)
			}
		}

		rlog.Debugf("%d wrote %d keys commit version %d\n", request.TopoHeights[i], write_count, commit_version)

		chain.Store.Topo_store.Write(request.TopoHeights[i], bl.GetHash(), commit_version, int64(bl.Height)) // commit everything
	}

	connection.logger.Infof("Bootstrap completed successfully")
	// load the chain from the disk
	chain.Initialise_Chain_From_DB()
	chain.Sync = true
	return
}
