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

package main

import "fmt"
import "context"
import "runtime/debug"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/rpc"

//import "github.com/deroproject/derohe/blockchain"

func (DERO_RPC_APIS) GetRandomAddress(ctx context.Context, p rpc.GetRandomAddress_Params) (result rpc.GetRandomAddress_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()
	topoheight := chain.Load_TOPO_HEIGHT()

	if topoheight > 100 {
		topoheight -= 5
	}

	var cursor_list []string

	{

		toporecord, err := chain.Store.Topo_store.Read(topoheight)

		if err != nil {
			panic(err)
		}
		ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
		if err != nil {
			panic(err)
		}

		treename := config.BALANCE_TREE
		if !p.SCID.IsZero() {
			treename = string(p.SCID[:])
		}

		balance_tree, err := ss.GetTree(treename)
		if err != nil {
			panic(err)
		}

		account_map := map[string]bool{}

		for i := 0; i < 100; i++ {

			k, _, err := balance_tree.Random()
			if err != nil {
				continue
			}

			var acckey crypto.Point
			if err := acckey.DecodeCompressed(k[:]); err != nil {
				continue
			}

			addr := rpc.NewAddressFromKeys(&acckey)
			addr.Mainnet = true
			if globals.Config.Name != config.Mainnet.Name { // anything other than mainnet is testnet at this point in time
				addr.Mainnet = false
			}
			account_map[addr.String()] = true
			if len(account_map) > 140 {
				break
			}
		}

		for k := range account_map {
			cursor_list = append(cursor_list, k)
		}
	}

	/*
	   		c := balance_tree.Cursor()
	   		for k, v, err := c.First(); err == nil; k, v, err = c.Next() {
	               _ = v
	   			//fmt.Printf("key=%x, value=%x err %s\n", k, v, err)

	   			var acckey crypto.Point
	   			if err := acckey.DecodeCompressed(k[:]); err != nil {
	   				panic(err)
	   			}

	   			addr := address.NewAddressFromKeys(&acckey)
	   			if globals.Config.Name != config.Mainnet.Name { // anything other than mainnet is testnet at this point in time
	   				addr.Network = globals.Config.Public_Address_Prefix
	   			}
	   			cursor_list = append(cursor_list, addr.String())
	   			if len(cursor_list) >= 20 {
	   				break
	   			}
	   		}

	   	}
	*/

	result.Address = cursor_list
	result.Status = "OK"

	return result, nil
}
