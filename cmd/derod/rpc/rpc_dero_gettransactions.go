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

package rpc

import "fmt"
import "context"
import "encoding/hex"
import "encoding/binary"
import "runtime/debug"

//import "github.com/romana/rlog"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/cryptography/bn256"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/dvm"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/blockchain"

func GetTransaction(ctx context.Context, p rpc.GetTransaction_Params) (result rpc.GetTransaction_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	for i := 0; i < len(p.Tx_Hashes); i++ {

		hash := crypto.HashHexToHash(p.Tx_Hashes[i])

		{ // check if tx is from  blockchain
			var tx transaction.Transaction
			var tx_bytes []byte
			if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(hash); err != nil { // if tx not found return empty rpc

				// check whether we can get the tx from the pool
				{
					tx := chain.Mempool.Mempool_Get_TX(hash)
					if tx != nil { // found the tx in the mempool
						var related rpc.Tx_Related_Info

						related.Block_Height = -1 // not mined
						related.In_pool = true

						result.Txs_as_hex = append(result.Txs_as_hex, hex.EncodeToString(tx.Serialize()))
						result.Txs = append(result.Txs, related)
					} else {
						var related rpc.Tx_Related_Info
						result.Txs_as_hex = append(result.Txs_as_hex, "") // a not found tx will return ""
						result.Txs = append(result.Txs, related)
					}
				}
				continue // no more processing required
			} else {

				//fmt.Printf("txhash %s loaded %d bytes\n", hash, len(tx_bytes))

				if err = tx.Deserialize(tx_bytes); err != nil {
					//logger.Warnf("rpc txhash %s  could not be decoded, err %s\n", hash, err)
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

					valid_blid, invalid_blid, valid := chain.IS_TX_Valid(hash)

					//logger.Infof(" tx %s related info valid_blid %s invalid_blid %+v valid %v ",hash, valid_blid, invalid_blid, valid)

					if valid {
						related.ValidBlock = valid_blid.String()

						// topo height at which it was mined
						topo_height := int64(chain.Load_Block_Topological_order(valid_blid))
						related.Block_Height = topo_height

						if tx.TransactionType != transaction.REGISTRATION {
							// we must now fill in compressed ring members
							if toporecord, err := chain.Store.Topo_store.Read(topo_height); err == nil {
								if ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version); err == nil {

									if tx.TransactionType == transaction.SC_TX {
										scid := tx.GetHash()
										if tx.SCDATA.Has(rpc.SCACTION, rpc.DataUint64) && rpc.SC_INSTALL == rpc.SC_ACTION(tx.SCDATA.Value(rpc.SCACTION, rpc.DataUint64).(uint64)) {

											if sc_data_tree, err := ss.GetTree(string(scid[:])); err == nil {
												var code_bytes []byte
												if code_bytes, err = sc_data_tree.Get(dvm.SC_Code_Key(scid)); err == nil {
													related.Code = string(code_bytes)

												}

												var zerohash crypto.Hash
												if balance_bytes, err := sc_data_tree.Get(zerohash[:]); err == nil {
													if len(balance_bytes) == 8 {
														related.Balance = binary.BigEndian.Uint64(balance_bytes[:])
													}
												}

											}

										}
									}

									// expand the tx, no need to do proof checking
									err = chain.Expand_Transaction_NonCoinbase(&tx)
									if err != nil {
										return result, err
									}

									for t := range tx.Payloads {
										var ring []string
										for j := 0; j < int(tx.Payloads[t].Statement.RingSize); j++ {
											astring := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[j]))
											astring.Mainnet = globals.Config.Name == config.Mainnet.Name
											ring = append(ring, astring.String())

										}
										related.Ring = append(related.Ring, ring)
									}

									if signer, err1 := blockchain.Extract_signer(&tx); err1 == nil {
										var p bn256.G1
										if err = p.DecodeCompressed(signer[:]); err == nil {
											s := rpc.NewAddressFromKeys((*crypto.Point)(&p))
											s.Mainnet = globals.Config.Name == config.Mainnet.Name
											related.Signer = s.String()
										}
									}

								}
							}
						}
					}
					for i := range invalid_blid {
						related.InvalidBlock = append(related.InvalidBlock, invalid_blid[i].String())
					}

					result.Txs_as_hex = append(result.Txs_as_hex, hex.EncodeToString(tx.Serialize()))
					result.Txs = append(result.Txs, related)
				}
				continue
			}
		}
	}

	result.Status = "OK"
	err = nil

	//logger.Debugf("result %+v\n", result)
	return
}
