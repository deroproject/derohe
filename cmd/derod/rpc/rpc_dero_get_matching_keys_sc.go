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
	"regexp"
	"runtime/debug"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/dvm"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/graviton"
)

// GetMatchingKeysSC returns all keys matching the given regex pattern
func GetMatchingKeysSC(ctx context.Context, p rpc.GetMatchingKeysSC_Params) (result rpc.GetMatchingKeysSC_Result, err error) {
	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace r %s %s", r, debug.Stack())
		}
	}()

	scid := crypto.HashHexToHash(p.SCID)
	topoheight := chain.Load_TOPO_HEIGHT()

	if p.TopoHeight >= 1 {
		topoheight = p.TopoHeight
	}

	// Parse the regex keys only one time
	regex_keys := make([]*regexp.Regexp, len(p.Patterns))
	for i := range p.Patterns {
		regex_keys[i], err = regexp.Compile(p.Patterns[i])
		if err != nil {
			return
		}
	}

	// Initialize the result
	result.Keys = make([][]string, len(p.Patterns))
	for i := range result.Keys {
		result.Keys[i] = make([]string, 0)
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
				var k []byte
				for k, _, err = cursor.First(); err == nil; k, _, err = cursor.Next() {
					var key dvm.Variable
					// 0x3 is beginning of valid DVM types, we handle only DVM String keys here
					if k[len(k)-1] >= 0x3 && k[len(k)-1] < 0x80 && nil == key.UnmarshalBinary(k) && key.Type == dvm.String && key.ValueString != "" {
						for i := range regex_keys {
							if regex_keys[i].MatchString(key.ValueString) {
								result.Keys[i] = append(result.Keys[i], key.ValueString)
							}
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
