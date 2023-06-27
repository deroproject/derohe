// Copyright 2017-2022 DERO Project. All rights reserved.
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

import (
	_ "embed"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/dvm"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/graviton"
)

//go:embed hardcoded_sc/nameservice.bas
var source_nameservice string

//go:embed hardcoded_sc/nameservice_updateable.bas
var source_nameservice_updateable string

// process the miner tx, giving fees, miner rewatd etc
func (chain *Blockchain) install_hardcoded_contracts(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, balance_tree *graviton.Tree, sc_tree *graviton.Tree, height uint64) (err error) {

	if height == 0 {
		if _, _, err = dvm.ParseSmartContract(source_nameservice); err != nil {
			logger.Error(err, "error Parsing hard coded sc")
			panic(err)
		}

		var name crypto.Hash
		name[31] = 1
		if err = chain.install_hardcoded_sc(cache, ss, balance_tree, sc_tree, source_nameservice, name); err != nil {
			panic(err)
		}
	}

	// it is updated at 0 height for testnets
	if height == uint64(globals.Config.HF1_HEIGHT) { // update SC at specific height
		if _, _, err = dvm.ParseSmartContract(source_nameservice_updateable); err != nil {
			logger.Error(err, "error Parsing hard coded sc")
			panic(err)
		}

		var name crypto.Hash
		name[31] = 1
		if err = chain.install_hardcoded_sc(cache, ss, balance_tree, sc_tree, source_nameservice_updateable, name); err != nil {
			panic(err)
		}
	}

	return
}

// hard coded contracts generally do not do any initialization
func (chain *Blockchain) install_hardcoded_sc(cache map[crypto.Hash]*graviton.Tree, ss *graviton.Snapshot, balance_tree *graviton.Tree, sc_tree *graviton.Tree, source string, scid crypto.Hash) (err error) {
	w_sc_tree := &dvm.Tree_Wrapper{Tree: sc_tree, Entries: map[string][]byte{}}
	var w_sc_data_tree *dvm.Tree_Wrapper

	meta := dvm.SC_META_DATA{}
	w_sc_data_tree = dvm.Wrapped_tree(cache, ss, scid)

	// install SC, should we check for sanity now, why or why not
	w_sc_data_tree.Put(dvm.SC_Code_Key(scid), dvm.Variable{Type: dvm.String, ValueString: source}.MarshalBinaryPanic())
	w_sc_tree.Put(dvm.SC_Meta_Key(scid), meta.MarshalBinary())

	// we must commit all the changes

	// anything below should never give error
	if _, ok := cache[scid]; !ok {
		cache[scid] = w_sc_data_tree.Tree
	}

	for k, v := range w_sc_data_tree.Entries { // commit entire data to tree
		if err = w_sc_data_tree.Tree.Put([]byte(k), v); err != nil {
			return
		}
	}

	for k, v := range w_sc_tree.Entries {
		if err = w_sc_tree.Tree.Put([]byte(k), v); err != nil {
			return
		}
	}

	return nil
}
