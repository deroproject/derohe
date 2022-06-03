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

import (
	"context"

	"github.com/stratumfarm/derohe/p2p"
	"github.com/stratumfarm/derohe/rpc"
)

func GetPeers(ctx context.Context) (result *rpc.GetPeersResult) {
	p := p2p.GetPeersInfo()
	return &rpc.GetPeersResult{
		Peers:         toRpcPeers(p.Peers),
		WhitelistSize: p.WhitelistSize,
		GreylistSize:  p.GreylistSize,
	}
}

func toRpcPeers(p []*p2p.Peer) []*rpc.Peer {
	rp := make([]*rpc.Peer, len(p))
	for i, v := range p {
		rp[i] = &rpc.Peer{
			Address:          v.Address,
			ID:               v.ID,
			Miner:            v.Miner,
			LastConnected:    v.LastConnected,
			FailCount:        v.FailCount,
			ConnectAfter:     v.ConnectAfter,
			BlacklistBefore:  v.BlacklistBefore,
			GoodCount:        v.GoodCount,
			Version:          v.Version,
			Whitelist:        v.Whitelist,
			ConnectionStatus: v.ConnectionStatus,
		}
	}
	return rp
}
