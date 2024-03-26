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

type filter struct {
	key   *regexp.Regexp
	value *regexp.Regexp
	or    bool
	// all mappers are applied before the regex match
	before_map_key   *rpc.ValueMapperSC
	before_map_value *rpc.ValueMapperSC
	// all mappers are applied after the regex match
	after_map_key   *rpc.ValueMapperSC
	after_map_value *rpc.ValueMapperSC
}

func SearchVariablesSC(ctx context.Context, p rpc.SearchVariablesSC_Params) (result rpc.SearchVariablesSC_Result, err error) {
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
	parsed_filters := make([]filter, len(p.Filters))
	{
		var key, value *regexp.Regexp
		var before_map_key, before_map_value, after_map_key, after_map_value *rpc.ValueMapperSC
		for i := range p.Filters {
			key = nil
			value = nil

			f := p.Filters[i]
			if f.KeyPattern != "" {
				key, err = regexp.Compile(f.KeyPattern)
				if err != nil {
					return
				}
			}
			if f.ValuePattern != "" {
				value, err = regexp.Compile(f.ValuePattern)
				if err != nil {
					return
				}
			}

			// Check if the filter is valid
			if key == nil && value == nil || (key == nil || value == nil) && f.IsOr {
				return result, fmt.Errorf("invalid filter: %d", i)
			}

			// Either before or after, not both
			if (f.BeforeKeyMapper != "" && f.AfterKeyMapper != "") || (f.BeforeValueMapper != "" && f.AfterValueMapper != "") {
				return result, fmt.Errorf("invalid mapper in filter: %d", i)
			}

			if f.BeforeKeyMapper != "" {
				before_map_key = &f.BeforeKeyMapper
			}
			if f.BeforeValueMapper != "" {
				before_map_value = &f.BeforeValueMapper
			}
			if f.AfterKeyMapper != "" {
				after_map_key = &f.AfterKeyMapper
			}
			if f.AfterValueMapper != "" {
				after_map_value = &f.AfterValueMapper
			}

			parsed_filters[i] = filter{
				key,
				value,
				f.IsOr,
				before_map_key,
				before_map_value,
				after_map_key,
				after_map_value,
			}
		}
	}

	// Initialize the result
	result.Data = make([]rpc.FilterSC_Result, len(p.Filters))
	for i := range result.Data {
		result.Data[i] = rpc.FilterSC_Result{
			Keys:   make([]interface{}, 0),
			Values: make([]interface{}, 0),
		}
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
				var key, value dvm.Variable
				for k, v, err = cursor.First(); err == nil; k, v, err = cursor.Next() {
					if k[len(k)-1] >= 0x3 && k[len(k)-1] < 0x80 && nil == key.UnmarshalBinary(k) && nil == value.UnmarshalBinary(v) {
						// prevent that a filter is applied multiple times
						current_key := key
						current_value := value

						for i := range parsed_filters {
							// set original data
							key = current_key
							value = current_value

							filter := parsed_filters[i]

							skip := false
							if filter.key != nil {
								switch key.Type {
								case dvm.String:
									key.ValueString, err = MapData(filter.before_map_key, key.ValueString)
									if err != nil {
										continue
									}

									if !filter.key.MatchString(key.ValueString) {
										skip = true
									}
								case dvm.Uint64:
									if !filter.key.MatchString(fmt.Sprintf("%d", key.ValueUint64)) {
										skip = true
									}
								}
							}

							if skip && !filter.or {
								continue
							}

							if filter.value != nil {
								switch value.Type {
								case dvm.String:
									value.ValueString, err = MapData(filter.before_map_value, value.ValueString)
									if err != nil {
										continue
									}

									if !filter.value.MatchString(value.ValueString) {
										if skip || !filter.or {
											continue
										}
									}
								case dvm.Uint64:
									if !filter.value.MatchString(fmt.Sprintf("%d", value.ValueUint64)) {
										if skip || !filter.or {
											continue
										}
									}
								}
							}

							// if we are here, the filter matches, add the data
							switch key.Type {
							case dvm.String:
								key.ValueString, err = MapData(filter.after_map_key, key.ValueString)
								if err != nil {
									return result, fmt.Errorf("error after mapping key: %s", err)
								}

								result.Data[i].Keys = append(result.Data[i].Keys, key.ValueString)
							case dvm.Uint64:
								result.Data[i].Keys = append(result.Data[i].Keys, key.ValueUint64)
							}

							switch value.Type {
							case dvm.String:
								value.ValueString, err = MapData(filter.after_map_value, value.ValueString)
								if err != nil {
									return result, fmt.Errorf("error after mapping value: %s", err)
								}
								result.Data[i].Values = append(result.Data[i].Values, value.ValueString)
							case dvm.Uint64:
								result.Data[i].Values = append(result.Data[i].Values, value.ValueUint64)
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

// MapData maps the data according to the given mapper
func MapData(mapper *rpc.ValueMapperSC, data string) (result string, err error) {
	if mapper == nil {
		return data, nil
	}

	switch *mapper {
	case rpc.ValueMapperSC_Address:
		var addr *rpc.Address
		addr, err = rpc.NewAddressFromCompressedKeys([]byte(data))
		if err != nil {
			return
		}
		result = addr.String()
	case rpc.ValueMapperSC_ToHex:
		result = fmt.Sprintf("%x", data)
	default:
		return data, nil
	}
	return
}
