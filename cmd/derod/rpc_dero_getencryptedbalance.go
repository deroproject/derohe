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
import "math"
import "context"
import "encoding/hex"
import "runtime/debug"

//import	"log"
//import 	"net/http"
import "golang.org/x/xerrors"

import "github.com/deroproject/graviton"

import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/blockchain"
import "github.com/deroproject/derohe/address"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/structures"

func (DERO_RPC_APIS) GetEncryptedBalance(ctx context.Context, p structures.GetEncryptedBalance_Params) (result structures.GetEncryptedBalance_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace %s", debug.Stack())
		}
	}()

	uaddress, err := globals.ParseValidateAddress(p.Address)
	if err != nil {
		panic(err)
	}

	registration := LocatePointOfRegistration(uaddress)

	topoheight := chain.Load_TOPO_HEIGHT()

	if p.Merkle_Balance_TreeHash == "" && p.TopoHeight >= 0 && p.TopoHeight <= topoheight { // get balance tree at specific topoheight
		topoheight = p.TopoHeight
	}

	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	if err != nil {
		panic(err)
	}

	ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	if err != nil {
		panic(err)
	}

	var balance_tree *graviton.Tree

	if p.Merkle_Balance_TreeHash != "" { // user requested a specific tree hash version

		hash, err := hex.DecodeString(p.Merkle_Balance_TreeHash)
		if err != nil {
			panic(err)
		}

		if len(hash) != 32 {
			panic("corruted hash")
		}

		balance_tree, err = ss.GetTreeWithRootHash(hash)

	} else {

		balance_tree, err = ss.GetTree(blockchain.BALANCE_TREE)

	}
	if err != nil {
		panic(err)
	}

	balance_serialized, err := balance_tree.Get(uaddress.Compressed())

	if err != nil {
		if xerrors.Is(err, graviton.ErrNotFound) { // address needs registration
			return structures.GetEncryptedBalance_Result{ // return success
				Registration: registration,
				Status:       errormsg.ErrAccountUnregistered.Error(),
			}, errormsg.ErrAccountUnregistered

		} else {
			panic(err)
		}
	}
	merkle_hash, err := balance_tree.Hash()
	if err != nil {
		panic(err)
	}

	// calculate top height merkle tree hash
	//var dmerkle_hash crypto.Hash

	toporecord, err = chain.Store.Topo_store.Read(chain.Load_TOPO_HEIGHT())
	if err != nil {
		panic(err)
	}

	ss, err = chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	if err != nil {
		panic(err)
	}

	balance_tree, err = ss.GetTree(blockchain.BALANCE_TREE)
	if err != nil {
		panic(err)
	}

	dmerkle_hash, err := balance_tree.Hash()
	if err != nil {
		panic(err)
	}

	return structures.GetEncryptedBalance_Result{ // return success
		Data:                     fmt.Sprintf("%x", balance_serialized),
		Registration:             registration,
		Height:                   toporecord.Height,
		Topoheight:               topoheight,
		BlockHash:                fmt.Sprintf("%x", toporecord.BLOCK_ID),
		Merkle_Balance_TreeHash:  fmt.Sprintf("%x", merkle_hash[:]),
		DHeight:                  chain.Get_Height(),
		DTopoheight:              chain.Load_TOPO_HEIGHT(),
		DMerkle_Balance_TreeHash: fmt.Sprintf("%x", dmerkle_hash[:]),
		Status:                   "OK",
	}, nil
}

// if address is unregistered, returns negative numbers
func LocatePointOfRegistration(uaddress *address.Address) int64 {

	addr := uaddress.Compressed()

	low := chain.LocatePruneTopo() // in case of purging DB, this should start from N

	topoheight := chain.Load_TOPO_HEIGHT()
	high := int64(topoheight)

	if !IsRegisteredAtTopoHeight(addr, topoheight) {
		return -1
	}

	if IsRegisteredAtTopoHeight(addr, low) {
		return low
	}

	lowest := int64(math.MaxInt64)
	for low <= high {
		median := (low + high) / 2
		if IsRegisteredAtTopoHeight(addr, median) {
			if lowest > median {
				lowest = median
			}
			high = median - 1
		} else {
			low = median + 1
		}
	}

	//fmt.Printf("found point %d\n", lowest)

	return lowest
}

func IsRegisteredAtTopoHeight(addr []byte, topoheight int64) bool {

	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	if err != nil {
		panic(err)
	}

	ss, err := chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
	if err != nil {
		panic(err)
	}

	var balance_tree *graviton.Tree
	balance_tree, err = ss.GetTree(blockchain.BALANCE_TREE)
	if err != nil {
		panic(err)
	}

	_, err = balance_tree.Get(addr)

	if err != nil {
		if xerrors.Is(err, graviton.ErrNotFound) { // address needs registration
			return false

		} else {
			panic(err)
		}
	}

	return true
}
