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
	"encoding/binary"
	"fmt"
	"runtime/debug"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/graviton"
)

func GetBalancesSC(ctx context.Context, p rpc.GetBalancesSC_Params) (result rpc.GetBalancesSC_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace r %s %s", r, debug.Stack())
		}
	}()

	if p.Assets == nil {
		p.Assets = []crypto.Hash{}
	}

	if p.All && len(p.Assets) != 0 {
		err = fmt.Errorf("cannot specify both all_balances and specific assets in same request")
		return
	}

	if !p.All && len(p.Assets) == 0 {
		err = fmt.Errorf("must specify either all_balances or specific assets in same request")
		return
	}

	result.Balances = map[crypto.Hash]uint64{}

	scid := crypto.HashHexToHash(p.SCID)
	topoheight := chain.Load_TOPO_HEIGHT()

	if p.TopoHeight >= 1 {
		topoheight = p.TopoHeight
	}

	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	if err == nil {
		var ss *graviton.Snapshot
		ss, err = chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
		if err == nil {
			var sc_data_tree *graviton.Tree
			sc_data_tree, err = ss.GetTree(string(scid[:]))
			if err == nil {
				cursor := sc_data_tree.Cursor()
				var k, v []byte
				var asset crypto.Hash
				for k, v, err = cursor.First(); err == nil; k, v, err = cursor.Next() {
					// it's SC balance
					if len(k) == 32 && len(v) == 8 {
						asset = crypto.HashHexToHash(string(k))
						if p.All || IsInSlice(p.Assets, asset) {
							result.Balances[asset] = binary.BigEndian.Uint64(v)
						}
					}
				}
			}
		}
	}

	result.Status = "OK"
	err = nil

	return
}

func IsInSlice(slice []crypto.Hash, value crypto.Hash) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
