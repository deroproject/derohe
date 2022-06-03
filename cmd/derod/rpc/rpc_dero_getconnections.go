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

func GetConnections(ctx context.Context) (result rpc.GetConnectionResult) {
	return rpc.GetConnectionResult{
		Connections: toRpcConnections(p2p.GetConnections()),
	}
}

func toRpcConnections(c []*p2p.Connection) []*rpc.Connection {
	rc := make([]*rpc.Connection, len(c))
	for i, v := range c {
		rc[i] = &rpc.Connection{
			Height:                v.Height,
			StableHeight:          v.StableHeight,
			TopoHeight:            v.TopoHeight,
			Pruned:                v.Pruned,
			LastObjectRequestTime: v.LastObjectRequestTime,
			Latency:               v.Latency,
			BytesIn:               v.BytesIn,
			BytesOut:              v.BytesOut,
			Top_Version:           v.Top_Version,
			Peer_ID:               v.Peer_ID,
			Port:                  v.Port,
			State:                 v.State,
			Syncing:               v.Syncing,
			StateHash:             v.StateHash.String(),
			Created:               v.Created.String(),
			Incoming:              v.Incoming,
			Addr:                  v.Addr.String(),
			SyncNode:              v.SyncNode,
			ProtocolVersion:       v.ProtocolVersion,
			Tag:                   v.Tag,
			DaemonVersion:         v.DaemonVersion,
			Top_ID:                v.Top_ID.String(),
		}
	}
	return nil
}
