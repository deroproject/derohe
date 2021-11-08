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

package blockchain

// this file installs hard coded contracts

//import "fmt"
import _ "embed"

/*
import "strings"
import "strconv"
import "encoding/hex"
import "encoding/binary"
import "math/big"
import "golang.org/x/xerrors"


import "github.com/deroproject/derohe/cryptography/bn256"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/premine"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/rpc"



*/

import "github.com/deroproject/graviton"
import "github.com/deroproject/derohe/dvm"
import "github.com/deroproject/derohe/cryptography/crypto"

//go:embed hardcoded_sc/nameservice.bas
var source_nameservice string

// process the miner tx, giving fees, miner rewatd etc
func (chain *Blockchain) install_hardcoded_contracts(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, balance_tree *graviton.Tree, sc_tree *graviton.Tree, height uint64) (err error) {

	if height != 0 {
		return
	}

	if _, _, err = dvm.ParseSmartContract(source_nameservice); err != nil {
		logger.Error(err, "error Parsing hard coded sc")
		return
	}

	var name crypto.Hash
	name[31] = 1
	if err = chain.install_hardcoded_sc(cache, ss, balance_tree, sc_tree, source_nameservice, name); err != nil {
		return
	}

	//fmt.Printf("source code embedded %s\n",source_nameservice)

	return
}

// hard coded contracts generally do not do any initialization
func (chain *Blockchain) install_hardcoded_sc(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, balance_tree *graviton.Tree, sc_tree *graviton.Tree, source string, scid crypto.Hash) (err error) {
	w_sc_tree := &Tree_Wrapper{tree: sc_tree, entries: map[string][]byte{}}
	var w_sc_data_tree *Tree_Wrapper

	meta := SC_META_DATA{}
	w_sc_data_tree = wrapped_tree(cache, ss, scid)

	// install SC, should we check for sanity now, why or why not
	w_sc_data_tree.Put(SC_Code_Key(scid), dvm.Variable{Type: dvm.String, ValueString: source}.MarshalBinaryPanic())
	w_sc_tree.Put(SC_Meta_Key(scid), meta.MarshalBinary())

	// we must commit all the changes

	// anything below should never give error
	if _, ok := cache[scid]; !ok {
		cache[scid] = w_sc_data_tree.tree
	}

	for k, v := range w_sc_data_tree.entries { // commit entire data to tree
		if err = w_sc_data_tree.tree.Put([]byte(k), v); err != nil {
			return
		}
	}

	for k, v := range w_sc_tree.entries {
		if err = w_sc_tree.tree.Put([]byte(k), v); err != nil {
			return
		}
	}

	return nil
}
