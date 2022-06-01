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

	"github.com/deroproject/graviton"
	"github.com/stratumfarm/derohe/cryptography/crypto"
	"github.com/stratumfarm/derohe/globals"
	"github.com/stratumfarm/derohe/rpc"
)

func CheckAddressStatus(ctx context.Context, p rpc.CheckAddressStatusParams) (result rpc.CheckAddressStatusResult) {
	uaddress, err := globals.ParseValidateAddress(p.Address)
	if err != nil {
		panic(err)
	}

	topoheight := chain.Load_TOPO_HEIGHT()
	var balance_tree *graviton.Tree

	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	if err != nil {
		panic(err)
	}

	ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	if err != nil {
		panic(err)
	}

	treename := string(crypto.ZEROHASH[:])
	keyname := uaddress.Compressed()
	if balance_tree, err = ss.GetTree(treename); err != nil {
		panic(err)
	}
	_, _, _, err = balance_tree.GetKeyValueFromKey(keyname)
	var registered bool
	if err == nil {
		registered = true
	}
	return rpc.CheckAddressStatusResult{Registered: registered}
}
