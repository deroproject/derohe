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
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/graviton"
)

// GetValuesFromKeys returns all values of requested keys in same order
func GetValuesFromKeysSC(ctx context.Context, p rpc.GetValuesFromKeysSC_Params) (result rpc.GetValuesFromKeysSC_Result, err error) {
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

	// Initialize the result
	result.Values = make([]interface{}, len(p.Keys))

	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	if err == nil {
		var ss *graviton.Snapshot
		ss, err = chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
		if err == nil {
			var sc_data_tree *graviton.Tree
			sc_data_tree, err = ss.GetTree(string(scid[:]))
			if err == nil {
				var variable dvm.Variable
				var bytes []byte
				for i := range p.Keys {
					k := p.Keys[i]
					if val, ok := k.Key.(string); ok {
						// Special flag in case someone store in following format:
						// STORE(SIGNER(), "value")
						if k.IsAddress {
							var addr rpc.Address
							err = addr.UnmarshalText([]byte(val))
							if err != nil {
								return result, fmt.Errorf("key '%s' string is not an address: %s", val, err)
							}
							val = string(addr.Compressed())
						}
						variable = dvm.Variable{
							Type:        dvm.String,
							ValueString: val,
						}
					} else if val, ok := k.Key.(uint64); ok {
						if k.IsAddress {
							return result, fmt.Errorf("key Uint64 cannot be address type: %s", k.Key)
						}

						variable = dvm.Variable{
							Type:        dvm.Uint64,
							ValueUint64: val,
						}
					} else {
						return result, fmt.Errorf("invalid key type '%T' for key '%s'", k.Key, k.Key)
					}

					bytes, err = variable.MarshalBinary()
					if err != nil {
						return result, fmt.Errorf("failed to marshal key '%s': %s", k.Key, err)
					}

					bytes, err = sc_data_tree.Get(bytes)
					err = variable.UnmarshalBinary(bytes)
					if err != nil {
						return result, fmt.Errorf("failed to unmarshal value data with key '%s': %s", k.Key, err)
					}

					switch variable.Type {
					case dvm.String:
						variable.ValueString, err = MapData(&k.ValueMapper, variable.ValueString)
						if err != nil {
							return result, fmt.Errorf("failed to map value data with key '%s': %s", k.Key, err)
						}
						result.Values[i] = variable.ValueString
					case dvm.Uint64:
						result.Values[i] = variable.ValueUint64
					}
				}
			}
		}
	}

	result.Status = "OK"
	err = nil

	return
}
