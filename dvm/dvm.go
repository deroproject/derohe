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
import "text/scanner"
import "strings"
import "strconv"
import "unicode"
import "unicode/utf8"
import "go/ast"
import "go/parser"
import "go/token"
import "math"

import "runtime/debug"
import "github.com/blang/semver/v4"
import "github.com/deroproject/derohe/cryptography/crypto"

//import "github.com/deroproject/derohe/rpc"

type Vtype int

// the numbers start from 3 to avoid collisions and can go max upto 0x7f before collision occur
const (
	None    Vtype = 0x0
	Invalid Vtype = 0x3 // default is  invalid
	Uint64  Vtype = 0x4 // uint64 data type
	String  Vtype = 0x5 // string
)

var replacer = strings.NewReplacer("< =", "<=", "> =", ">=", "= =", "==", "! =", "!=", "& &", "&&", "| |", "||", "< <", "<<", "> >", ">>", "< >", "!=")

// Some global variables are always accessible, namely
// SCID  TXID which installed the SC
// TXID  current TXID under which this SC is currently executing
// BLID  current BLID under which TXID is found, THIS CAN be used as deterministic RND Generator, if the SC needs secure randomness
// BL_HEIGHT current height of blockchain

type Variable struct {
	Name        string `cbor:"N,omitempty" json:"N,omitempty"`
	Type        Vtype  `cbor:"T,omitempty" json:"T,omitempty"` // we have only 2 data types
	ValueUint64 uint64 `cbor:"V,omitempty" json:"VI,omitempty"`
	ValueString string `cbor:"V,omitempty" json:"VS,omitempty"`
}

type Function struct {
	Name        string              `cbor:"N,omitempty" json:"N,omitempty"`
	Params      []Variable          `cbor:"P,omitempty" json:"P,omitempty"`
	ReturnValue Variable            `cbor:"R,omitempty" json:"R,omitempty"`
	Lines       map[uint64][]string `cbor:"L,omitempty" json:"L,omitempty"`
	// map from line number to array index below
	LinesNumberIndex map[uint64]uint64 `cbor:"LI,omitempty" json:"LI,omitempty"` // a map is used to avoid sorting/searching
	LineNumbers      []uint64          `cbor:"LN,omitempty" json:"LN,omitempty"`
}

const LIMIT_interpreted_lines = 2000 // testnet has hardcoded limit
const LIMIT_evals = 11000            // testnet has hardcoded limit eval limit

// each smart code is nothing but a collection of functions
type SmartContract struct {
	Functions map[string]Function `cbor:"F,omitempty" json:"F,omitempty"`
}

// we have a rudimentary line by line parser
// SC authors must make sure code coverage is 100 %
// we are doing away with AST
func ParseSmartContract(src_code string) (SC SmartContract, pos string, err error) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Recovered in function %+v", r)
		}
	}()

	var s scanner.Scanner
	s.Init(strings.NewReader(src_code))
	s.Filename = "code"
	s.Mode = scanner.ScanIdents | scanner.ScanFloats | scanner.ScanChars | scanner.ScanStrings | scanner.ScanRawStrings | scanner.SkipComments | scanner.ScanComments //  skip comments

	skip_line := int32(-1)
	var current_line int32 = -1
	var line_tokens []string
	var current_function *Function

	SC.Functions = map[string]Function{}

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		pos := s.Position.String()
		txt := s.TokenText()

		if strings.HasPrefix(txt, ";") || strings.HasPrefix(txt, "REM") { // skip  line, if this is the first word
			skip_line = int32(s.Position.Line)
		}
		if skip_line == int32(s.Position.Line) {
			continue
		}

		/*if strings.HasPrefix(txt, "//") || strings.HasPrefix(txt, "/*") { // skip comments
			continue
		}*/

	process_token:
		if current_line == -1 {
			current_line = int32(s.Position.Line)
		}

		if current_line == int32(s.Position.Line) { // collect a complete line
			line_tokens = append(line_tokens, txt)
		} else { // if new line found, process previous line
			if err = parse_function_line(&SC, &current_function, line_tokens); err != nil {
				return SC, pos, err
			}
			line_tokens = line_tokens[:0]

			current_line = -1
			goto process_token
		}
		//  fmt.Printf("%s: %s line %+v\n", s.Position, txt, line_tokens)
	}

	if len(line_tokens) > 0 { // last line  is processed here
		if err = parse_function_line(&SC, &current_function, line_tokens); err != nil {
			return SC, pos, err
		}
	}

	if current_function != nil {
		err = fmt.Errorf("EOF reached but End Function is missing \"%s\"", current_function.Name)
		return SC, pos, err
	}

	return
}

// checks whether a function name is valid
// a valid name starts with a non digit and does not contain .
func check_valid_name(name string) bool {
	r, size := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError || size == 0 {
		return false
	}
	return unicode.IsLetter(r)
}

func check_valid_type(name string) Vtype {
	switch strings.ToLower(name) {
	case "uint64":
		return Uint64
	case "string":
		return String
	}
	return Invalid
}

// this will parse 1 line at a time, if there is an error, it is returned
func parse_function_line(SC *SmartContract, function **Function, line []string) (err error) {
	pos := 0
	//fmt.Printf("parsing function line %+v\n", line)

	if *function == nil { //if no current function, only legal is Sub
		if !strings.EqualFold(line[pos], "Function") {
			return fmt.Errorf("Expecting declaration of function  but found \"%s\"", line[0])
		}
		pos++

		var f Function
		f.Lines = map[uint64][]string{} // initialize line map
		f.LinesNumberIndex = map[uint64]uint64{}

		if len(line) < (pos + 1) {
			return fmt.Errorf("function name missing")
		}

		if !check_valid_name(line[pos]) {
			return fmt.Errorf("function name \"%s\" contains invalid characters", line[pos])
		}
		f.Name = line[pos]

		pos++
		if len(line) < (pos+1) || line[pos] != "(" {
			return fmt.Errorf("function \"%s\" missing '('", f.Name)
		}

	parse_params: // now lets parse function params, but lets filter out ,
		pos++
		if len(line) < (pos + 1) {
			return fmt.Errorf("function \"%s\" missing function parameters", f.Name)
		}

		if line[pos] == "," {
			goto parse_params
		}
		if line[pos] == ")" {
			// function does not have any parameters
			// or all parameters have been parsed

		} else { // we must parse param name, param type as  pairs
			if len(line) < (pos + 2) {
				return fmt.Errorf("function \"%s\" missing function parameters", f.Name)
			}

			param_name := line[pos]
			param_type := check_valid_type(line[pos+1])

			if !check_valid_name(param_name) {
				return fmt.Errorf("function name \"%s\", variable name \"%s\" contains invalid characters", f.Name, param_name)
			}

			if param_type == Invalid {
				return fmt.Errorf("function name \"%s\", variable type \"%s\" is invalid", f.Name, line[pos+1])
			}
			f.Params = append(f.Params, Variable{Name: param_name, Type: param_type})

			pos++
			goto parse_params
		}

		pos++

		// check if we have return value
		if len(line) < (pos + 1) { // we do not have return value
			f.ReturnValue.Type = Invalid
		} else {
			return_type := check_valid_type(line[pos])
			if return_type == Invalid {
				return fmt.Errorf("function name \"%s\", return type \"%s\" is invalid", f.Name, line[pos])
			}
			f.ReturnValue.Type = return_type
		}

		*function = &f
		return nil
	} else if strings.EqualFold(line[pos], "End") && strings.EqualFold(line[pos+1], "Function") {
		SC.Functions[(*function).Name] = **function
		*function = nil
	} else if strings.EqualFold(line[pos], "Function") {
		return fmt.Errorf("Nested functions are not allowed")
	} else { // add line to current function, provided line numbers are not duplicated, line numbers are mandatory
		line_number, err := strconv.ParseUint(line[pos], 10, 64)
		if err != nil {
			return fmt.Errorf("Error Parsing line number \"%s\" in function  \"%s\" ", line[pos], (*function).Name)
		}
		if line_number == 0 || line_number == math.MaxUint64 {
			return fmt.Errorf("Error: line number cannot be %d  in function  \"%s\" ", line_number, (*function).Name)
		}

		if _, ok := (*function).Lines[line_number]; ok { // duplicate line number
			return fmt.Errorf("Error: duplicate line number within function  \"%s\" ", (*function).Name)
		}
		if len((*function).LineNumbers) >= 1 && line_number < (*function).LineNumbers[len((*function).LineNumbers)-1] {
			return fmt.Errorf("Error: line number must be ascending within function  \"%s\" ", (*function).Name)
		}

		line_copy := make([]string, len(line)-1, len(line)-1)
		copy(line_copy, line[1:])
		(*function).Lines[line_number] = line_copy // we need to copy and add ( since line is reused)
		(*function).LinesNumberIndex[line_number] = uint64(len((*function).LineNumbers))
		(*function).LineNumbers = append((*function).LineNumbers, line_number)

		// fmt.Printf("%s %d %+v\n", (*function).Name, line_number, line[0:])
		// fmt.Printf("%s %d %+v\n", (*function).Name, line_number, (*function).Lines[line_number])

	}

	return nil
}

// this will run a function from a loaded SC and execute it if possible
// it can run all internal functions
// parameters must be passed as strings
func runSmartContract_internal(SC *SmartContract, EntryPoint string, state *Shared_State, params map[string]interface{}) (result Variable, err error) {
	// if smart contract does not contain function, trigger exception
	function_call, ok := SC.Functions[EntryPoint]
	if !ok {
		err = fmt.Errorf("function \"%s\" is not available in SC", EntryPoint)
		return
	}

	var dvm DVM_Interpreter
	dvm.SC = SC
	dvm.f = function_call
	dvm.Locals = map[string]Variable{}

	dvm.State = state // set state to execute current function

	// parse parameters, rename them, make them available as local variables
	for _, p := range function_call.Params {
		variable := Variable{Name: p.Name, Type: p.Type}
		value, ok := params[p.Name]
		if !ok { // necessary parameter is missing from arguments
			err = fmt.Errorf("Argument \"%s\" is missing while invoking \"%s\"", p.Name, EntryPoint)
			return
		}

		// now lets parse the data,Uint64,Address,String,Blob
		switch p.Type {
		case Uint64:
			if variable.ValueUint64, err = strconv.ParseUint(value.(string), 0, 64); err != nil {
				return
			}
		case String:
			variable.ValueString = value.(string)

		default:
			panic("unknown parameter type cannot have parameters")

		}

		dvm.Locals[variable.Name] = variable
	}

	// all variables have been collected, start interpreter
	dvm.ReturnValue = dvm.f.ReturnValue // enforce return value to be of same type

	dvm.State.Monitor_recursion++ // higher recursion

	err = dvm.interpret_SmartContract()
	if err != nil {
		return
	}

	result = dvm.ReturnValue

	return
}

// it is similar to internal functions, however it enforces the condition that only Exportable functions are callable
// any function which has first character ASCII and upper case  is considered an exported function
func RunSmartContract(SC *SmartContract, EntryPoint string, state *Shared_State, params map[string]interface{}) (result Variable, err error) {
	// if smart contract does not contain function, trigger exception

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Recovered in function %+v stack %s", r, string(debug.Stack()))
		}

	}()

	r, size := utf8.DecodeRuneInString(EntryPoint)

	if r == utf8.RuneError || size == 0 {
		return result, fmt.Errorf("Invalid function name")

	}

	if r >= unicode.MaxASCII {
		return result, fmt.Errorf("Invalid function name, First character must be ASCII alphabet")
	}

	if !unicode.IsLetter(r) {
		return result, fmt.Errorf("Invalid function name, First character must be ASCII Letter")
	}

	if !unicode.IsUpper(r) {
		return result, fmt.Errorf("Invalid function name, First character must be Capital/Upper Case")
	}

	// initialize RND
	if state.Monitor_recursion == 0 {
		state.RND = Initialize_RND(state.Chain_inputs.SCID, state.Chain_inputs.BLID, state.Chain_inputs.TXID)
		state.Assets_Transfer = map[string]map[string]uint64{}
	}

	result, err = runSmartContract_internal(SC, EntryPoint, state, params)

	if err != nil {
		return result, err
	}
	if state.Monitor_recursion != 0 { // recursion must be zero at end
		return result, fmt.Errorf("Invalid  recursion level %d", state.Monitor_recursion)
	}

	// if recursion level is zero, we should check return value and persist the state changes

	return result, err
}

// this structure is all the inputs that are available to SC during execution
type Blockchain_Input struct {
	SCID          crypto.Hash // current smart contract which is executing
	BLID          crypto.Hash // BLID
	TXID          crypto.Hash // current TXID under which TX
	Signer        string      // address which signed this, inn 33 byte form
	BL_HEIGHT     uint64      // current chain height under which current tx is valid
	BL_TOPOHEIGHT uint64      // current block topo height which can be used to  uniquely pinpoint the block
	BL_TIMESTAMP  uint64      // epoch second resolution
}

// all DVMs triggered by the first call, will share this structure
// sharing this structure means RND number attacks are not possible
// all storage state is shared, this means something similar to solidity delegatecall
// this is necessary to prevent number of attacks
type Shared_State struct {
	SCIDZERO crypto.Hash // points to DERO SCID , which is zero
	SCIDSELF crypto.Hash // points to SELF SCID, this separation is necessary, if we enable cross SC calls
	// but note they bring all sorts of mess, bugs
	Persistance     bool // whether the results will be persistant or it's just a demo/test call
	Trace           bool // enables tracing to screen
	GasComputeUsed  int64
	GasComputeLimit int64
	GasComputeCheck bool // if gascheck is true, bail out as soon as limit is breached
	GasStoreUsed    int64
	GasStoreLimit   int64
	GasStoreCheck   bool // storage gas, bail out as soon as limit is breached

	Chain_inputs *Blockchain_Input // all blockchain info is available here

	Assets          map[crypto.Hash]uint64       // all assets supplied with this tx, including DERO main asset
	Assets_Transfer map[string]map[string]uint64 // any Assets that this TX wants to send OUT
	// transfers are only processed after the contract has terminated successfully

	RamStore map[Variable]Variable

	RND   *RND        // this is initialized only once  while invoking entrypoint
	Store *TX_Storage // mechanism to access a data store, can discard changes

	Monitor_recursion         int64 // used to control recursion amount 64 calls are more than necessary
	Monitor_lines_interpreted int64 // number of lines interpreted
	Monitor_ops               int64 // number of ops evaluated, for expressions, variables

}

// consumr and check compute gas
func (state *Shared_State) ConsumeGas(c int64) {
	if state != nil {
		state.GasComputeUsed += c
		if state.GasComputeCheck && state.GasComputeUsed > state.GasComputeLimit {
			panic("Insufficient Gas")
		}
	}
}

// consume and check storage gas
func (state *Shared_State) ConsumeStorageGas(c int64) {
	if state != nil {
		state.GasStoreUsed += c
		if state.GasStoreCheck && state.GasStoreUsed > state.GasStoreLimit {
			panic("Insufficient Storage Gas")
		}
	}
}

type DVM_Interpreter struct {
	Version     semver.Version // current version set by setversion
	SCID        string
	SC          *SmartContract
	EntryPoint  string
	f           Function
	IP          uint64              // current line number
	ReturnValue Variable            // Result of current function call
	Locals      map[string]Variable // all local variables

	Chain_inputs *Blockchain_Input // all blockchain info is available here

	State *Shared_State // all shared state between  DVM is available here

	RND *RND // this is initialized only once  while invoking entrypoint

	store *TX_Storage // mechanism to access a data store, can discard changes

}

func (i *DVM_Interpreter) incrementIP(newip uint64) (line []string, err error) {
	var ok bool

try_again:
	if newip == 0 { // we are simply falling through
		index := i.f.LinesNumberIndex[i.IP] // find the pos in the line numbers and find the current line_number
		index++
		if i.IP == 0 {
			index = 0 // start from first line
		}
		if uint64(len(i.f.LineNumbers)) <= index { // if function does not contain more lines to execute, return
			err = fmt.Errorf("No lines after line number (%d) in SC %s in function %s", i.IP, i.SCID, i.f.Name)
			return
		}

		i.IP = i.f.LineNumbers[index]

	} else { // we have a GOTO and must jump to the line numbr mentioned
		i.IP = newip
	}
	line, ok = i.f.Lines[i.IP]
	if !ok {
		err = fmt.Errorf("No such line number (%d) in SC %s in function %s", i.IP, i.SCID, i.f.Name)
		return
	}
	i.State.Monitor_lines_interpreted++ // increment line interpreted

	if len(line) == 0 { // increment to next line
		goto try_again
	}
	return
}

// this runs a smart contract function with specific params
func (i *DVM_Interpreter) interpret_SmartContract() (err error) {

	newIP := uint64(0)
	for {
		var line []string
		line, err = i.incrementIP(newIP)
		if err != nil {
			return
		}

		i.State.ConsumeGas(5000) // every line number has some gas costs

		newIP = 0 // this is necessary otherwise, it will trigger an infinite loop in the case given below

		/*
			    * Function SetOwner(value Uint64, newowner String) Uint64
				10  IF LOAD("owner") == SIGNER() THEN GOTO 30
				20  RETURN 1
				30  STORE("owner",newowner)
				40  RETURN 0
				End Function
		*/

		if i.State.Monitor_lines_interpreted > LIMIT_interpreted_lines {
			panic(fmt.Sprintf("%d lines interpreted, reached limit %d", LIMIT_interpreted_lines, LIMIT_interpreted_lines))
		}

		//fmt.Printf("received line to interpret %+v err\n", line, err)
		switch {
		case strings.EqualFold(line[0], "DIM"):
			newIP, err = i.interpret_DIM(line[1:])
		case strings.EqualFold(line[0], "LET"):
			newIP, err = i.interpret_LET(line[1:])
		case strings.EqualFold(line[0], "GOTO"):
			newIP, err = i.interpret_GOTO(line[1:])
		case strings.EqualFold(line[0], "IF"):
			newIP, err = i.interpret_IF(line[1:])
		case strings.EqualFold(line[0], "RETURN"):
			newIP, err = i.interpret_RETURN(line[1:])

		//ability to print something for debugging purpose
		case strings.EqualFold(line[0], "PRINT"):
			fallthrough
		case strings.EqualFold(line[0], "PRINTF"):
			newIP, err = i.interpret_PRINT(line[1:])

			// if we are here, the first part is unknown

		default:

			// we should try to evaluate expression and make sure it's  a function call
			// now lets evaluate the expression

			expr, err1 := parser.ParseExpr(replacer.Replace(strings.Join(line, " ")))
			if err1 != nil {
				err = err1
				return
			}

			if _, ok := expr.(*ast.CallExpr); !ok {
				return fmt.Errorf("not a function call line %+v\n", line)
			}
			i.eval(expr)

		}

		if i.State.Trace {
			fmt.Printf("interpreting line %+v   err:'%v'\n", line, err)
		}
		if err != nil {
			err = fmt.Errorf("err while interpreting line %+v err %s\n", line, err)
			return
		}
		if newIP == math.MaxUint64 {
			break
		}
	}
	return
}

// this is very limited and can be used print only variables
func (dvm *DVM_Interpreter) interpret_PRINT(args []string) (newIP uint64, err error) {
	var variable Variable
	var ok bool
	if len(args) > 0 {
		params := []interface{}{}
		for i := 1; i < len(args); i++ {
			if variable, ok = dvm.Locals[args[i]]; !ok { // TODO what about printing globals
				/*if variable,ok := dvm.Locals[exp.Name];!ok{

				  }*/

			}
			if ok {
				switch variable.Type {
				case Uint64:
					params = append(params, variable.ValueUint64)
				case String:
					params = append(params, variable.ValueString)

				default:
					panic("Unhandled data_type")
				}

			} else {
				params = append(params, fmt.Sprintf("unknown variable %s", args[i]))
			}
		}

		//_, err = fmt.Printf(strings.Trim(args[0], "\"")+"\n", params...)
	}
	return
}

// process DIM line
func (dvm *DVM_Interpreter) interpret_DIM(line []string) (newIP uint64, err error) {

	if len(line) <= 2 || !strings.EqualFold(line[len(line)-2], "as") {
		return 0, fmt.Errorf("Invalid DIM syntax")
	}

	// check last data type
	data_type := check_valid_type(line[len(line)-1])
	if data_type == Invalid {
		return 0, fmt.Errorf("function name \"%s\", No such Data type \"%s\"", dvm.f.Name, line[len(line)-1])
	}

	for i := 0; i < len(line)-2; i++ {
		if line[i] != "," { // ignore separators

			if !check_valid_name(line[i]) {
				return 0, fmt.Errorf("function name \"%s\", variable name \"%s\" contains invalid characters", dvm.f.Name, line[i])
			}

			// check whether variable is already defined
			if _, ok := dvm.Locals[line[i]]; ok {
				return 0, fmt.Errorf("function name \"%s\", variable name \"%s\" contains invalid characters", dvm.f.Name, line[i])
			}

			// all data variables are pre-initialized

			switch data_type {
			case Uint64:
				dvm.Locals[line[i]] = Variable{Name: line[i], Type: Uint64, ValueUint64: uint64(0)}
			case String:
				dvm.Locals[line[i]] = Variable{Name: line[i], Type: String, ValueString: ""}

			default:
				panic("Unhandled data_type")
			}
			// fmt.Printf("Initialising variable %s %+v\n",line[i],dvm.Locals[line[i]])

		}
	}

	return
}

// process LET statement
func (dvm *DVM_Interpreter) interpret_LET(line []string) (newIP uint64, err error) {

	if len(line) <= 2 || !strings.EqualFold(line[1], "=") {
		err = fmt.Errorf("Invalid LET syntax")
		return
	}

	if _, ok := dvm.Locals[line[0]]; !ok {
		err = fmt.Errorf("function name \"%s\", variable name \"%s\"  is used without definition", dvm.f.Name, line[0])
		return
	}
	result := dvm.Locals[line[0]]

	expr, err := parser.ParseExpr(strings.Join(line[2:], " "))
	if err != nil {
		return
	}

	expr_result := dvm.eval(expr)
	//fmt.Printf("expression %s = %+v\n", line[0],expr_result)

	//fmt.Printf(" %+v \n", dvm.Locals[line[0]])
	switch result.Type {
	case Uint64:
		result.ValueUint64 = expr_result.(uint64)
	case String:
		result.ValueString = expr_result.(string)

	default:
		panic("Unhandled data_type")
	}

	dvm.Locals[line[0]] = result
	//  fmt.Printf(" %+v \n", dvm.Locals[line[0]])

	return
}

// process GOTO line
func (dvm *DVM_Interpreter) interpret_GOTO(line []string) (newIP uint64, err error) {

	if len(line) != 1 {
		err = fmt.Errorf("GOTO  contains 1 mandatory line number as argument")
		return
	}

	newIP, err = strconv.ParseUint(line[0], 0, 64)
	if err != nil {
		return
	}

	if newIP == 0 || newIP == math.MaxUint64 {
		return 0, fmt.Errorf("GOTO  has invalid line number \"%d\"", newIP)
	}
	return
}

// process IF line
// IF has two forms  vis  x,y are line numbers
// IF expr THEN GOTO x
// IF expr THEN GOTO x ELSE GOTO y
func (dvm *DVM_Interpreter) interpret_IF(line []string) (newIP uint64, err error) {

	thenip := uint64(0)
	elseip := uint64(0)

	// first form of IF
	if len(line) >= 4 && strings.EqualFold(line[len(line)-3], "THEN") && strings.EqualFold(line[len(line)-2], "GOTO") {

		thenip, err = strconv.ParseUint(line[len(line)-1], 0, 64)
		if err != nil {
			return
		}
		line = line[:len(line)-3]

	} else if len(line) >= 7 && strings.EqualFold(line[len(line)-6], "THEN") && strings.EqualFold(line[len(line)-5], "GOTO") && strings.EqualFold(line[len(line)-3], "ELSE") && strings.EqualFold(line[len(line)-2], "GOTO") {

		thenip, err = strconv.ParseUint(line[len(line)-4], 0, 64)
		if err != nil {
			return
		}

		elseip, err = strconv.ParseUint(line[len(line)-1], 0, 64)
		if err != nil {
			return
		}

		if elseip == 0 || elseip == math.MaxUint64 {
			return 0, fmt.Errorf("ELSE GOTO  has invalid line number \"%d\"", thenip)
		}

		line = line[:len(line)-6]
	} else {
		err = fmt.Errorf("Invalid IF syntax")
		return
	}

	if thenip == 0 || thenip == math.MaxUint64 {
		return 0, fmt.Errorf("THEN GOTO  has invalid line number \"%d\"", thenip)
	}

	// now lets evaluate the expression

	expr, err := parser.ParseExpr(replacer.Replace(strings.Join(line, " ")))
	if err != nil {
		return
	}

	expr_result := dvm.eval(expr)
	//fmt.Printf("if %d %T expr( %s)\n", expr_result, expr_result, replacer.Replace(strings.Join(line, " ")))
	if result, ok := expr_result.(uint64); ok {
		if result != 0 {
			newIP = thenip
		} else {
			newIP = elseip
		}
	} else {

		err = fmt.Errorf("Invalid IF expression  \"%s\"", replacer.Replace(strings.Join(line, " ")))
	}

	return

}

// process RETURN line
func (dvm *DVM_Interpreter) interpret_RETURN(line []string) (newIP uint64, err error) {

	if dvm.ReturnValue.Type == Invalid {
		if len(line) != 0 {
			err = fmt.Errorf("function name \"%s\" cannot return anything", dvm.f.Name)
			return
		}

		dvm.State.Monitor_recursion-- // lower recursion
		newIP = math.MaxUint64        // simple return
		return
	}

	if len(line) == 0 {
		err = fmt.Errorf("function name \"%s\" should return  a value", dvm.f.Name)
		return
	}

	// we may be returning an expression which must be solved
	expr, err := parser.ParseExpr(replacer.Replace(strings.Join(line, " ")))
	if err != nil {
		return
	}

	expr_result := dvm.eval(expr)
	//fmt.Printf("expression %+v %T\n", expr_result, expr_result)

	switch dvm.ReturnValue.Type {
	case Uint64:
		dvm.ReturnValue.ValueUint64 = expr_result.(uint64)
	case String:
		dvm.ReturnValue.ValueString = expr_result.(string)

	default:
		panic("unexpected data type")

	}

	dvm.State.Monitor_recursion-- // lower recursion
	newIP = math.MaxUint64        // simple return

	return
}

// only returns identifiers
func (dvm *DVM_Interpreter) eval_identifier(exp ast.Expr) string {
	switch exp := exp.(type) {
	case *ast.Ident: // it's a variable,
		return exp.Name
	default:
		panic("expecting identifier")
	}
}

func (dvm *DVM_Interpreter) eval(exp ast.Expr) interface{} {

	dvm.State.Monitor_ops++ // maintain counter

	if dvm.State.Monitor_ops > LIMIT_evals {
		panic(fmt.Sprintf("%d lines interpreted, evals reached limit %d", dvm.State.Monitor_lines_interpreted, LIMIT_evals))
	}

	//fmt.Printf("exp %+v  %T\n", exp, exp)
	switch exp := exp.(type) {
	case *ast.ParenExpr:
		return dvm.eval(exp.X)

	case *ast.UnaryExpr: // there are 2 unary operators, one is binary NOT , second is logical not
		switch exp.Op {
		case token.XOR:
			return ^(dvm.eval(exp.X).(uint64))
		case token.NOT:
			x := dvm.eval(exp.X)
			switch x := x.(type) {
			case uint64:
				return ^x
			case string:
				if IsZero(x) == 1 {
					return uint64(1)
				}
				return uint64(0)

			}
		}

	case *ast.BinaryExpr:
		return dvm.evalBinaryExpr(exp)
	case *ast.Ident: // it's a variable,
		if _, ok := dvm.Locals[exp.Name]; !ok {
			panic(fmt.Sprintf("function name \"%s\", variable name \"%s\"  is used without definition", dvm.f.Name, exp.Name))

		}
		//fmt.Printf("value %s %d\n",exp.Name,  dvm.Locals[exp.Name].Value)

		switch dvm.Locals[exp.Name].Type {
		case Uint64:
			return dvm.Locals[exp.Name].ValueUint64
		case String:
			return dvm.Locals[exp.Name].ValueString
		default:
			panic("unexpected data type")
		}

	// there are 2 types of calls, one within the smartcontract
	// other one crosses smart contract boundaries
	case *ast.CallExpr:
		func_name := dvm.eval_identifier(exp.Fun)
		//fmt.Printf("Call expression %+v %s \"%s\" \n",exp,exp.Fun, func_name)
		// if call is internal
		//

		// try to handle internal functions, SC function cannot overide internal functions
		if ok, result := dvm.Handle_Internal_Function(exp, func_name); ok {
			return result
		}
		function_call, ok := dvm.SC.Functions[func_name]
		if !ok {
			panic(fmt.Sprintf("Unknown function called \"%s\"", exp.Fun))
		}
		if len(function_call.Params) != len(exp.Args) {
			panic(fmt.Sprintf("function \"%s\" called with incorrect number of arguments , expected %d , actual %d", func_name, len(function_call.Params), len(exp.Args)))
		}

		arguments := map[string]interface{}{}
		for i, p := range function_call.Params {
			switch p.Type {
			case Uint64:
				arguments[p.Name] = fmt.Sprintf("%d", dvm.eval(exp.Args[i]).(uint64))
			case String:
				arguments[p.Name] = dvm.eval(exp.Args[i]).(string)
			}
		}

		// allow calling unexported functions
		result, err := runSmartContract_internal(dvm.SC, func_name, dvm.State, arguments)
		if err != nil {
			panic(err)
		}
		switch function_call.ReturnValue.Type {
		case Uint64:
			return result.ValueUint64
		case String:
			return result.ValueString
			//default:
			//      	panic(fmt.Sprintf("unexpected data type %T", function_call.ReturnValue.Type))
		}
		return nil

	case *ast.BasicLit:
		switch exp.Kind {
		case token.INT:
			i, err := strconv.ParseUint(exp.Value, 0, 64)
			if err != nil {
				panic(err)
			}
			return i
		case token.STRING:
			unquoted, err := strconv.Unquote(exp.Value)
			if err != nil {
				panic(err)
			}
			return unquoted
		}
	default:
		panic(fmt.Sprintf("Unhandled expression type %+v", exp))

	}

	panic("We should never reach here while evaluating expressions")
	return 0
}

// this can be used to check whether variable has a default value
// for uint64 , it is 0
// for string , it is ""
// TODO Address, Blob
func IsZero(value interface{}) uint64 {
	switch v := value.(type) {
	case uint64:
		if v == 0 {
			return 1
		}
	case string:
		if v == "" {
			return 1
		}

	default:
		panic("IsZero not being handled")

	}

	return 0
}

func (dvm *DVM_Interpreter) evalBinaryExpr(exp *ast.BinaryExpr) interface{} {

	dvm.State.ConsumeGas(800) // every expr evaluation has some cost

	left := dvm.eval(exp.X)
	right := dvm.eval(exp.Y)

	//fmt.Printf("left '%+v  %T' %+v  right '%+v  %T'\n", left, left, exp.Op, right, right)

	// special case to append uint64 to strings
	if fmt.Sprintf("%T", left) == "string" && fmt.Sprintf("%T", right) == "uint64" {
		return left.(string) + fmt.Sprintf("%d", right)
	}

	if fmt.Sprintf("%T", left) != fmt.Sprintf("%T", right) {
		panic(fmt.Sprintf("Expressions cannot be different type(String/Uint64) left (val %+v %+v)   right (%+v %+v)", left, exp.X, right, exp.Y))
	}

	// logical ops are handled differently
	switch exp.Op {
	case token.LAND:
		if (IsZero(left) == 0) && (IsZero(right) == 0) { // both sides should be set
			return uint64(1)
		}

		return uint64(0)
	case token.LOR:
		//fmt.Printf("left %d   right %d\n", left,right)
		//fmt.Printf("left %v   right %v\n", (IsZero(left) != 0),(IsZero(right) != 0))
		if (IsZero(left) == 0) || (IsZero(right) == 0) {
			return uint64(1)
		}
		return uint64(0)
	}

	// handle string operands
	if fmt.Sprintf("%T", left) == "string" {
		left_string := left.(string)
		right_string := right.(string)

		switch exp.Op {
		case token.ADD:
			if len(left_string)+len(right_string) >= 1024*1024 {
				panic("too big string value")
			}
			return left_string + right_string
		case token.EQL:
			if left_string == right_string {
				return uint64(1)
			}
			return uint64(0)
		case token.NEQ:
			if left_string != right_string {
				return uint64(1)
			}
			return uint64(0)
		default:
			panic(fmt.Sprintf("String data type only support addition operation ('%s') not supported", exp.Op))
		}
	}

	left_uint64 := left.(uint64)
	right_uint64 := right.(uint64)

	switch exp.Op {
	case token.ADD:
		return left_uint64 + right_uint64 // TODO : can we add rounding case here and raise exception
	case token.SUB:
		return left_uint64 - right_uint64 // TODO : can we add rounding case here and raise exception
	case token.MUL:
		return left_uint64 * right_uint64
	case token.QUO:
		return left_uint64 / right_uint64
	case token.REM:
		return left_uint64 % right_uint64

		//bitwise ops
	case token.AND:
		return left_uint64 & right_uint64
	case token.OR:
		return left_uint64 | right_uint64
	case token.XOR:
		return left_uint64 ^ right_uint64
	case token.SHL:
		return left_uint64 << right_uint64
	case token.SHR:
		return left_uint64 >> right_uint64

	case token.EQL:
		if left_uint64 == right_uint64 {
			return uint64(1)
		}
	case token.NEQ:
		if left_uint64 != right_uint64 {
			return uint64(1)
		}
	case token.LEQ:
		if left_uint64 <= right_uint64 {
			return uint64(1)
		}
	case token.GEQ:
		if left_uint64 >= right_uint64 {
			return uint64(1)
		}
	case token.LSS:
		if left_uint64 < right_uint64 {
			return uint64(1)
		}
	case token.GTR:
		if left_uint64 > right_uint64 {
			return uint64(1)
		}
	default:
		panic("This operation cannot be handled")
	}
	return uint64(0)
}
