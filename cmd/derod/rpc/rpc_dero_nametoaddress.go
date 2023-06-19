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
	"fmt"
	"runtime/debug"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/dvm"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
)

//import "github.com/deroproject/derosuite/blockchain"

func NameToAddress(ctx context.Context, p rpc.NameToAddress_Params) (result rpc.NameToAddress_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	topoheight := chain.Load_TOPO_HEIGHT()

	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	if err != nil {
		panic(err)
	}
	ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	if err != nil {
		panic(err)
	}

	var zerohash crypto.Hash
	zerohash[31] = 1
	treename := string(zerohash[:])

	tree, err := ss.GetTree(treename)
	if err != nil {
		panic(err)
	}

	var value_bytes []byte
	if value_bytes, err = tree.Get(dvm.Variable{Type: dvm.String, ValueString: p.Name}.MarshalBinaryPanic()); err == nil {

		var v dvm.Variable
		if err = v.UnmarshalBinary(value_bytes); err != nil {
			return
		}

		addr, _ := rpc.NewAddressFromCompressedKeys([]byte(v.ValueString))
		if err != nil {
			return
		}
		addr.Mainnet = globals.IsMainnet()
		result.Address = addr.String()
		result.Name = p.Name
		result.Status = "OK"
	}
	return

}
