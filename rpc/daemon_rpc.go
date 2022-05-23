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

// this package contains struct definitions and related processing code

package rpc

import "github.com/deroproject/derohe/cryptography/crypto"

// this is used to print blockheader for the rpc and the daemon
type BlockHeader_Print struct {
	Depth         int64    `json:"depth"`
	Difficulty    string   `json:"difficulty"`
	Hash          string   `json:"hash"`
	Height        int64    `json:"height"`
	TopoHeight    int64    `json:"topoheight"`
	Major_Version uint64   `json:"major_version"`
	Minor_Version uint64   `json:"minor_version"`
	Nonce         uint64   `json:"nonce"`
	Orphan_Status bool     `json:"orphan_status"`
	SyncBlock     bool     `json:"syncblock"`
	SideBlock     bool     `json:"sideblock"`
	TXCount       int64    `json:"txcount"`
	Miners        []string `json:"miners"` // note 1 part goes to integrator/remaining is distributed to all

	Reward    uint64   `json:"reward"`
	Tips      []string `json:"tips"`
	Timestamp uint64   `json:"timestamp"`
}

type (
	GetBlockHeaderByTopoHeight_Params struct {
		TopoHeight uint64 `json:"topoheight"`
	}
	GetBlockHeaderByHeight_Result struct {
		Block_Header BlockHeader_Print `json:"block_header"`
		Status       string            `json:"status"`
	}
)

// GetBlockHeaderByHash
type (
	GetBlockHeaderByHash_Params struct {
		Hash string `json:"hash"`
	} // no params
	GetBlockHeaderByHash_Result struct {
		Block_Header BlockHeader_Print `json:"block_header"`
		Status       string            `json:"status"`
	}
)

// get block count
type (
	GetBlockCount_Params struct {
		// NO params
	}
	GetBlockCount_Result struct {
		Count  uint64 `json:"count"`
		Status string `json:"status"`
	}
)

// getblock
type (
	GetBlock_Params struct {
		Hash   string `json:"hash,omitempty"`   // Monero Daemon breaks if both are provided
		Height uint64 `json:"height,omitempty"` // Monero Daemon breaks if both are provided
	} // no params
	GetBlock_Result struct {
		Blob         string            `json:"blob"`
		Json         string            `json:"json"`
		Block_Header BlockHeader_Print `json:"block_header"`
		Status       string            `json:"status"`
	}
)

type (
	NameToAddress_Params struct {
		Name       string `json:"name"`                 // Name for look up
		TopoHeight int64  `json:"topoheight,omitempty"` // lookup in reference to this topo height
	} // no params
	NameToAddress_Result struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Status  string `json:"status"`
	}
)

// get block template request response
type (
	GetBlockTemplate_Params struct {
		Wallet_Address string `json:"wallet_address"`
		Block          bool   `json:"block"`
		Miner          string `json:"miner"`
	}
	GetBlockTemplate_Result struct {
		JobID              string `json:"jobid"`
		Blocktemplate_blob string `json:"blocktemplate_blob,omitempty"`
		Blockhashing_blob  string `json:"blockhashing_blob,omitempty"`
		Difficulty         string `json:"difficulty"`
		Difficultyuint64   uint64 `json:"difficultyuint64"`
		Height             uint64 `json:"height"`
		Prev_Hash          string `json:"prev_hash"`
		EpochMilli         uint64 `json:"epochmilli"`
		Blocks             uint64 `json:"blocks"`     // number of blocks found
		MiniBlocks         uint64 `json:"miniblocks"` // number of miniblocks found
		Rejected           uint64 `json:"rejected"`   // reject count
		LastError          string `json:"lasterror"`  // last error
		Status             string `json:"status"`
	}
)

type ( // array without name containing block template in hex
	SubmitBlock_Params struct {
		JobID                 string `json:"jobid"`
		MiniBlockhashing_blob string `json:"mbl_blob"`
	}
	SubmitBlock_Result struct {
		JobID     string `json:"jobid"`
		MBLID     string `json:"mblid"`
		BLID      string `json:"blid,omitempty"`
		Status    string `json:"status"`
		MiniBlock bool   `json:"mini"` // tells whether minibock was accepted or block was acceppted
	}
)

type (
	GetLastBlockHeader_Params struct{} // no params
	GetLastBlockHeader_Result struct {
		Block_Header BlockHeader_Print `json:"block_header"`
		Status       string            `json:"status"`
	}
)

//get encrypted balance call
type (
	GetEncryptedBalance_Params struct {
		Address                 string      `json:"address"`
		SCID                    crypto.Hash `json:"scid"`
		Merkle_Balance_TreeHash string      `json:"treehash,omitempty"`
		TopoHeight              int64       `json:"topoheight,omitempty"`
	} // no params
	GetEncryptedBalance_Result struct {
		SCID                     crypto.Hash `json:"scid"`
		Data                     string      `json:"data"`         // balance is in hex form, 66 * 2 byte = 132 bytes
		Registration             int64       `json:"registration"` // at what topoheight the account was registered
		Bits                     int         `json:"bits"`         // no. of bits required to access the public key from the chain
		Height                   int64       `json:"height"`       // at what height is this balance
		Topoheight               int64       `json:"topoheight"`   // at what topoheight is this balance
		BlockHash                crypto.Hash `json:"blockhash"`    // blockhash at this topoheight
		Merkle_Balance_TreeHash  string      `json:"treehash"`
		DHeight                  int64       `json:"dheight"`     //  daemon height
		DTopoheight              int64       `json:"dtopoheight"` // daemon topoheight
		DMerkle_Balance_TreeHash string      `json:"dtreehash"`   // daemon dmerkle tree hash
		Status                   string      `json:"status"`
	}
)

type (
	GetTxPool_Params struct{} // no params
	GetTxPool_Result struct {
		Tx_list []string `json:"txs,omitempty"`
		Status  string   `json:"status"`
	}
)

// get height http response as json
type (
	Daemon_GetHeight_Result struct {
		Height       uint64 `json:"height"`
		StableHeight int64  `json:"stableheight"`
		TopoHeight   int64  `json:"topoheight"`

		Status string `json:"status"`
	}
)

type (
	On_GetBlockHash_Params struct {
		X [1]uint64
	}
	On_GetBlockHash_Result struct{}
)

type (
	GetTransaction_Params struct {
		Tx_Hashes []string `json:"txs_hashes"`
		Decode    uint64   `json:"decode_as_json,omitempty"` // Monero Daemon breaks if this sent
	} // no params
	GetTransaction_Result struct {
		Txs_as_hex  []string          `json:"txs_as_hex"`
		Txs_as_json []string          `json:"txs_as_json,omitempty"`
		Txs         []Tx_Related_Info `json:"txs"`
		Status      string            `json:"status"`
	}

	Tx_Related_Info struct {
		As_Hex         string     `json:"as_hex"`
		As_Json        string     `json:"as_json,omitempty"`
		Block_Height   int64      `json:"block_height"`
		Reward         uint64     `json:"reward"`  // miner tx rewards are decided by the protocol during execution
		Ignored        bool       `json:"ignored"` // tell whether this tx is okau as per client protocol or bein ignored
		In_pool        bool       `json:"in_pool"`
		Output_Indices []uint64   `json:"output_indices"`
		Tx_hash        string     `json:"tx_hash"`
		ValidBlock     string     `json:"valid_block"`   // TX is valid in this block
		InvalidBlock   []string   `json:"invalid_block"` // TX is invalid in this block,  0 or more
		Ring           [][]string `json:"ring"`          // ring members completed, since tx contains compressed
		Signer         string     `json:"signer"`        // if signer could be extracted, it will be placed here
		Balance        uint64     `json:"balance"`       // if tx is SC, give SC balance at start
		Code           string     `json:"code"`          // smart contract code at start
		BalanceNow     uint64     `json:"balancenow"`    // if tx is SC, give SC balance at current topo height
		CodeNow        string     `json:"codenow"`       // smart contract code at current topo

	}
)

type (
	GetSC_Params struct {
		SCID       string   `json:"scid"`
		Code       bool     `json:"code,omitempty"`       // if true code will be returned
		Variables  bool     `json:"variables,omitempty"`  // if true all SC variables will be returned
		TopoHeight int64    `json:"topoheight,omitempty"` // all queries are related to this topoheight
		KeysUint64 []uint64 `json:"keysuint64,omitempty"`
		KeysString []string `json:"keysstring,omitempty"`
		KeysBytes  [][]byte `json:"keysbytes,omitempty"` // all keys can also be represented as bytes
	}
	GetSC_Result struct {
		ValuesUint64       []string               `json:"valuesuint64,omitempty"`
		ValuesString       []string               `json:"valuesstring,omitempty"`
		ValuesBytes        []string               `json:"valuesbytes,omitempty"`
		VariableStringKeys map[string]interface{} `json:"stringkeys,omitempty"`
		VariableUint64Keys map[uint64]interface{} `json:"uint64keys,omitempty"`
		Balances           map[string]uint64      `json:"balances,omitempty"`
		Balance            uint64                 `json:"balance"`
		Code               string                 `json:"code"`
		Status             string                 `json:"status"`
	}
)

type (
	GetRandomAddress_Params struct {
		SCID crypto.Hash `json:"scid"`
	}

	GetRandomAddress_Result struct {
		Address []string `json:"address"` // daemon will return around 20 address in 1 go
		Status  string   `json:"status"`
	}
)

type (
	SendRawTransaction_Params struct {
		Tx_as_hex string `json:"tx_as_hex"`
	}
	SendRawTransaction_Result struct {
		Status string `json:"status"`
		Reason string `json:"string"`
	}
)

type (
	GetInfo_Params struct{} // no params
	GetInfo_Result struct {
		Alt_Blocks_Count           uint64  `json:"alt_blocks_count"`
		Difficulty                 uint64  `json:"difficulty"`
		Grey_PeerList_Size         uint64  `json:"grey_peerlist_size"`
		Height                     int64   `json:"height"`
		StableHeight               int64   `json:"stableheight"`
		TopoHeight                 int64   `json:"topoheight"`
		Merkle_Balance_TreeHash    string  `json:"treehash"`
		AverageBlockTime50         float32 `json:"averageblocktime50"`
		Incoming_connections_count uint64  `json:"incoming_connections_count"`
		Outgoing_connections_count uint64  `json:"outgoing_connections_count"`
		Target                     uint64  `json:"target"`
		Target_Height              uint64  `json:"target_height"`
		Testnet                    bool    `json:"testnet"`
		Network                    string  `json:"network"`
		Top_block_hash             string  `json:"top_block_hash"`
		Tx_count                   uint64  `json:"tx_count"`
		Tx_pool_size               uint64  `json:"tx_pool_size"`
		Dynamic_fee_per_kb         uint64  `json:"dynamic_fee_per_kb"`
		Total_Supply               uint64  `json:"total_supply"`
		Median_Block_Size          uint64  `json:"median_block_size"`
		White_peerlist_size        uint64  `json:"white_peerlist_size"`
		Version                    string  `json:"version"`

		Miners               int `json:"connected_miners"`
		Miniblocks_In_Memory int `json:"miniblocks_in_memory"`

		CountBlocks        int64   `json:"blocks_count"`
		CountMinisAccepted int64   `json:"miniblocks_accepted_count"`
		CountMinisRejected int64   `json:"miniblocks_rejected_count"`
		Mining_Velocity    float64 `json:"mining_velocity"`
		Uptime             uint64  `json:"uptime"`

		HashrateEstimatePercent_1hr  uint64 `json:"hashrate_1hr"`
		HashrateEstimatePercent_1day uint64 `json:"hashrate_1d"`
		HashrateEstimatePercent_7day uint64 `json:"hashrate_7d"`

		Status string `json:"status"`
	}
)

type GasEstimate_Params Transfer_Params // same structure as used by transfer call
type GasEstimate_Result struct {
	GasCompute uint64 `json:"gascompute"`
	GasStorage uint64 `json:"gasstorage"`
	Status     string `json:"status"`
}
