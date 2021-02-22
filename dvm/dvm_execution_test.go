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
import "reflect"
import "testing"

//import "github.com/deroproject/derosuite/crypto"

var execution_tests = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"demo 1",
		`Function TestRun(s Uint64) Uint64
                 10 Return s
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"s": "8"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	},
	{
		"invalid return expression",
		`Function TestRun(s Uint64) Uint64
                 10 Return 1s
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"s": "8"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function with 2 params",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 + a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "4", "a2": "4"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function with 2 string params",
		`Function TestRun(a1 String,a2 String) String
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 + a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "4", "a2": "4"},
		nil,
		Variable{Type: String, Value: "44"},
	},

	// test all arithmetic operations
	{
		"valid  function  testing  substraction ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 - a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "12", "a2": "4"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  multiplication ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 * a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "4"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  division ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 / a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "25", "a2": "3"}, // it is 25 to confirm we are doing integer division
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  modulus ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 % a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "35", "a2": "9"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  SHL ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 << a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "4", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  SHR ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 >> a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "32", "a2": "2"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  OR ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 | a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "8", "a2": "8"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  AND ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 & a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "9", "a2": "10"},
		nil,
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"valid  function  testing  NOT ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  !a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "9", "a2": "10"},
		nil,
		Variable{Type: Uint64, Value: uint64(18446744073709551606)}, //NOT 9 == 18446744073709551606
	}, {
		"valid  function  testing  ^XOR ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  ^a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "9", "a2": "10"},
		nil,
		Variable{Type: Uint64, Value: uint64(18446744073709551606)}, //NOT 9 == 18446744073709551606
	}, {
		"valid  function  testing  XOR ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 ^ a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "60", "a2": "13"},
		nil,
		Variable{Type: Uint64, Value: uint64(49)},
	}, {
		"valid  function  testing  ||  ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 || a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "0", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"valid  function  testing  &&  1 ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 && a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "0", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(0)},
	}, {
		"valid  function  testing  &&  2",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  (a1 && a2)
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(1)},
	},

	{
		"valid  function  testing  &&  3",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  0 || (a1 && a2)
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(1)},
	},
	{
		"valid  function  testing  LET ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"valid  function  testing  IF THEN form1  fallthrough",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"valid  function  testing  IF THEN form1  THEN case",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(0)},
	},
	{
		"valid  function  testing  IF THEN ELSE form1  THEN case",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30 ELSE GOTO 20
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(0)},
	}, {
		"valid  function  testing  IF THEN ELSE form1  ELSE case",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30 ELSE GOTO 20
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"valid  function  testing  != success ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 != 3 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	},
	{
		"valid  function  testing  !=  failed",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 != 3 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "3", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},

	{
		"valid  function  testing  <> ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 <> 3 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"invalid operator testing  = ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 = 3 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(99)},
	},
	{
		"valid  function  testing  > success ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 > 1 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing  > failed ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if 1 > a1 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing  >= success",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 >= 2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing  >= failed",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 >= 2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing  < success ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 < 3 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing  < failed ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 < 3 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "5", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing  <= success ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 <= 2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing  <= success ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 <= 2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "4", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing string == success",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 == a2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "asdf", "a2": "asdf"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing string == success",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 == "asdf" then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "asdf", "a2": "asdf"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing string == failed",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 == a2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "asdf", "a2": "asdf1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing !string  success ",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if !a1  then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "", "a2": "asdf1"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing !string  fail ",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if !a1  then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "a1", "a2": "asdf1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing string != ",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 != a2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "asdf", "a2": "asdfz"},
		nil,
		Variable{Type: Uint64, Value: uint64(99)},
	}, {
		"valid  function  testing LOR ",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 != a2 || a1 != a2 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "asdf", "a2": "asdf"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  comparision of uint64 /string ",
		`Function TestRun(a1 String,a2 String) Uint64
		 10 dim s1, s2 as Uint64
		 13 LET s1 = 99
		 15 if a1 != s1 then GOTO 30 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "asdf", "a2": "asdfz"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(99)},
	},

	{
		"valid  function  arbitrary function evaluation",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 fact_recursive(10)
		 20 LET s1 = fact_recursive(10)
		 30 return  s1
                 End Function
                 Function fact_recursive(s Uint64) Uint64
                    10  IF s == 1 THEN GOTO 20 ELSE GOTO 30
                    20  RETURN 1
                    30  RETURN  s * fact_recursive(s -1)
                    End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(3628800)},
	},

	{
		"Invalid  function with 2 string params substractions",
		`Function TestRun(a1 String,a2 String) String
		 10 dim s1, s2 as Uint64
		 20 
		 30 return  a1 - a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "4", "a2": "4"},
		fmt.Errorf("dummy"),
		Variable{Type: String, Value: "44"},
	},
}

// run the test
func Test_execution(t *testing.T) {
	for _, test := range execution_tests {

		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)

		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}

	}
}

// ensure 100% coverage of IF
var execution_tests_if = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"valid  function  testing  IF THEN form1  fallthrough",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"valid  function  testing  IF THEN form1  THEN case",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(0)},
	},
	{
		"valid  function  testing  IF THEN ELSE form1  THEN case",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30 ELSE GOTO 20
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(0)},
	}, {
		"valid  function  testing  IF THEN ELSE form1  ELSE case",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30 ELSE GOTO 20
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},

	{
		"invalid  function  testing  IF THEN form1  malformed if then",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 = != 2 thenn GOTO 30
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  IF THEN form1  malformed else",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 30 ELSEE GOTO 20
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  IF THEN form1  malformed then line number",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 0 ELSE GOTO 20
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  IF THEN form1  malformed then else number",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 20 ELSE GOTO 0
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  IF THEN form1  malformed unparseable then line number",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO ewr 
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  IF THEN ELSE form1  malformed unparseable then line number",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO ewr ELSE GOTO 20
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  IF THEN form1  malformed  unparseable  else number",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 20 ELSE GOTO ewr
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
	{
		"invalid  function  testing  IF THEN form1  unknownelse number",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 if a1 == 2 then GOTO 20 ELSE GOTO 43535
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  IF THEN  invalid IF expression",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 13 dim x1 as String
		 15 if x1 then GOTO 20 ELSE GOTO 43535
		 20 RETURN 2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "77", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
}

// run the test
func Test_IF_execution(t *testing.T) {
	for _, test := range execution_tests_if {

		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)

		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}

	}
}

// ensure 100% coverage of DIM
var execution_tests_dim = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"valid  function  testing  DIM ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing  DIM address",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Address
		 15 GOTO 30
		 17 RETURN 0
		 20 
		 30 return  a1 + a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing  DIM blob",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Blob
		 15 GOTO 30
		 17 RETURN 0
		 20 
		 30 return  a1 + a2
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid dim expression collision between arguments and local variable",
		`Function TestRun(s Uint64) Uint64
		 10 dim s as Uint64
		 20 
		 30 return 8
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"s": "8"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"invalid dim expression  invalid variable name",
		`Function TestRun(s Uint64) Uint64
		 10 dim 1s as Uint64
		 20 
		 30 return 8
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"s": "8"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"invalid dim expression  invalid 2nd variable  name",
		`Function TestRun(s Uint64) Uint64
		 10 dim s1,2s2 as Uint64
		 20 
		 30 return 8
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"s": "8"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"invalid dim expression  invalid dim syntax",
		`Function TestRun(s Uint64) Uint64
		 10 dim tu ajs Uint64
		 20 
		 30 return 8
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"s": "8"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(8)},
	}, {
		"invalid dim expression  invalid dim syntax",
		`Function TestRun(s Uint64) Uint64
		 10 dim tu as UUint64
		 20 
		 30 return 8
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"s": "8"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(8)},
	},
}

// run the test
func Test_DIM_execution(t *testing.T) {
	for _, test := range execution_tests_dim {

		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)

		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}

	}
}

// ensure 100% coverage of LET
var execution_tests_let = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"valid  function  testing  LET ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing  LET Address",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 12 dim add1,add2 as Address
		 13 let  add1 = add2
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"valid  function  testing  LET blob",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 12 dim add1, add2 as Blob
		 13 let  add1 = add2
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  LET ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 LET 
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  LET  undefined local variable",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 LET s3 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  LET malformed expression",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 LET s1 = ((a1) + a2 
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  LET putting int into string",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as String
		 20 LET s1 = (a1) + a2 
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
}

// run the test
func Test_LET_execution(t *testing.T) {
	for _, test := range execution_tests_let {

		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)

		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}

	}
}

// ensure 100% coverage of PRINT
var execution_tests_print = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"valid  function  testing  PRINT ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 
		 17 PRINT "%d %d" s1 s2
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  PRINT ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 
		 17 PRINT "%d %d" s1 s2 s3
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	},
}

// run the test
func Test_PRINT_execution(t *testing.T) {
	for _, test := range execution_tests_print {

		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)

		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}

	}
}

// generic cases of coding mistakes
var execution_tests_generic = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"invalid  function  testing  Generic case if function does not return ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 
		 17 PRINT "%d %d" s1 s2
		 20 LET s1 = a1 + a2
		 // 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  Generic case if invalid expression evaluated ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 
		 17 dummy (s1 s2 s3
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  Generic case if valid expression but not a function call ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 
		 17 2 +3
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},

	// these case check callability of unexported functions etc
	{
		"valid  function  testing  Invalid Rune ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		string([]byte{0x80}), // EntryPoint checking
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"valid  function  testing  executing first non-ascii function ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		"☺☻TestRun", // EntryPoint checking
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"valid  function  testing  executing first non-letter function",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		"1TestRun", // EntryPoint checking
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"valid  function  testing  executing unexported function  ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		"testRun", // EntryPoint checking
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"invalid  function  testing  executing non-existing function ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		"TestRun1", // no function exists
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	},

	{
		"valid  function  testing  parameter missing ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a3": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	},

	{
		"valid  function  testing  Invalid parameter uint64 ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "-1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	},
	{
		"valid  function  testing  use unknown variable ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1*a3
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"valid  function  testing  use uint64 larger than 64 bits ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1* 118446744073709551615 
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	},

	{
		"valid  function  testing  recursive FACTORIAL ",
		`Function fact_recursive(s Uint64) Uint64
	10  IF s == 1 THEN GOTO 20 ELSE GOTO 30
	20  RETURN 1
	30  RETURN  s * fact_recursive(s -1)
	End Function
	Function TestRun(a1 Uint64) Uint64
	10 dim result as Uint64
	20 LET result = fact_recursive(a1)
        60  printf "FACTORIAL of %d = %d" s result 
	70  RETURN result
	End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "10", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(3628800)},
	},

	{
		"invalid  function  testing   paramter skipped ",
		`Function fact_recursive(s Uint64) Uint64
	10  IF s == 1 THEN GOTO 20 ELSE GOTO 30
	20  RETURN 1
	30  RETURN  s * fact_recursive(s -1)
	End Function
	Function TestRun(a1 Uint64) Uint64
	10 dim result as Uint64
	20 LET result = fact_recursive()
        60  printf "FACTORIAL of %d = %d" s result 
	70  RETURN result
	End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "10", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(3628800)},
	},
	{
		"invalid  function  testing   unknown function called ",
		`Function fact_recursive(s Uint64) Uint64
	10  IF s == 1 THEN GOTO 20 ELSE GOTO 30
	20  RETURN 1
	30  RETURN  s * fact_recursive(s -1)
	End Function
	Function TestRun(a1 Uint64) Uint64
	10 dim result as Uint64
	20 LET result = fact_recursive_unknown(a1)
        60  printf "FACTORIAL of %d = %d" s result 
	70  RETURN result
	End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "10", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(3628800)},
	},
	{
		"invalid  function  testing   error withing sub function call",
		`Function fact_recursive(s Uint64) Uint64
	10  IF s == 1 THEN GOTO 20 ELSE GOTO 30
	20  RETURN 1
	30  RETURN  s * fact_recursive(s -1) *  (
	End Function
	Function TestRun(a1 Uint64) Uint64
	10 dim result as Uint64
	20 LET result = fact_recursive(a1)
        60  printf "FACTORIAL of %d = %d" s result 
	70  RETURN result
	End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "10", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(3628800)},
	},

	{
		"valid  function  testing   sub function call without return",
		`Function dummy(s Uint64) 

	20  RETURN 
	End Function
	Function TestRun(a1 Uint64) Uint64
	10 dim result as Uint64
	20 LET result = a1
        60  dummy(a1)
	70  RETURN result
	End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "10", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(10)},
	},
	{
		"valid  function  testing  recursive FACTORIAL ",
		`Function fact_recursive(s Uint64, a Address, b Blob, str String) Uint64
	10  IF s == 1 THEN GOTO 20 ELSE GOTO 30
	20  RETURN 1
	30  RETURN  s * fact_recursive(s -1,a,b,str)
	End Function
	Function TestRun(a1 Uint64,a Address, b Blob, str String) Uint64
	10 dim result as Uint64
	20 LET result = fact_recursive(a1,a,b,str)
        60  printf "FACTORIAL of %d = %d" s result 
	70  RETURN result
	End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "10", "a2": "1", "a": "address param", "b": "blob param", "str": "str_param"},
		nil,
		Variable{Type: Uint64, Value: uint64(3628800)},
	},
	{
		"valid  function  testing  recursive FACTORIAL ",
		`Function fact_recursive(s Uint64, a Address, b Blob, str String) Uint64
	10  IF s == 1 THEN GOTO 20 ELSE GOTO 30
	20  RETURN 1
	30  RETURN  s * fact_recursive(s -1,a,b,str)
	End Function
	Function TestRun(a1 Uint64,a Address, b Blob, str String) Uint64
	10 dim result as Uint64
	20 LET result = fact_recursive(a1,a,b,str)
        60  printf "FACTORIAL of %d = %d" s result 
	70  RETURN result
	End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "10", "a2": "1", "a": "address param", "b": "blob param", "str": "str_param"},
		nil,
		Variable{Type: Uint64, Value: uint64(3628800)},
	},
}

// run the test
func Test_Generic_execution(t *testing.T) {
	for _, test := range execution_tests_generic {

		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)

		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}

	}
}

// ensure 100% coverage of RETURN
var execution_tests_return = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"valid  function  testing  RETURN uint64 ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"valid  function  testing  RETURN String ",
		`Function TestRun(a1 String,a2 Uint64) String
		 30 return  a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: String, Value: "1"},
	},
	{
		"valid  function  testing  RETURN Address ",
		`Function TestRun(a1 Address,a2 Uint64) Address
		 30 return  a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Address, Value: "1"},
	}, {
		"valid  function  testing  RETURN Blob ",
		`Function TestRun(a1 Blob,a2 Uint64) Blob
		 30 return  a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Blob, Value: "1"},
	}, {
		"valid  function  testing   non returning function returning nothing ",
		`Function TestRun(a1 Uint64,a2 Uint64) 
		 30 return  
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Invalid},
	},
	{
		"valid  function  testing   non returning function returning something ",
		`Function TestRun(a1 Uint64,a2 Uint64) 
		 30 return  a1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: "1"},
	}, {
		"invalid  function  testing  RETURN uint64 ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	}, {
		"invalid  function  testing  RETURN contains unparseable function ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 30 return  ( a1 
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(1)},
	},
}

// run the test
func Test_RETURN_execution(t *testing.T) {
	for _, test := range execution_tests_return {
		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}
		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)
		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}
	}
}

// ensure 100% coverage of GOTO
var execution_tests_goto = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"valid  function  testing  GOTO ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  GOTO ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO qeee
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  GOTO ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO 
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	}, {
		"invalid  function  testing  GOTO ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO 0
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  s1
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		fmt.Errorf("dummy"),
		Variable{Type: Uint64, Value: uint64(2)},
	},
}

// run the test
func Test_GOTO_execution(t *testing.T) {
	for _, test := range execution_tests_goto {
		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)
		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Error while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}
	}
}
