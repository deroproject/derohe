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

//import "fmt"
//import "net"

//import "container/list"

import (
	"github.com/stratumfarm/derohe/cryptography/crypto"
	"github.com/stratumfarm/derohe/globals"
)

//import "github.com/deroproject/derosuite/blockchain"

// peer has requested chain

func (c *Connection) Chain(request Chain_Request_Struct, response *Chain_Response_Struct) error {
	defer handle_connection_panic(c)
	if len(request.Block_list) < 1 { // malformed request ban peer
		c.logger.V(3).Info("malformed chain request  received, banning peer", "request", request)
		c.exit()
		return nil
	}

	if len(request.Block_list) != len(request.TopoHeights) || len(request.Block_list) > 1024 {
		c.logger.V(3).Info("Peer chain is invalid", "blocks", len(request.Block_list), "topos", len(request.TopoHeights))
		c.exit()
		return nil
	}

	if request.Block_list[len(request.Block_list)-1] != globals.Config.Genesis_Block_Hash {
		c.logger.V(3).Info("Peer chain is invalid", "blocks", len(request.Block_list), "topos", len(request.TopoHeights))
		c.exit()
		return nil
	}

	// we must give user our version of the chain
	start_height := int64(0)
	start_topoheight := int64(0)

	for i := 0; i < len(request.Block_list); i++ { // find the common point in our chain ( the block is NOT orphan)

		//c.logger.Info("Checking block for chain detection", "i",i, "topo",request.TopoHeights[i], "blid", crypto.Hash(request.Block_list[i]))

		if chain.Block_Exists(request.Block_list[i]) && chain.Is_Block_Topological_order(request.Block_list[i]) &&
			request.TopoHeights[i] == chain.Load_Block_Topological_order(request.Block_list[i]) {
			start_height = chain.Load_Height_for_BL_ID(request.Block_list[i])
			start_topoheight = chain.Load_Block_Topological_order(request.Block_list[i])
			c.logger.V(2).Info("Found common point in chain", "hash", crypto.Hash(request.Block_list[i]), "height", start_height, "topoheight", start_topoheight)
			break
		}
	}

	// we can serve maximum of 512 BLID = 16K KB
	const MAX_BLOCKS = 512

	for i := start_topoheight; i <= chain.Load_TOPO_HEIGHT() && len(response.Block_list) <= MAX_BLOCKS; i++ {
		hash, _ := chain.Load_Block_Topological_order_at_index(i)
		response.Block_list = append(response.Block_list, [32]byte(hash))
	}

	// we must also fill blocks for the  last top 10 heights, so client can sync faster to alt tips
	top_height := chain.Get_Height()
	counter := 0
	for ; top_height > 0 && counter <= 10; top_height-- {
		blocks := chain.Get_Blocks_At_Height(top_height)
		for i := range blocks {
			response.TopBlocks = append([][32]byte{blocks[i]}, response.TopBlocks...) // blocks are ordered height wise
		}
		counter++
	}

	response.Start_height = start_height
	response.Start_topoheight = start_topoheight
	fill_common(&response.Common) // fill common info
	c.update(&request.Common)     // update common information

	return nil
}
