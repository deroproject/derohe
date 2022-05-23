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
import "time"
import "context"
import "runtime/debug"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/p2p"

import "github.com/deroproject/derohe/blockchain"

func GetInfo(ctx context.Context) (result rpc.GetInfo_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	//result.Difficulty = chain.Get_Difficulty_At_Block(top_id)
	result.Height = chain.Get_Height()
	result.StableHeight = chain.Get_Stable_Height()
	result.TopoHeight = chain.Load_TOPO_HEIGHT()

	{
		version, err := chain.ReadBlockSnapshotVersion(chain.Get_Top_ID())
		if err != nil {
			panic(err)
		}
		balance_merkle_hash, err := chain.Load_Merkle_Hash(version)
		if err != nil {
			panic(err)
		}
		result.Merkle_Balance_TreeHash = fmt.Sprintf("%X", balance_merkle_hash[:])
	}

	blid, err := chain.Load_Block_Topological_order_at_index(result.TopoHeight)
	if err == nil {
		result.Difficulty = chain.Get_Difficulty_At_Tips(chain.Get_TIPS()).Uint64()
	}

	result.Status = "OK"
	result.Version = config.Version.String()
	result.Top_block_hash = blid.String()
	result.Target = chain.Get_Current_BlockTime()

	if result.TopoHeight-chain.LocatePruneTopo() > 100 {
		blid50, err := chain.Load_Block_Topological_order_at_index(result.TopoHeight - 50)
		if err == nil {
			now := chain.Load_Block_Timestamp(blid)
			now50 := chain.Load_Block_Timestamp(blid50)
			result.AverageBlockTime50 = float32(now-now50) / (50.0 * 1000)
		}
	}

	//result.Target_Height = uint64(chain.Get_Height())

	result.Tx_pool_size = uint64(len(chain.Mempool.Mempool_List_TX()))
	// get dynamic fees per kb, used by wallet for tx creation
	//result.Dynamic_fee_per_kb = config.FEE_PER_KB
	//result.Median_Block_Size = config.CRYPTONOTE_MAX_BLOCK_SIZE

	result.Total_Supply = (config.PREMINE + blockchain.CalcBlockReward(uint64(result.TopoHeight))*uint64(result.TopoHeight)) // valid for few years
	result.Total_Supply = result.Total_Supply / 100000                                                                       // only give deros remove fractional part

	if globals.Config.Name != config.Mainnet.Name { // anything other than mainnet is testnet at this point in time
		result.Testnet = true
	}

	if globals.IsSimulator() {
		result.Network = "Simulator"
	}

	in, out := p2p.Peer_Direction_Count()
	result.Incoming_connections_count = in
	result.Outgoing_connections_count = out
	result.Miners = CountMiners()
	result.Miniblocks_In_Memory = chain.MiniBlocks.Count()
	result.CountMinisRejected = CountMinisRejected
	result.CountMinisAccepted = CountMinisAccepted
	result.CountBlocks = CountBlocks
	result.Mining_Velocity = float64(float64((CountMinisAccepted+CountBlocks)-CountMinisRejected)/time.Now().Sub(globals.StartTime).Seconds()) * 3600
	result.Uptime = uint64(time.Now().Sub(globals.StartTime).Seconds())

	result.HashrateEstimatePercent_1hr = uint64((float64(chain.Get_Network_HashRate()) * HashrateEstimatePercent_1hr()) / 100)
	result.HashrateEstimatePercent_1day = uint64((float64(chain.Get_Network_HashRate()) * HashrateEstimatePercent_1day()) / 100)
	result.HashrateEstimatePercent_7day = uint64((float64(chain.Get_Network_HashRate()) * HashrateEstimatePercent_7day()) / 100)

	return result, nil
}
