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

type DVM_FUNCTION_PTR func(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{})

var func_table = map[string][]func_data{}

type func_data struct {
	Range semver.Range
	Cost  int64
	Ptr   DVM_FUNCTION_PTR
}

func init() {
	func_table["version"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_version}}
	func_table["load"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_load}}
	func_table["exists"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_exists}}
	func_table["store"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_store}}
	func_table["delete"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_delete}}
	func_table["random"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_random}}
	func_table["scid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_scid}}
	func_table["blid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_blid}}
	func_table["txid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_txid}}
	func_table["dero"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_dero}}
	func_table["block_height"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_block_height}}
	func_table["block_timestamp"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_block_timestamp}}
	func_table["signer"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_signer}}
	func_table["update_sc_code"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_update_sc_code}}
	func_table["is_address_valid"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_is_address_valid}}
	func_table["address_raw"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_address_raw}}
	func_table["address_string"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_address_string}}
	func_table["send_dero_to_address"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_send_dero_to_address}}
	func_table["send_asset_to_address"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_send_asset_to_address}}
	func_table["derovalue"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_derovalue}}
	func_table["assetvalue"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_assetvalue}}
	func_table["atoi"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_atoi}}
	func_table["itoa"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_itoa}}
	func_table["sha256"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_sha256}}
	func_table["sha3256"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_sha3256}}
	func_table["keccak256"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_keccak256}}
	func_table["hex"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_hex}}
	func_table["hexdecode"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_hexdecode}}
	func_table["min"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_min}}
	func_table["max"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_max}}
	func_table["strlen"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_strlen}}
	func_table["substr"] = []func_data{func_data{Range: semver.MustParseRange(">=0.0.0"), Cost: 1, Ptr: dvm_substr}}
}

func (dvm *DVM_Interpreter) process_cost(c int64) {
	dvm.Cost += c

	// TODO check cost overflow
}

// this will handle all internal functions which may be required/necessary to expand DVM functionality
func (dvm *DVM_Interpreter) Handle_Internal_Function(expr *ast.CallExpr, func_name string) (handled bool, result interface{}) {
	var err error
	_ = err

	if funcs, ok := func_table[strings.ToLower(func_name)]; ok {
		for _, f := range funcs {
			if f.Range(dvm.Version) {
				dvm.process_cost(f.Cost)
				return f.Ptr(dvm, expr)
			}
		}

		return false, nil
	}

	return false, nil
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

func dvm_version(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("version expects 1 parameters")
	}
	if version_str, ok := dvm.eval(expr.Args[0]).(string); !ok {
		panic("unsupported version format")
	} else {
		dvm.Version = semver.MustParse(version_str)
	}
	return true, uint64(1)
}

func dvm_load(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 {
		panic("Load function expects a single varible as parameter")
	}
	key := dvm.eval(expr.Args[0])
	switch k := key.(type) {
	case uint64:
		return true, dvm.Load(Variable{Type: Uint64, ValueUint64: k})
	case string:
		return true, dvm.Load(Variable{Type: String, ValueString: k})
	default:
		panic("This variable cannot be loaded")
	}
}

func dvm_exists(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 {
		panic("Exists function expects a single varible as parameter")
	}
	key := dvm.eval(expr.Args[0]) // evaluate the argument and use the result
	switch k := key.(type) {
	case uint64:
		return true, dvm.Exists(Variable{Type: Uint64, ValueUint64: k})
	case string:
		return true, dvm.Exists(Variable{Type: String, ValueString: k})
	default:
		panic("This variable cannot be loaded")
	}
}

func dvm_store(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 2 {
		panic("Store function expects 2 variables as parameter")
	}
	key_eval := dvm.eval(expr.Args[0])
	value_eval := dvm.eval(expr.Args[1])
	var key, value Variable
	switch k := key_eval.(type) {
	case uint64:
		key = Variable{Type: Uint64, ValueUint64: k}

	case string:
		key = Variable{Type: String, ValueString: k}
	default:
		panic("This variable cannot be stored")
	}

	switch k := value_eval.(type) {
	case uint64:
		value = Variable{Type: Uint64, ValueUint64: k}
	case string:
		value = Variable{Type: String, ValueString: k}
	default:
		panic("This variable cannot be stored")
	}

	dvm.Store(key, value)
	return true, nil
}

func dvm_delete(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 {
		panic("Delete function expects 2 variables as parameter")
	}
	key_eval := dvm.eval(expr.Args[0])
	var key Variable
	switch k := key_eval.(type) {
	case uint64:
		key = Variable{Type: Uint64, ValueUint64: k}

	case string:
		key = Variable{Type: String, ValueString: k}
	default:
		panic("This variable cannot be deleted")
	}

	dvm.Delete(key)
	return true, nil
}

func dvm_random(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
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

func dvm_scid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 {
		panic("SCID function expects 0 parameters")
	}
	return true, string(dvm.State.Chain_inputs.SCID[:])
}
func dvm_blid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 {
		panic("blid function expects 0 parameters")
	}
	return true, string(dvm.State.Chain_inputs.BLID[:])
}
func dvm_txid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 {
		panic("txid function expects 0 parameters")
	}
	return true, string(dvm.State.Chain_inputs.TXID[:])
}

func dvm_dero(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 {
		panic("dero function expects 0 parameters")
	}
	var zerohash crypto.Hash
	return true, string(zerohash[:])
}
func dvm_block_height(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 {
		panic("BLOCK_HEIGHT function expects 0 parameters")
	}
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
func dvm_block_timestamp(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 {
		panic("BLOCK_TIMESTAMP function expects 0 parameters")
	}
	return true, dvm.State.Chain_inputs.BL_TIMESTAMP
}

func dvm_signer(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 {
		panic("SIGNER function expects 0 parameters")
	}
	return true, dvm.State.Chain_inputs.Signer
}

func dvm_update_sc_code(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 {
		panic("UPDATE_SC_CODE function expects 1 parameters")
	}
	code_eval := dvm.eval(expr.Args[0])
	switch k := code_eval.(type) {
	case string:
		dvm.State.Store.Store(DataKey{Key: Variable{Type: String, ValueString: "C"}}, Variable{Type: String, ValueString: k}) // TODO verify code authenticity how
		return true, uint64(1)
	default:
		return true, uint64(0)
	}
}

func dvm_is_address_valid(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 {
		panic("IS_ADDRESS_VALID function expects 1 parameters")
	}

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

func dvm_address_raw(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 {
		panic("ADDRESS_RAW function expects 1 parameters")
	}

	addr_eval := dvm.eval(expr.Args[0])
	switch k := addr_eval.(type) {
	case string:
		if addr, err := rpc.NewAddress(k); err == nil {
			return true, string(addr.Compressed())
		}

		return true, nil // fallthrough not supported in type switch
	default:
		return true, nil
	}
}

func dvm_address_string(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 {
		panic("ADDRESS_STRING function expects 1 parameters")
	}

	addr_eval := dvm.eval(expr.Args[0])
	switch k := addr_eval.(type) {
	case string:
		p := new(crypto.Point)
		if err := p.DecodeCompressed([]byte(k)); err == nil {

			addr := rpc.NewAddressFromKeys(p)
			return true, addr.String()
		}

		return true, nil // fallthrough not supported in type switch
	default:
		return true, nil
	}
}

func dvm_send_dero_to_address(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 2 {
		panic("SEND_DERO_TO_ADDRESS function expects 2 parameters")
	}

	addr_eval := dvm.eval(expr.Args[0])
	amount_eval := dvm.eval(expr.Args[1])

	if err := new(crypto.Point).DecodeCompressed([]byte(addr_eval.(string))); err != nil {
		panic("address must be valid DERO network address")
	}

	if _, ok := amount_eval.(uint64); !ok {
		panic("amount must be valid  uint64")
	}

	if amount_eval.(uint64) == 0 {
		return true, amount_eval
	}
	var zerohash crypto.Hash
	dvm.State.Store.SendExternal(dvm.State.Chain_inputs.SCID, zerohash, addr_eval.(string), amount_eval.(uint64)) // add record for external transfer
	return true, amount_eval
}
func dvm_send_asset_to_address(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 3 {
		panic("SEND_ASSET_TO_ADDRESS function expects 3 parameters") // address, amount, asset
	}

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
		return true, amount_eval
	}

	if len(asset_eval.(string)) != 32 {
		panic("asset must be valid string of 32 byte length")
	}
	var asset crypto.Hash
	copy(asset[:], ([]byte(asset_eval.(string))))

	dvm.State.Store.SendExternal(dvm.State.Chain_inputs.SCID, asset, addr_eval.(string), amount_eval.(uint64)) // add record for external transfer

	return true, amount_eval
}

func dvm_derovalue(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 0 { // expression without limit
		panic("DEROVALUE expects no parameters")
	} else {
		return true, dvm.State.Assets[dvm.State.SCIDZERO]
	}
}

func dvm_assetvalue(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("assetVALUE expects 1 parameters")
	} else {
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
}

func dvm_itoa(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("itoa expects 1 parameters")
	} else {
		asset_eval := dvm.eval(expr.Args[0])

		if _, ok := asset_eval.(uint64); !ok {
			panic("itoa argument must be valid uint64")
		}

		return true, fmt.Sprintf("%d", asset_eval.(uint64))
	}
}

func dvm_atoi(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("itoa expects 1 parameters")
	} else {
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
}

func dvm_strlen(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("itoa expects 1 parameters")
	} else {
		asset_eval := dvm.eval(expr.Args[0])

		if _, ok := asset_eval.(string); !ok {
			panic("atoi argument must be valid string")
		}
		return true, uint64(len([]byte(asset_eval.(string))))
	}
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

func dvm_substr(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 3 { // expression without limit
		panic("substr expects 3 parameters")
	}
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

func dvm_sha256(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("sha256 expects 1 parameters")
	}
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}

	hash := sha256.Sum256([]byte(input_eval.(string)))
	return true, string(hash[:])
}

func dvm_sha3256(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("sha3256 expects 1 parameters")
	}
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}

	hash := sha3.Sum256([]byte(input_eval.(string)))
	return true, string(hash[:])
}

func dvm_keccak256(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("keccak256 expects 1 parameters")
	}
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}

	h1 := sha3.NewLegacyKeccak256()
	h1.Write([]byte(input_eval.(string)))
	hash := h1.Sum(nil)
	return true, string(hash[:])
}

func dvm_hex(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("hex expects 1 parameters")
	}
	input_eval := dvm.eval(expr.Args[0])
	if _, ok := input_eval.(string); !ok {
		panic("input argument must be valid string")
	}
	return true, hex.EncodeToString([]byte(input_eval.(string)))
}
func dvm_hexdecode(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 1 { // expression without limit
		panic("hex expects 1 parameters")
	}
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

func dvm_min(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 2 { // expression without limit
		panic("min expects 2 parameters")
	}
	a1 := dvm.eval(expr.Args[0])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	a2 := dvm.eval(expr.Args[1])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	if a1.(uint64) < a2.(uint64) {
		return true, a1
	}
	return true, a2
}

func dvm_max(dvm *DVM_Interpreter, expr *ast.CallExpr) (handled bool, result interface{}) {
	if len(expr.Args) != 2 { // expression without limit
		panic("min expects 2 parameters")
	}
	a1 := dvm.eval(expr.Args[0])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	a2 := dvm.eval(expr.Args[1])
	if _, ok := a1.(uint64); !ok {
		panic("input argument must be uint64")
	}

	if a1.(uint64) > a2.(uint64) {
		return true, a1
	}
	return true, a2
}
