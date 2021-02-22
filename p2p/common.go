package p2p

import "math/big"
import "sync/atomic"
import "github.com/romana/rlog"

import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/cryptography/crypto"

// fill the common part from our chain
func fill_common(common *Common_Struct) {
	common.Height = chain.Get_Height()
	//common.StableHeight = chain.Get_Stable_Height()
	common.TopoHeight = chain.Load_TOPO_HEIGHT()
	//common.Top_ID, _ = chain.Load_BL_ID_at_Height(common.Height - 1)

	high_block, err := chain.Load_Block_Topological_order_at_index(common.TopoHeight)
	if err != nil {
		common.Cumulative_Difficulty = "0"
	} else {
		common.Cumulative_Difficulty = chain.Load_Block_Cumulative_Difficulty(high_block).String()
	}

	if common.StateHash, err = chain.Load_Merkle_Hash(common.TopoHeight); err != nil {
		panic(err)
	}

	common.Top_Version = uint64(chain.Get_Current_Version_at_Height(int64(common.Height))) // this must be taken from the hardfork

}

// used while sendint TX ASAP
// this also skips statehash
func fill_common_skip_topoheight(common *Common_Struct) {
	fill_common(common)
	return

}

// update some common properties quickly
func (connection *Connection) update(common *Common_Struct) {
	//connection.Lock()
	//defer connection.Unlock()
	var hash crypto.Hash
	atomic.StoreInt64(&connection.Height, common.Height) // satify race detector GOD
	if common.StableHeight != 0 {
		atomic.StoreInt64(&connection.StableHeight, common.StableHeight) // satify race detector GOD
	}
	atomic.StoreInt64(&connection.TopoHeight, common.TopoHeight) // satify race detector GOD

	//connection.Top_ID = common.Top_ID
	if common.Cumulative_Difficulty != "" {
		connection.Cumulative_Difficulty = common.Cumulative_Difficulty

		var x *big.Int
		x = new(big.Int)
		if _, ok := x.SetString(connection.Cumulative_Difficulty, 10); !ok { // if Cumulative_Difficulty could not be parsed, kill connection
			rlog.Warnf("Could not Parse Cumulative_Difficulty in common '%s' \"%s\" ", connection.Cumulative_Difficulty, globals.CTXString(connection.logger))
			connection.exit()
		}

		connection.CDIFF.Store(x) // do it atomically
	}

	if connection.Top_Version != common.Top_Version {
		atomic.StoreUint64(&connection.Top_Version, common.Top_Version) // satify race detector GOD
	}
	if common.StateHash != hash {
		connection.StateHash = common.StateHash
	}

}
