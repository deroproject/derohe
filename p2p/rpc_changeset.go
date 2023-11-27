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

import (
	"fmt"

	"github.com/deroproject/derohe/config"
	"github.com/deroproject/graviton"
)

const max_request_topoheights = 50

// notifies inventory
func (c *Connection) ChangeSet(request ChangeList, response *Changes) (err error) {
	defer handle_connection_panic(c)
	if len(request.TopoHeights) < 1 || len(request.TopoHeights) > max_request_topoheights { // we are expecting 1 block or 1 tx
		c.logger.V(1).Info("malformed object request received, banning peer", "request", request)
		c.exit()
		return nil
	}

	c.update(&request.Common) // update common information

	previous_topo := request.TopoHeights[0] - 1 // used to verify the topo heights are in order
	// first requested topo can't be higher than chain AND can't be lower than 10 (because of connection.TopoHeight-50-max_request_topoheights < 10)
	if previous_topo > chain.Load_TOPO_HEIGHT() || previous_topo < 10 {
		c.logger.V(1).Info("malformed object request received, banning peer", "request", request)
		c.exit()
		return fmt.Errorf("invalid topo height for change set request (chain topo = %d, first topo requested = %d)", chain.Load_TOPO_HEIGHT(), previous_topo)
	}

	for _, topo := range request.TopoHeights {
		// Check for well formed requested
		if topo <= previous_topo || previous_topo+1 != topo {
			c.logger.V(1).Info("malformed object request  received, banning peer", "request", request)
			c.exit()
			return fmt.Errorf("invalid topo height for change set request (current = %d, previous = %d)", topo, previous_topo)
		}
		previous_topo = topo

		var cbl Complete_Block

		blid, err := chain.Load_Block_Topological_order_at_index(topo)
		if err != nil {
			return err
		}

		bl, _ := chain.Load_BL_FROM_ID(blid)
		cbl.Block = bl.Serialize()
		for j := range bl.Tx_hashes {
			var tx_bytes []byte
			if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(bl.Tx_hashes[j]); err != nil {
				return err
			}
			cbl.Txs = append(cbl.Txs, tx_bytes) // append all the txs
		}

		cbl.Difficulty = chain.Load_Block_Difficulty(blid).String()

		// now we must load all the changes the block has done to the state tree
		previous_sr, err := chain.Store.Topo_store.Read(topo - 1)
		if err != nil {
			return err
		}
		current_sr, err := chain.Store.Topo_store.Read(topo)
		if err != nil {
			return err
		}

		{ // do the heavy lifting, merge all changes before this topoheight
			var previous_ss, current_ss *graviton.Snapshot

			if previous_ss, err = chain.Store.Balance_store.LoadSnapshot(previous_sr.State_Version); err == nil {
				if current_ss, err = chain.Store.Balance_store.LoadSnapshot(current_sr.State_Version); err == nil {
					if response.KeyCount == 0 {
						var current_balance_tree *graviton.Tree
						if current_balance_tree, err = current_ss.GetTree(config.BALANCE_TREE); err == nil {
							response.KeyCount = current_balance_tree.KeyCountEstimate()
						}
					}
					var changes Tree_Changes
					if changes, err = record_changes(previous_ss, current_ss, config.BALANCE_TREE); err == nil {
						cbl.Changes = append(cbl.Changes, changes)
					}

					if response.SCKeyCount == 0 {
						var current_sc_tree *graviton.Tree
						if current_sc_tree, err = current_ss.GetTree(config.SC_META); err == nil {
							response.SCKeyCount = current_sc_tree.KeyCountEstimate()
						}
					}
					if changes, err = record_changes(previous_ss, current_ss, config.SC_META); err == nil {
						cbl.Changes = append(cbl.Changes, changes)
						// now lets build all the SC changes
						for _, kkey := range cbl.Changes[1].Keys {
							var sc_data Tree_Changes
							//fmt.Printf("bundling SC changes %x\n", k)
							if sc_data, err = record_changes(previous_ss, current_ss, string(kkey)); err == nil {
								cbl.Changes = append(cbl.Changes, sc_data)
							}

						}
					}
				}
			}

			if err != nil {
				return err
			}

			response.CBlocks = append(response.CBlocks, cbl)
		}
	}

	// if everything is OK, we must respond with object response
	fill_common(&response.Common) // fill common info

	return nil
}

// this will record all the changes
func record_changes(previous_ss, current_ss *graviton.Snapshot, treename string) (changes Tree_Changes, err error) {
	var previous_tree, current_tree *graviton.Tree
	if previous_tree, err = previous_ss.GetTree(treename); err == nil {
		if current_tree, err = current_ss.GetTree(treename); err == nil {

			change_handler := func(k, v []byte) {
				changes.Keys = append(changes.Keys, k)
				changes.Values = append(changes.Values, v)
			}
			err = graviton.Diff(previous_tree, current_tree, nil, change_handler, change_handler)
		}
	}

	changes.TreeName = []byte(treename)

	return
}
