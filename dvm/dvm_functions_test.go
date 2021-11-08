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
import "encoding/hex"

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
		Variable{Type: Uint64, ValueUint64: uint64(5)},
	},
	{
		"valid  function  testing  BLOCK_TIMESTAMP() ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 15 GOTO 20
		 17 RETURN 0
		 20 LET s1 = a1 + a2
		 30 return  BLOCK_TIMESTAMP()
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(9)},
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
		Variable{Type: String, ValueString: string(crypto.ZEROHASH[:])},
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
		Variable{Type: String, ValueString: string(crypto.ZEROHASH[:])},
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
		Variable{Type: String, ValueString: string(crypto.ZEROHASH[:])},
	}, {
		"valid  function  testing  Sha256() ",
		`Function TestRun(input String) String
		 30 return  SHA256(input)
         End Function`,
		"TestRun",
		map[string]interface{}{"input": "abc"},
		nil,
		Variable{Type: String, ValueString: string([]byte{0xba, 0x78, 0x16, 0xbf, 0x8f, 0x01, 0xcf, 0xea, 0x41, 0x41, 0x40, 0xde, 0x5d, 0xae, 0x22, 0x23, 0xb0, 0x03, 0x61, 0xa3, 0x96, 0x17, 0x7a, 0x9c, 0xb4, 0x10, 0xff, 0x61, 0xf2, 0x00, 0x15, 0xad})},
	},

	{
		"sha3256() blank string test ",
		`Function TestRun(input String) String
		 30 return  sha3256(input)
         End Function`,
		"TestRun",
		map[string]interface{}{"input": string("")},
		nil,
		Variable{Type: String, ValueString: string(decodeHex("A7FFC6F8BF1ED76651C14756A061D662F580FF4DE43B49FA82D80A4B80F8434A"))},
	},

	{
		"valid  function  testing  sha3256() ",
		`Function TestRun(input String) String
		 30 return  sha3256(input)
         End Function`,
		"TestRun",
		map[string]interface{}{"input": string([]byte{0xcc})},
		nil,
		Variable{Type: String, ValueString: string(decodeHex("677035391CD3701293D385F037BA32796252BB7CE180B00B582DD9B20AAAD7F0"))},
	},
	{
		"Keccak256() blank string test ",
		`Function TestRun(input String) String
		 30 return  KECCAK256(input)
         End Function`,
		"TestRun",
		map[string]interface{}{"input": string("")},
		nil,
		Variable{Type: String, ValueString: string(decodeHex("C5D2460186F7233C927E7DB2DCC703C0E500B653CA82273B7BFAD8045D85A470"))},
	},

	{
		"Keccak256()  0xcc",
		`Function TestRun(input String) String
		 30 return  KECCAK256(input)
         End Function`,
		"TestRun",
		map[string]interface{}{"input": string(decodeHex("41FB"))},
		nil,
		Variable{Type: String, ValueString: string(decodeHex("A8EACEDA4D47B3281A795AD9E1EA2122B407BAF9AABCB9E18B5717B7873537D2"))},
	},

	{
		"hex() ",
		`Function TestRun(input String) String
		 30 return  hex(input)
         End Function`,
		"TestRun",
		map[string]interface{}{"input": string(decodeHex("41FB"))},
		nil,
		Variable{Type: String, ValueString: string("41fb")},
	},

	{
		"hexdecode()",
		`Function TestRun(input String) String
		 30 return  hexdecode(input)
         End Function`,
		"TestRun",
		map[string]interface{}{"input": string("41FB")},
		nil,
		Variable{Type: String, ValueString: string(decodeHex("41FB"))},
	},
}

func decodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// run the test
func Test_FUNCTION_execution(t *testing.T) {
	for _, test := range execution_tests_functions {
		sc, _, err := ParseSmartContract(test.Code)
		if err != nil {
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected nil\nActual %s\n", test.Name, err)
		}

		state := &Shared_State{Chain_inputs: &Blockchain_Input{BL_HEIGHT: 5, BL_TIMESTAMP: 9, SCID: crypto.ZEROHASH,
			BLID: crypto.ZEROHASH, TXID: crypto.ZEROHASH}}
		result, err := RunSmartContract(&sc, test.EntryPoint, state, test.Args)
		switch {
		case test.Eerr == nil && err == nil:
			if !reflect.DeepEqual(result, test.result) {
				t.Fatalf("Invalid result while executing smart contract \"%s\"\nExpected result %v\nActual result %v\n", test.Name, test.result, result)
			}
		case test.Eerr != nil && err != nil: // pass
		case test.Eerr == nil && err != nil:
			fallthrough
		case test.Eerr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Eerr, err)
		}
	}
}
