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

import "github.com/romana/rlog"

// peer has requested some objects, we must respond
// if certain object is not in our list we respond with empty buffer for that slot
// an object is either a block or a tx
func (connection *Connection) GetObject(request ObjectList, response *Objects) error {

	var err error
	if len(request.Block_list) < 1 && len(request.Tx_list) < 1 { // we are expecting 1 block or 1 tx
		rlog.Warnf("malformed object request  received, banning peer %+v %s", request)
		connection.exit()
		return nil
	}
	connection.update(&request.Common) // update common information

	for i := range request.Block_list { // find the block
		var cbl Complete_Block
		bl, err := chain.Load_BL_FROM_ID(request.Block_list[i])
		if err != nil {
			return err
		}
		cbl.Block = bl.Serialize()
		for j := range bl.Tx_hashes {
			var tx_bytes []byte
			if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(bl.Tx_hashes[j]); err != nil {
				return err
			}
			cbl.Txs = append(cbl.Txs, tx_bytes) // append all the txs

		}
		response.CBlocks = append(response.CBlocks, cbl)
	}

	for i := range request.Tx_list { // find the common point in our chain
		var tx_bytes []byte
		if tx := chain.Mempool.Mempool_Get_TX(request.Tx_list[i]); tx != nil { // if tx can be satisfied from pool, so be it
			tx_bytes = tx.Serialize()
		} else if tx := chain.Regpool.Regpool_Get_TX(request.Tx_list[i]); tx != nil { // if tx can be satisfied from regpool, so be it
			tx_bytes = tx.Serialize()
		} else if tx_bytes, err = chain.Store.Block_tx_store.ReadTX(request.Tx_list[i]); err != nil {
			return err
		}
		response.Txs = append(response.Txs, tx_bytes) // append all the txs
	}

	// if everything is OK, we must respond with object response
	fill_common(&response.Common) // fill common info

	//rlog.Tracef(3, "OBJECT RESPONSE SENT  sent size %d %s", len(serialized), connection.logid)
	return nil
}
