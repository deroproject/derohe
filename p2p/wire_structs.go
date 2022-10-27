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

import "fmt"
import "strings"
import "github.com/deroproject/derohe/cryptography/crypto"

// This file defines the structure for the protocol which is CBOR ( which is standard) stream multiplexed using yamux
// stream multiplexing allows us have bidirection RPC using net/rpc
// the p2p package is currently the most complex within the entire project

// the protocol is partly syncronous, partly asyncronous

// used to parse incoming packet for for command , so as a repective command command could be triggered
type Common_Struct struct {
	Height       int64       `cbor:"HEIGHT"`
	TopoHeight   int64       `cbor:"THEIGHT"`
	StableHeight int64       `cbor:"SHEIGHT"`
	StateHash    [32]byte    `cbor:"STATE"`
	PeerList     []Peer_Info `cbor:"PLIST,omitempty"` // it will contain peerlist every 30 minutes
	T0           int64       `cbor:"T0,omitempty"`    // see https://en.wikipedia.org/wiki/Network_Time_Protocol
	T1           int64       `cbor:"T1,omitempty"`    // time when this was sent, in unixmicro
	T2           int64       `cbor:"T2,omitempty"`    // time when this was sent, in unixmicro
	Top_Version  uint64      `cbor:"HF"`              // this basically represents the hard fork version
}

type Dummy struct { // empty strcut returned
	Common Common_Struct `cbor:"COMMON"` // add all fields of Common
}

// at start, client sends handshake and server will respond to handshake
type Handshake_Struct struct {
	Common          Common_Struct `cbor:"COMMON"`   // add all fields of Common
	ProtocolVersion string        `cbor:"PVERSION"` // version is a sematic version string semver
	Tag             string        `cbor:"TAG"`      // user specific tag
	DaemonVersion   string        `cbor:"DVERSION"`
	UTC_Time        int64         `cbor:"UTC"`
	Local_Port      uint32        `cbor:"LP"`
	Peer_ID         uint64        `cbor:"PID"`
	Pruned          int64         `cbor:"PRUNED"`
	Network_ID      [16]byte      `cbor:"NID"` // 16 bytes
	Flags           []string      `cbor:"FLAGS"`
	PeerList        []Peer_Info   `cbor:"PLIST"`
	Extension_List  []string      `cbor:"EXT"`
	Request         bool          `cbor:"REQUEST"` //whether this is a request
}

type Peer_Info struct {
	Addr  string `cbor:"ADDR"` // ip:port pair
	Miner bool   `cbor:"MINER"`
	//ID       uint64 `cbor:"I"`
	//LastSeen uint64 `cbor:"LS"`
}

type Chain_Request_Struct struct { // our version of chain
	Common      Common_Struct `cbor:"COMMON"` // add all fields of Common
	Block_list  [][32]byte    `cbor:"BLIST"`  // block list
	TopoHeights []int64       `cbor:"TOPO"`   // topo heights of added blocks
}

type Chain_Response_Struct struct { // peers gives us point where to get the chain
	Common           Common_Struct `cbor:"COMMON"` // add all fields of Common
	Start_height     int64         `cbor:"SH"`
	Start_topoheight int64         `cbor:"STH"`
	Block_list       [][32]byte    `cbor:"BLIST"`
	TopBlocks        [][32]byte    `cbor:"TOPBLOCKS"` // top blocks used for faster syncronisation of alt-tips
	// this contains all blocks hashes for the last 10 heights, heightwise ordered
}

type ObjectList struct {
	Common     Common_Struct       `cbor:"COMMON"`         // add all fields of Common
	Sent       int64               `cbor:"SENT,omitempty"` // this is timestamp in microsecs only filled in notifications, and must be passed down
	Block_list [][32]byte          `cbor:"BLIST,omitempty"`
	Tx_list    [][32]byte          `cbor:"TXLIST,omitempty"`
	Chunk_list [][32 + 1 + 32]byte `cbor:"CLIST,omitempty"` // CLIST, first is block id, last byte is chunkid, max 255  chunks supported
}

type Objects struct {
	Common     Common_Struct    `cbor:"COMMON"`         // add all fields of Common
	Sent       int64            `cbor:"SENT,omitempty"` // this is timestamp in microsecs only filled in notifications, and must be passed down
	CBlocks    []Complete_Block `cbor:"CBLOCKS,omitempty"`
	Txs        [][]byte         `cbor:"TXS,omitempty"`
	MiniBlocks [][]byte         `cbor:"MBLS,omitempty"`   // miniblocks
	Chunks     []Block_Chunk    `cbor:"CHUNKS,omitempty"` // all requested chunks are here
}

// used to request what all changes are done by the block to the chain
type ChangeList struct {
	Common      Common_Struct `cbor:"COMMON"` // add all fields of Common
	TopoHeights []int64       `cbor:"TOPO"`
}

type Changes struct {
	Common     Common_Struct    `cbor:"COMMON"` // add all fields of Common
	CBlocks    []Complete_Block `cbor:"CBLOCKS"`
	KeyCount   int64            `cbor:"KEYCOUNT"`
	SCKeyCount int64            `cbor:"SCKEYCOUNT"`
}

type Request_Tree_Section_Struct struct {
	Common        Common_Struct `cbor:"COMMON"`             // add all fields of Common
	Topo          int64         `cbor:"TOPO"`               // request section from the balance tree of this topo
	TreeName      []byte        `cbor:"TREENAME,omitempty"` // changes to state tree
	Section       []byte        `cbor:"SECTION"`            // section path from which data must be received
	SectionLength uint64        `cbor:"SECTIONL"`           // section length in bits
}

type Response_Tree_Section_Struct struct {
	Common        Common_Struct `cbor:"COMMON"`             // add all fields of Common
	Topo          int64         `cbor:"TOPO"`               // request section from the balance tree of this topo
	TreeName      []byte        `cbor:"TREENAME,omitempty"` // changes to state tree
	Section       []byte        `cbor:"SECTION"`
	SectionLength uint64        `cbor:"SECTIONL"` // section length in bits
	StateHash     [32]byte      `cbor:"STATE"`
	Keys          [][]byte      `cbor:"KEYS,omitempty"`   // changes to state tree
	Values        [][]byte      `cbor:"VALUES,omitempty"` // changes to state tree
	KeyCount      int64         `cbor:"KEYCOUNT"`         // estimated keys in this tree
}

type Tree_Changes struct {
	TreeName []byte   `cbor:"TREENAME,omitempty"` // changes to state tree
	Keys     [][]byte `cbor:"KEYS,omitempty"`     // changes to state tree
	Values   [][]byte `cbor:"VALUES,omitempty"`   // changes to state tree
}

type Complete_Block struct {
	Block      []byte         `cbor:"BLOCK,omitempty"`
	Txs        [][]byte       `cbor:"TXS,omitempty"`
	Difficulty string         `cbor:"DIFF,omitempty"`    // Diff
	Changes    []Tree_Changes `cbor:"CHANGES,omitempty"` // changes to state tree
}

type Block_Chunk struct {
	Type        uint16   `cbor:"T"`     // used for denote object type
	HHash       [32]byte `cbor:"BLH"`   // used for checks
	BLID        [32]byte `cbor:"BLID"`  // blid for which this chunk belongs
	DSIZE       uint     `cbor:"DSIZE"` // Datasize in bytes
	BLOCK       []byte   `cbor:"BL"`    // block itself,together with miniblock, and txhashes, txs are part of data
	CHUNK_COUNT uint     `cbor:"CC"`    // total chunks
	CHUNK_NEED  uint     `cbor:"CN"`    // chunks needed to decode the data
	CHUNK_HASH  []uint64 `cbor:"CH"`    // all chubk hashes truncated to 8 bytes
	CHUNK_ID    uint     `cbor:"CID"`   // this chunkid, 0 based
	CHUNK_DATA  []byte   `cbor:"CD"`    // chunkdata
}

type TXSET struct {
	Txs [][]byte `cbor:"TXS"`
}

func (x *Block_Chunk) HeaderHash() [32]byte {
	var chunk_hashes []string

	for i := range x.CHUNK_HASH {
		chunk_hashes = append(chunk_hashes, fmt.Sprintf("%d", x.CHUNK_HASH[i]))
	}

	input_data := fmt.Sprintf("%d-%x-%d-%d-%d-%s", x.Type, x.BLID[:], x.DSIZE, x.CHUNK_COUNT, x.CHUNK_NEED, strings.Join(chunk_hashes, ","))
	//fmt.Printf("input_data %s\n", input_data)
	return crypto.Keccak256(x.BLOCK, []byte(input_data))
}
