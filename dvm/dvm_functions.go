// Copyright 2017-2018 DERO Project. All rights reserved.
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

package dvm

import "fmt"
import "go/ast"
import "strconv"
import "strings"
import "crypto/sha256"
import "encoding/hex"
import "golang.org/x/crypto/sha3"
import "github.com/blang/semver/v4"

import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/cryptography/crypto"

// this files defines  external functions which can be called in DVM
// for example to load and store data from the blockchain and other basic functions

// random number generator is the basis
// however, further investigation is needed if we would like to enable users to use pederson commitments
// they can be used like
// original SC developers delivers a pederson commitment to SC as external oracle
// after x users have played lottery, dev reveals the commitment using which the winner is finalised
// this needs more investigation
// also, more investigation is required to enable predetermined external oracles

type DVM_FUNCTION_PTR_UINT64 func(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64)
type DVM_FUNCTION_PTR_STRING func(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string)
type DVM_FUNCTION_PTR_ANY func(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{})

var func_table = map[string][]func_data{}

type func_data struct {
	Range       semver.Range
	ComputeCost int64
	StorageCost int64
	PtrU        DVM_FUNCTION_PTR_UINT64
	PtrS        DVM_FUNCTION_PTR_STRING
	Ptr         DVM_FUNCTION_PTR_ANY
}

func init() {
	func_table["version"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 1000, StorageCost: 0, PtrU: dvm_version}}
	func_table["load"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, Ptr: dvm_load}}
	func_table["exists"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, PtrU: dvm_exists}}
	func_table["store"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 10000, StorageCost: 0, PtrU: dvm_store}}
	func_table["delete"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 3000, StorageCost: 0, PtrU: dvm_delete}}
	func_table["mapexists"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 1000, StorageCost: 0, PtrU: dvm_mapexists}}
	func_table["mapget"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 1000, StorageCost: 0, Ptr: dvm_mapget}}
	func_table["mapstore"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 1000, StorageCost: 0, PtrU: dvm_mapstore}}
	func_table["mapdelete"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 1000, StorageCost: 0, PtrU: dvm_mapdelete}}
	func_table["random"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 2500, StorageCost: 0, PtrU: dvm_random}}
	func_table["scid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 2000, StorageCost: 0, PtrS: dvm_scid}}
	func_table["blid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 2000, StorageCost: 0, PtrS: dvm_blid}}
	func_table["txid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 2000, StorageCost: 0, PtrS: dvm_txid}}
	func_table["dero"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 10000, StorageCost: 0, PtrS: dvm_dero}}
	func_table["block_height"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 2000, StorageCost: 0, PtrU: dvm_block_height}}
	func_table["block_timestamp"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 2500, StorageCost: 0, PtrU: dvm_block_timestamp}}
	func_table["signer"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, PtrS: dvm_signer}}
	func_table["update_sc_code"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, PtrU: dvm_update_sc_code}}
	func_table["is_address_valid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 50000, StorageCost: 0, PtrU: dvm_is_address_valid}}
	func_table["address_raw"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 60000, StorageCost: 0, PtrS: dvm_address_raw}}
	func_table["address_string"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 50000, StorageCost: 0, PtrS: dvm_address_string}}
	func_table["send_dero_to_address"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 70000, StorageCost: 0, PtrU: dvm_send_dero_to_address}}
	func_table["send_asset_to_address"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 90000, StorageCost: 0, PtrU: dvm_send_asset_to_address}}
	func_table["derovalue"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 10000, StorageCost: 0, PtrU: dvm_derovalue}}
	func_table["assetvalue"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 10000, StorageCost: 0, PtrU: dvm_assetvalue}}
	func_table["atoi"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, PtrU: dvm_atoi}}
	func_table["itoa"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, PtrS: dvm_itoa}}
	func_table["sha256"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 25000, StorageCost: 0, PtrS: dvm_sha256}}
	func_table["sha3256"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 25000, StorageCost: 0, PtrS: dvm_sha3256}}
	func_table["keccak256"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 25000, StorageCost: 0, PtrS: dvm_keccak256}}
	func_table["hex"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 10000, StorageCost: 0, PtrS: dvm_hex}}
	func_table["hexdecode"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 10000, StorageCost: 0, PtrS: dvm_hexdecode}}
	func_table["min"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, PtrU: dvm_min}}
	func_table["max"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 5000, StorageCost: 0, PtrU: dvm_max}}
	func_table["strlen"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 20000, StorageCost: 0, PtrU: dvm_strlen}}
	func_table["substr"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 20000, StorageCost: 0, PtrS: dvm_substr}}
	func_table["panic"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), ComputeCost: 10000, StorageCost: 0, PtrU: dvm_panic}}
}

// this will handle all internal functions which may be required/necessary to expand DVM functionality
func (dvm *DVM_Interpreter) Handle_Internal_Function(expr *ast.CallExpr, func_name string) (handled bool, result interface{}) {

	if func_data_array, ok := func_table[strings.ToLower(func_name)]; ok {
		for _, f := range func_data_array {
			if f.Range(dvm.Version) {
				dvm.State.ConsumeGas(f.ComputeCost)
				if f.PtrU != nil {
					return f.PtrU(dvm, expr)
				} else if f.PtrS != nil {
					return f.PtrS(dvm, expr)
				} else {
					return f.Ptr(dvm, expr)
				}
			}
		}
		panic("function doesnot match any version")
	}
	//panic("function does not exist")
	return false, nil // function does not exist
}

// the load/store functions are sandboxed and thus cannot affect any other SC storage
// loads  a variable from store
func (dvm *DVM_Interpreter) Load(key Variable) interface{} {
	var found uint64
	result := dvm.State.Store.Load(DataKey{SCID: dvm.State.Chain_inputs.SCID, Key: key}, &found)

	switch result.Type {
	case Uint64:
		return result.ValueUint64
	case String:
		return result.ValueString

	default:
		panic("Unhandled data_type")
	}
}

// whether a variable exists in store or not
func (dvm *DVM_Interpreter) Exists(key Variable) uint64 {
	var found uint64
	dvm.State.Store.Load(DataKey{SCID: dvm.State.Chain_inputs.SCID, Key: key}, &found)
	return found
}

func (dvm *DVM_Interpreter) Store(key Variable, value Variable) {
	dvm.State.Store.Store(DataKey{SCID: dvm.State.Chain_inputs.SCID, Key: key}, value)
}

func (dvm *DVM_Interpreter) Delete(key Variable) {
	dvm.State.Store.Delete(DataKey{SCID: dvm.State.Chain_inputs.SCID, Key: key})
}

// we should migrate to generics ASAP
func convertdatatovariable(datai interface{}) Variable {
	switch k := datai.(type) {
	case uint64:
		return Variable{Type: Uint64, ValueUint64: k}
	case string:
		return Variable{Type: String, ValueString: k}
	default:
		panic("This variable cannot be loaded")
	}
}

// checks whether necessary number of arguments have been provided
func checkargscount(expected, actual int) {
	if expected != actual {
		panic("incorrect number of arguments")
	}
}

func dvm_version(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments

	if version_str, ok := dvm.eval(expr.Args[0]).(string); !ok {
		panic("unsupported version format")
	} else {
		dvm.Version = semver.MustParse(version_str)
	}
	return true, uint64(1)
}

func dvm_load(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	key := dvm.eval(expr.Args[0])
	return true, dvm.Load(convertdatatovariable(key))

}

func dvm_exists(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	key := dvm.eval(expr.Args[0])     // evaluate the argument and use the result
	return true, dvm.Exists(convertdatatovariable(key))
}

func dvm_store(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(2, len(expr.Args)) // check number of arguments
	key := convertdatatovariable(dvm.eval(expr.Args[0]))
	value := convertdatatovariable(dvm.eval(expr.Args[1]))

	dvm.Store(key, value)
	return true, 1
}

func dvm_delete(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	key := convertdatatovariable(dvm.eval(expr.Args[0]))
	dvm.Delete(key)
	return true, uint64(1)
}

func dvm_mapexists(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args))                    // check number of arguments
	key := convertdatatovariable(dvm.eval(expr.Args[0])) // evaluate the argument and use the result

	if _, ok := dvm.State.RamStore[key]; ok {
		return true, uint64(1)
	}
	return true, uint64(0)

}

func dvm_mapget(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	checkargscount(1, len(expr.Args))                    // check number of arguments
	key := convertdatatovariable(dvm.eval(expr.Args[0])) // evaluate the argument and use the result

	v := dvm.State.RamStore[key]

	if v.Type == Uint64 {
		return true, v.ValueUint64
	} else if v.Type == String {
		return true, v.ValueString
	} else {
		panic("This variable cannot be obtained")
	}
}

func dvm_mapstore(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(2, len(expr.Args))                      // check number of arguments
	key := convertdatatovariable(dvm.eval(expr.Args[0]))   // evaluate the argument and use the result
	value := convertdatatovariable(dvm.eval(expr.Args[1])) // evaluate the argument and use the result

	dvm.State.RamStore[key] = value
	return true, uint64(1)
}

func dvm_mapdelete(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args))                    // check number of arguments
	key := convertdatatovariable(dvm.eval(expr.Args[0])) // evaluate the argument and use the result

	delete(dvm.State.RamStore, key)
	return true, uint64(1)
}

func dvm_random(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	if len(expr.Args) >= 2 {
		panic("RANDOM function expects 0 or 1 number as parameter")
	}

	if len(expr.Args) == 0 { // expression without limit
		return true, dvm.State.RND.Random()
	}

	range_eval := dvm.eval(expr.Args[0])
	switch k := range_eval.(type) {
	case uint64:
		return true, dvm.State.RND.Random_MAX(k)
	default:
		panic("This variable cannot be randomly generated")
	}
}

func dvm_scid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	return true, string(dvm.State.Chain_inputs.SCID[:])
}
func dvm_blid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	return true, string(dvm.State.Chain_inputs.BLID[:])
}
func dvm_txid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	return true, string(dvm.State.Chain_inputs.TXID[:])
}

func dvm_dero(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	var zerohash crypto.Hash
	return true, string(zerohash[:])
}
func dvm_block_height(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	return true, dvm.State.Chain_inputs.BL_HEIGHT
}

/*
	func dvm_block_topoheight(dvm *DVM_Interpreter, expr *ast.CallExpr)(handled bool, result interface{}){
		if len(expr.Args) != 0 {
				panic("BLOCK_HEIGHT function expects 0 parameters")
			}
			return true, dvm.State.Chain_inputs.BL_TOPOHEIGHT
	}
*/
func dvm_block_timestamp(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	return true, dvm.State.Chain_inputs.BL_TIMESTAMP
}

func dvm_signer(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	return true, dvm.State.Chain_inputs.Signer
}

func dvm_update_sc_code(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	code_eval := dvm.eval(expr.Args[0])
	switch k := code_eval.(type) {
	case string:
		dvm.State.Store.Store(DataKey{Key: Variable{Type: String, ValueString: "C"}}, Variable{Type: String, ValueString: k}) // TODO verify code authenticity how
		return true, uint64(1)
	default:
		return true, uint64(0)
	}
}

func dvm_is_address_valid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments

	addr_eval := dvm.eval(expr.Args[0])
	switch k := addr_eval.(type) {
	case string:

		addr_raw := new(crypto.Point)
		if err := addr_raw.DecodeCompressed([]byte(k)); err == nil {
			return true, uint64(1)
		}
		return true, uint64(0) // fallthrough not supported in type switch

	default:
		return true, uint64(0)
	}
}

func dvm_address_raw(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments

	addr_eval := dvm.eval(expr.Args[0])
	switch k := addr_eval.(type) {
	case string:
		if addr, err := rpc.NewAddress(k); err == nil {
			return true, string(addr.Compressed())
		}

		return true, "" // fallthrough not supported in type switch
	default:
		return true, ""
	}
}

func dvm_address_string(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments

	addr_eval := dvm.eval(expr.Args[0])
	switch k := addr_eval.(type) {
	case string:
		p := new(crypto.Point)
		if err := p.DecodeCompressed([]byte(k)); err == nil {

			addr := rpc.NewAddressFromKeys(p)
			return true, addr.String()
		}

		return true, "" // fallthrough not supported in type switch
	default:
		return true, ""
	}
}

func dvm_send_dero_to_address(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(2, len(expr.Args)) // check number of arguments

	addr_eval := dvm.eval(expr.Args[0])
	amount_eval := dvm.eval(expr.Args[1])

	if err := new(crypto.Point).DecodeCompressed([]byte(addr_eval.(string))); err != nil {
		panic("address must be valid DERO network address")
	}

	if _, ok := amount_eval.(uint64); !ok {
		panic("amount must be valid  uint64")
	}

	if amount_eval.(uint64) == 0 {
		return true, amount_eval.(uint64)
	}
	var zerohash crypto.Hash
	dvm.State.Store.SendExternal(dvm.State.Chain_inputs.SCID, zerohash, addr_eval.(string), amount_eval.(uint64)) // add record for external transfer
	return true, amount_eval.(uint64)
}
func dvm_send_asset_to_address(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(3, len(expr.Args)) // check number of arguments

	addr_eval := dvm.eval(expr.Args[0])
	amount_eval := dvm.eval(expr.Args[1])
	asset_eval := dvm.eval(expr.Args[2])

	if err := new(crypto.Point).DecodeCompressed([]byte(addr_eval.(string))); err != nil {
		panic("address must be valid DERO network address")
	}

	if _, ok := amount_eval.(uint64); !ok {
		panic("amount must be valid  uint64")
	}

	if _, ok := asset_eval.(string); !ok {
		panic("asset must be valid string")
	}

	//fmt.Printf("sending asset %x (%d) to address %x\n", asset_eval.(string), amount_eval.(uint64),[]byte(addr_eval.(string)))

	if amount_eval.(uint64) == 0 {
		return true, amount_eval.(uint64)
	}

	if len(asset_eval.(string)) != 32 {
		panic("asset must be valid string of 32 byte length")
	}
	var asset crypto.Hash
	copy(asset[:], ([]byte(asset_eval.(string))))

	dvm.State.Store.SendExternal(dvm.State.Chain_inputs.SCID, asset, addr_eval.(string), amount_eval.(uint64)) // add record for external transfer

	return true, amount_eval.(uint64)
}

func dvm_derovalue(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(0, len(expr.Args)) // check number of arguments
	return true, dvm.State.Assets[dvm.State.SCIDZERO]

}

func dvm_assetvalue(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments

	asset_eval := dvm.eval(expr.Args[0])

	if _, ok := asset_eval.(string); !ok {
		panic("asset must be valid string")
	}
	if len(asset_eval.(string)) != 32 {
		panic("asset must be valid string of 32 byte length")
	}
	var asset crypto.Hash
	copy(asset[:], ([]byte(asset_eval.(string))))

	return true, dvm.State.Assets[asset]

}

func dvm_itoa(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments

	asset_eval := dvm.eval(expr.Args[0])

	if _, ok := asset_eval.(uint64); !ok {
		panic("itoa argument must be valid uint64")
	}

	return true, fmt.Sprintf("%d", asset_eval.(uint64))

}

func dvm_atoi(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments

	asset_eval := dvm.eval(expr.Args[0])

	if _, ok := asset_eval.(string); !ok {
		panic("atoi argument must be valid string")
	}

	if u, err := strconv.ParseUint(asset_eval.(string), 10, 64); err != nil {
		panic(err)
	} else {
		return true, u
	}

}

func dvm_strlen(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	asset_eval := dvm.eval(expr.Args[0])

	if _, ok := asset_eval.(string); !ok {
		panic("atoi argument must be valid string")
	}
	return true, uint64(len([]byte(asset_eval.(string))))

}

func substr(input string, start uint64, length uint64) string {
	asbytes := []byte(input)

	if start >= uint64(len(asbytes)) {
		return ""
	}

	if start+length > uint64(len(asbytes)) {
		length = uint64(len(asbytes)) - start
	}

	return string(asbytes[start : start+length])
}

func dvm_substr(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(3, len(expr.Args)) // check number of arguments
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}
	offset_eval := dvm.eval(expr.Args[1])
	if _, ok := offset_eval.(uint64); !ok {
		panic("input argument must be valid uint64")
	}
	length_eval := dvm.eval(expr.Args[2])
	if _, ok := length_eval.(uint64); !ok {
		panic("input argument must be valid uint64")
	}

	return true, substr(input_eval.(string), offset_eval.(uint64), length_eval.(uint64))
}

func dvm_sha256(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}

	hash := sha256.Sum256([]byte(input_eval.(string)))
	return true, string(hash[:])
}

func dvm_sha3256(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}

	hash := sha3.Sum256([]byte(input_eval.(string)))
	return true, string(hash[:])
}

func dvm_keccak256(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}

	h1 := sha3.NewLegacyKeccak256()
	h1.Write([]byte(input_eval.(string)))
	hash := h1.Sum(nil)
	return true, string(hash[:])
}

func dvm_hex(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}
	return true, hex.EncodeToString([]byte(input_eval.(string)))
}
func dvm_hexdecode(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result string) {
	checkargscount(1, len(expr.Args)) // check number of arguments
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}

	if b, err := hex.DecodeString(input_eval.(string)); err != nil {
		panic(err)
	} else {
		return true, string(b)
	}
}

func dvm_min(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(2, len(expr.Args)) // check number of arguments

	a1 := dvm.eval(expr.Args[0])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	a2 := dvm.eval(expr.Args[1])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	if a1.(uint64) < a2.(uint64) {
		return true, a1.(uint64)
	}
	return true, a2.(uint64)
}

func dvm_max(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	checkargscount(2, len(expr.Args)) // check number of arguments
	a1 := dvm.eval(expr.Args[0])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	a2 := dvm.eval(expr.Args[1])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	if a1.(uint64) > a2.(uint64) {
		return true, a1.(uint64)
	}
	return true, a2.(uint64)
}

func dvm_panic(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result uint64) {
	panic("panic function called")
	return true, uint64(0)
}
