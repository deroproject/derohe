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

//import "fmt"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/blockchain"

// this function is only used by the RPC and is not used by the core and should be moved to RPC interface

/* fill up the above structure from the blockchain */
func GetBlockHeader(chain *blockchain.Blockchain, hash crypto.Hash) (result rpc.BlockHeader_Print, err error) {
	bl, err := chain.Load_BL_FROM_ID(hash)
	if err != nil {
		return
	}

	result.TopoHeight = -1
	if chain.Is_Block_Topological_order(hash) {
		result.TopoHeight = chain.Load_Block_Topological_order(hash)
	}
	result.Height = chain.Load_Height_for_BL_ID(hash)
	result.Depth = chain.Get_Height() - result.Height
	result.Difficulty = chain.Load_Block_Difficulty(hash).String()
	result.Hash = hash.String()
	result.Major_Version = uint64(bl.Major_Version)
	result.Minor_Version = uint64(bl.Minor_Version)
	result.Orphan_Status = chain.Is_Block_Orphan(hash)
	if result.TopoHeight >= chain.LocatePruneTopo()+10 { // this result may/may not be valid at just above prune heights
		result.SyncBlock = chain.IsBlockSyncBlockHeight(hash)
	}
	result.SideBlock = chain.Isblock_SideBlock(hash)
	result.Reward = blockchain.CalcBlockReward(uint64(result.Height))
	result.TXCount = int64(len(bl.Tx_hashes))

	for i := range bl.Tips {
		result.Tips = append(result.Tips, bl.Tips[i].String())
	}
	//result.Prev_Hash = bl.Prev_Hash.String()
	result.Timestamp = bl.Timestamp

	return
}
