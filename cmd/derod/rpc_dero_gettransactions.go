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

package main

import "fmt"
import "context"
import "encoding/hex"
import "runtime/debug"

//import "github.com/romana/rlog"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/blockchain"
import "github.com/deroproject/graviton"

func (DERO_RPC_APIS) GetTransaction(ctx context.Context, p rpc.GetTransaction_Params) (result rpc.GetTransaction_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	for i := 0; i < len(p.Tx_Hashes); i++ {

		hash := crypto.HashHexToHash(p.Tx_Hashes[i])

		// check whether we can get the tx from the pool
		{
			tx := chain.Mempool.Mempool_Get_TX(hash)

			// logger.Debugf("checking tx in pool %+v", tx);
			if tx != nil { // found the tx in the mempool
				var related rpc.Tx_Related_Info

				related.Block_Height = -1 // not mined
				related.In_pool = true

				result.Txs_as_hex = append(result.Txs_as_hex, hex.EncodeToString(tx.Serialize()))
				result.Txs = append(result.Txs, related)

				continue // no more processing required
			}
		}

		{ // check if tx is from  blockchain
			var tx transaction.Transaction
			var tx_bytes []byte
			if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(hash); err != nil { // if tx not found return empty rpc
				var related rpc.Tx_Related_Info
				result.Txs_as_hex = append(result.Txs_as_hex, "") // a not found tx will return ""
				result.Txs = append(result.Txs, related)
				continue
			} else {

				//fmt.Printf("txhash %s loaded %d bytes\n", hash, len(tx_bytes))

				if err = tx.DeserializeHeader(tx_bytes); err != nil {
					logger.Warnf("rpc txhash %s  could not be decoded, err %s\n", hash, err)
					return
				}

				if err == nil {
					var related rpc.Tx_Related_Info

					// check whether tx is orphan

					//if chain.Is_TX_Orphan(hash) {
					//	result.Txs_as_hex = append(result.Txs_as_hex, "") // given empty data
					//	result.Txs = append(result.Txs, related)          // should we have an orphan tx marker
					//} else

					if tx.IsCoinbase() { // fill reward but only for coinbase
						//blhash, err := chain.Load_Block_Topological_order_at_index(nil, int64(related.Block_Height))
						//if err == nil { // if err return err
						related.Reward = 999999 //chain.Load_Block_Total_Reward(nil, blhash)
						//}
					}

					// also fill where the tx is found and in which block is valid and in which it is invalid
					blid_list,state_block,state_block_topo := chain.IS_TX_Mined(hash)

					//logger.Infof(" tx %s related info valid_blid %s invalid_blid %+v valid %v ",hash, valid_blid, invalid_blid, valid)

					if state_block_topo > 0 {
						related.StateBlock = state_block.String()						
						related.Block_Height = state_block_topo

						if tx.TransactionType != transaction.REGISTRATION {
							// we must now fill in compressed ring members
							if toporecord, err := chain.Store.Topo_store.Read(state_block_topo); err == nil {
								if ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version); err == nil {

									if tx.TransactionType == transaction.SC_TX {
										scid := tx.GetHash()
										if tx.SCDATA.Has(rpc.SCACTION, rpc.DataUint64) && rpc.SC_INSTALL == rpc.SC_ACTION(tx.SCDATA.Value(rpc.SCACTION, rpc.DataUint64).(uint64)) {
											if sc_meta_tree, err := ss.GetTree(config.SC_META); err == nil {
												var meta_bytes []byte
												if meta_bytes, err = sc_meta_tree.Get(blockchain.SC_Meta_Key(scid)); err == nil {
													var meta blockchain.SC_META_DATA // the meta contains the link to the SC bytes
													if err = meta.UnmarshalBinary(meta_bytes); err == nil {
														related.Balance = meta.Balance
													}
												}
											}
											if sc_data_tree, err := ss.GetTree(string(scid[:])); err == nil {
												var code_bytes []byte
												if code_bytes, err = sc_data_tree.Get(blockchain.SC_Code_Key(scid)); err == nil {
													related.Code = string(code_bytes)

												}

											}

										}
									}

									for t := range tx.Payloads {
										var ring [][]byte

										var tree *graviton.Tree

										if tx.Payloads[t].SCID.IsZero() {
											tree, err = ss.GetTree(config.BALANCE_TREE)
										} else {
											tree, err = ss.GetTree(string(tx.Payloads[t].SCID[:]))
										}

										if err != nil {
											fmt.Printf("no such SC %s\n", tx.Payloads[t].SCID)
										}

										for j := 0; j < int(tx.Payloads[t].Statement.RingSize); j++ {
											key_pointer := tx.Payloads[t].Statement.Publickeylist_pointers[j*int(tx.Payloads[t].Statement.Bytes_per_publickey) : (j+1)*int(tx.Payloads[t].Statement.Bytes_per_publickey)]
											_, key_compressed, _, err := tree.GetKeyValueFromHash(key_pointer)
											if err == nil {
												ring = append(ring, key_compressed)
											} else { // we should some how report error
												fmt.Printf("Error expanding member for txid %s t %d err %s key_compressed %x\n", hash, t, err, key_compressed)
											}
										}
										related.Ring = append(related.Ring, ring)
									}

								}
							}
						}
					}
					for i := range blid_list {
						related.MinedBlock = append(related.MinedBlock, blid_list[i].String())
					}

					result.Txs_as_hex = append(result.Txs_as_hex, hex.EncodeToString(tx.Serialize()))
					result.Txs = append(result.Txs, related)
				}
				continue
			}
		}

		{ // we could not fetch the tx, return an empty string
			result.Txs_as_hex = append(result.Txs_as_hex, "")
			err = fmt.Errorf("TX NOT FOUND %s", hash)
			return
		}

	}

	result.Status = "OK"
	err = nil

	//logger.Debugf("result %+v\n", result)
	return
}
