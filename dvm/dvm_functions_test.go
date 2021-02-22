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

//import "fmt"
import "reflect"
import "testing"

import "github.com/deroproject/derohe/cryptography/crypto"

// ensure 100% coverage of functions execution
var execution_tests_functions = []struct {
	Name       string
	Code       string
	EntryPoint string
	Args       map[string]interface{}
	Eerr       error    // execute error
	result     Variable // execution result
}{
	{
		"valid  function  testing  BLOCK_HEIGHT() ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  BLOCK_HEIGHT()
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(5)},
	},
	{
		"valid  function  testing  BLOCK_TOPOHEIGHT() ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  BLOCK_TOPOHEIGHT()
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, Value: uint64(9)},
	},
	{
		"valid  function  testing  SCID() ",
		`Function TestRun(a1 Uint64,a2 Uint64) String
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  SCID()
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: String, Value: crypto.ZEROHASH.String()},
	}, {
		"valid  function  testing  BLID() ",
		`Function TestRun(a1 Uint64,a2 Uint64) String
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  BLID()
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: String, Value: crypto.ZEROHASH.String()},
	}, {
		"valid  function  testing  TXID() ",
		`Function TestRun(a1 Uint64,a2 Uint64) String
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  TXID()
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: String, Value: crypto.ZEROHASH.String()},
	},
}

// run the test
func Test_FUNCTION_execution(t *testing.T) {
	for _, test := range execution_tests_functions {
		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{BL_HEIGHT: 5, BL_TOPOHEIGHT: 9, SCID: crypto.ZEROHASH,
			BLID: crypto.ZEROHASH, TXID: crypto.ZEROHASH}}
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
