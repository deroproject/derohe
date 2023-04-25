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

import "fmt"
import "context"

import "encoding/binary"
import "runtime/debug"

//import "github.com/romana/rlog"
import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/dvm"

//import "github.com/deroproject/derohe/transaction"

import "github.com/deroproject/graviton"

func GetSC(ctx context.Context, p rpc.GetSC_Params) (result rpc.GetSC_Result, err error) {

	defer func() { // safety so if anything wrong happens, we return error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured. stack trace r %s %s", r, debug.Stack())
		}
	}()

	result.VariableStringKeys = map[string]interface{}{}
	result.VariableUint64Keys = map[uint64]interface{}{}
	result.Balances = map[string]uint64{}

	scid := crypto.HashHexToHash(p.SCID)

	topoheight := chain.Load_TOPO_HEIGHT()

	if p.TopoHeight >= 1 {
		topoheight = p.TopoHeight
	}

	toporecord, err := chain.Store.Topo_store.Read(topoheight)
	// we must now fill in compressed ring members
	if err == nil {
		var ss *graviton.Snapshot
		ss, err = chain.Store.Balance_store.LoadSnapshot(toporecord.State_Version)
		if err == nil {
			/*
				var sc_meta_tree *graviton.Tree
				if sc_meta_tree, err = ss.GetTree(config.SC_META); err == nil {
					var meta_bytes []byte
					if meta_bytes, err = sc_meta_tree.Get(blockchain.SC_Meta_Key(scid)); err == nil {
						var meta blockchain.SC_META_DATA
						if err = meta.UnmarshalBinary(meta_bytes); err == nil {
							result.Balance = meta.Balance
						}
					}
				} else {
					return
				}
			*/

			var sc_data_tree *graviton.Tree
			sc_data_tree, err = ss.GetTree(string(scid[:]))
			if err == nil {
				var zerohash crypto.Hash
				if balance_bytes, err := sc_data_tree.Get(zerohash[:]); err == nil {
					if len(balance_bytes) == 8 {
						result.Balance = binary.BigEndian.Uint64(balance_bytes[:])
					}
				}
				if p.Code { // give SC code
					var code_bytes []byte
					var v dvm.Variable
					if code_bytes, err = sc_data_tree.Get(dvm.SC_Code_Key(scid)); err == nil {
						if err = v.UnmarshalBinary(code_bytes); err != nil {
							result.Code = "Unmarshal error"
						} else {
							result.Code = v.ValueString
						}
					}
				}
				if p.Variables { // user requested all variables
					cursor := sc_data_tree.Cursor()
					var k, v []byte
					for k, v, err = cursor.First(); err == nil; k, v, err = cursor.Next() {
						var vark, varv dvm.Variable

						_ = vark
						_ = varv
						_ = k
						_ = v

						//fmt.Printf("key '%x'  value '%x'\n", k, v)
						if len(k) == 32 && len(v) == 8 { // it's SC balance
							result.Balances[fmt.Sprintf("%x", k)] = binary.BigEndian.Uint64(v)
						} else if k[len(k)-1] >= 0x3 && k[len(k)-1] < 0x80 && nil == vark.UnmarshalBinary(k) && nil == varv.UnmarshalBinary(v) {
							switch vark.Type {
							case dvm.Uint64:
								if varv.Type == dvm.Uint64 {
									result.VariableUint64Keys[vark.ValueUint64] = varv.ValueUint64
								} else {
									result.VariableUint64Keys[vark.ValueUint64] = fmt.Sprintf("%x", []byte(varv.ValueString))
								}

							case dvm.String:
								if varv.Type == dvm.Uint64 {
									result.VariableStringKeys[vark.ValueString] = varv.ValueUint64
								} else {
									result.VariableStringKeys[vark.ValueString] = fmt.Sprintf("%x", []byte(varv.ValueString))
								}
							default:
								err = fmt.Errorf("UNKNOWN Data type")
								return
							}

						}
					}
				}

				// give any uint64 keys data if any
				for _, value := range p.KeysUint64 {
					var v dvm.Variable
					key, _ := dvm.Variable{Type: dvm.Uint64, ValueUint64: value}.MarshalBinary()

					var value_bytes []byte
					if value_bytes, err = sc_data_tree.Get(key); err != nil {
						result.ValuesUint64 = append(result.ValuesUint64, fmt.Sprintf("NOT AVAILABLE err: %s", err))
						continue
					}
					if err = v.UnmarshalBinary(value_bytes); err != nil {
						result.ValuesUint64 = append(result.ValuesUint64, "Unmarshal error")
						continue
					}
					switch v.Type {
					case dvm.Uint64:
						result.ValuesUint64 = append(result.ValuesUint64, fmt.Sprintf("%d", v.ValueUint64))
					case dvm.String:
						result.ValuesUint64 = append(result.ValuesUint64, fmt.Sprintf("%x", []byte(v.ValueString)))
					default:
						result.ValuesUint64 = append(result.ValuesUint64, "UNKNOWN Data type")
					}
				}
				for _, value := range p.KeysString {
					var v dvm.Variable
					key, _ := dvm.Variable{Type: dvm.String, ValueString: value}.MarshalBinary()

					var value_bytes []byte
					if value_bytes, err = sc_data_tree.Get(key); err != nil {
						//fmt.Printf("Getting key %x\n", key)
						result.ValuesString = append(result.ValuesString, fmt.Sprintf("NOT AVAILABLE err: %s", err))
						continue
					}
					if err = v.UnmarshalBinary(value_bytes); err != nil {
						result.ValuesString = append(result.ValuesString, "Unmarshal error")
						continue
					}
					switch v.Type {
					case dvm.Uint64:
						result.ValuesString = append(result.ValuesString, fmt.Sprintf("%d", v.ValueUint64))
					case dvm.String:
						result.ValuesString = append(result.ValuesString, fmt.Sprintf("%x", []byte(v.ValueString)))
					default:
						result.ValuesString = append(result.ValuesString, "UNKNOWN Data type")
					}
				}

				for _, value := range p.KeysBytes {
					var v dvm.Variable
					key, _ := dvm.Variable{Type: dvm.String, ValueString: string(value)}.MarshalBinary()

					var value_bytes []byte
					if value_bytes, err = sc_data_tree.Get(key); err != nil {
						result.ValuesBytes = append(result.ValuesBytes, "NOT AVAILABLE")
						continue
					}
					if err = v.UnmarshalBinary(value_bytes); err != nil {
						result.ValuesBytes = append(result.ValuesBytes, "Unmarshal error")
						continue
					}
					switch v.Type {
					case dvm.Uint64:
						result.ValuesBytes = append(result.ValuesBytes, fmt.Sprintf("%d", v.ValueUint64))
					case dvm.String:
						result.ValuesBytes = append(result.ValuesBytes, fmt.Sprintf("%s", v.ValueString))
					default:
						result.ValuesBytes = append(result.ValuesBytes, "UNKNOWN Data type")
					}
				}

			}

		}

	}

	result.Status = "OK"
	err = nil

	//logger.Debugf("result %+v\n", result);
	return
}
