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
//import "github.com/vmihailenco/msgpack"

import "github.com/deroproject/derohe/crypto"

//import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/structures"
import "github.com/deroproject/derohe/transaction"

func (DERO_RPC_APIS) GetTransaction(ctx context.Context, p structures.GetTransaction_Params) (result structures.GetTransaction_Result, err error) {

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
				var related structures.Tx_Related_Info

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
			if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(hash); err != nil {
				return
			} else {

				//fmt.Printf("txhash %s loaded %d bytes\n", hash, len(tx_bytes))

				if err = tx.DeserializeHeader(tx_bytes); err != nil {
					logger.Warnf("rpc txhash %s  could not be decoded, err %s\n", hash, err)
					return
				}

				if err == nil {
					var related structures.Tx_Related_Info

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
						related.Block_Height = int64(chain.Load_Block_Topological_order(valid_blid))

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

		{ // we could not fetch the tx, return an empty string
			result.Txs_as_hex = append(result.Txs_as_hex, "")
			err = fmt.Errorf("TX NOT FOUND %s", hash)
			return
		}

	}

	result.Status = "OK"
	err = nil

	//logger.Debugf("result %+v\n", result);
	return
}
