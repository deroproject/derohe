package p2p

import (
	"sync/atomic"
	"time"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
)

// fill the common part from our chain
func fill_common(common *Common_Struct) {
	var err error
	common.Height = chain.Get_Height()
	//common.StableHeight = chain.Get_Stable_Height()
	common.TopoHeight = chain.Load_TOPO_HEIGHT()

	version, err := chain.ReadBlockSnapshotVersion(chain.Get_Top_ID())
	if err != nil {
		panic(err)
	}

	if common.StateHash, err = chain.Load_Merkle_Hash(version); err != nil {
		panic(err)
	}

	common.Top_Version = uint64(chain.Get_Current_Version_at_Height(int64(common.Height))) // this must be taken from the hardfork
	common.T0 = globals.TimeSkipP2P().UTC().UnixMicro()
}

func fill_common_T1(common *Common_Struct) {
	common.T1 = globals.TimeSkipP2P().UTC().UnixMicro()
}

// sets up data for ntp style processing
func fill_common_T0T1T2(request, response *Common_Struct) {
	response.T0 = request.T0
	response.T1 = request.T1
	response.T2 = globals.TimeSkipP2P().UTC().UnixMicro()
}

// used while sendint TX ASAP
// this also skips statehash
func fill_common_skip_topoheight(common *Common_Struct) {
	fill_common(common)
	return

}

// update some common properties quickly
func (connection *Connection) update(common *Common_Struct) {
	connection.update_received = time.Now()
	var hash crypto.Hash
	atomic.StoreInt64(&connection.Height, common.Height) // satify race detector GOD
	if common.StableHeight != 0 {
		atomic.StoreInt64(&connection.StableHeight, common.StableHeight) // satify race detector GOD
	}
	atomic.StoreInt64(&connection.TopoHeight, common.TopoHeight) // satify race detector GOD

	//connection.Top_ID = common.Top_ID

	if connection.Top_Version != common.Top_Version {
		atomic.StoreUint64(&connection.Top_Version, common.Top_Version) // satify race detector GOD
	}
	if common.StateHash != hash {
		connection.StateHash = common.StateHash
	}

	T3 := globals.TimeSkipP2P().UTC().UnixMicro()

	if common.T0 != 0 && common.T1 != 0 && common.T2 != 0 {
		atomic.StoreInt64(&connection.Latency, int64(rtt_micro(common.T0, common.T1, common.T2, T3)))

		connection.clock_offsets[connection.clock_index] = offset_micro(common.T0, common.T1, common.T2, T3)
		connection.delays[connection.clock_index] = rtt_micro(common.T0, common.T1, common.T2, T3)
		connection.clock_index = (connection.clock_index + 1) % MAX_CLOCK_DATA_SET
		connection.calculate_avg_offset()

		//fmt.Printf("clock index %d\n",connection.clock_index)
	}

	// parse delivered peer list as grey list
	if len(common.PeerList) > 1 {
		connection.logger.V(4).Info("Peer provides peers", "count", len(common.PeerList))
		for i := range common.PeerList {
			if i < 31 {
				Peer_Add(&Peer{Address: common.PeerList[i].Addr, LastConnected: uint64(time.Now().UTC().Unix())})
			}
		}
	}
}

// calculate avg offset
func (connection *Connection) calculate_avg_offset() {
	var total, count time.Duration
	for i := 0; i < MAX_CLOCK_DATA_SET; i++ {
		if connection.clock_offsets[i] != 0 {
			total += connection.clock_offsets[i]
			count++
		}
	}
	connection.clock_offset = int64(total / count)
}
