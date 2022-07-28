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
import "github.com/holiman/uint256"

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
		"valid  function  testing  MAP*() with 256bit key",
		`Function TestRun(a1 Uint64, a2 Uint256) Uint64
		 20 DIM v1, v2 AS Uint256
		 30 MAPSTORE(a2, a1)
		 40 LET v1 = a2
		 50 IF MAPEXISTS(v1) THEN GOTO 100
		 60 RETURN 1
		 100 IF MAPGET(v1) == a1 THEN GOTO 200
		 110 RETUN 1
		 200 DIM str AS String
		 210 LET str = ITOA(v1)
		 230 LET v2 = UINT256(str)
		 240 IF MAPGET(v2) == a1 THEN GOTO 300
		 250 RETURN 1
		 300 MAPDELETE(v2)
		 310 IF !MAPEXISTS(v1) THEN GOTO 500
		 320 RETURN 1
		 500 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "987654321", "a2": "987654320"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(99)},
	},
	{
		"valid  function  testing  MAP*() with 64bit key",
		`Function TestRun(a1 Uint64, a2 Uint256) Uint64
		 20 DIM v1, v2 AS Uint64
		 25 DIM v3 as Uint256
		 30 MAPSTORE(a1, a2)
		 40 LET v1 = a1
		 50 IF MAPEXISTS(v1) THEN GOTO 100
		 60 RETURN 1
		 100 IF MAPGET(v1) == a2 THEN GOTO 200
		 110 RETUN 1
		 200 DIM str AS String
		 210 LET str = ITOA(a2)
		 230 LET v3 = UINT256(str)
		 240 IF MAPGET(v1) == v3 THEN GOTO 300
		 250 RETURN 1
		 300 MAPDELETE(v1)
		 310 IF !MAPEXISTS(v1) THEN GOTO 500
		 320 RETURN 1
		 500 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "987654321", "a2": "987654320"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(99)},
	},
	{
		"valid  function  testing  MAX() ",
		`Function TestRun(a1 Uint64, a2 Uint256) Uint64
		 10 IF MAX(a1, a2) == a1 THEN GOTO 50
		 20 RETURN 1
		 50 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "987654321", "a2": "987654320"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(99)},
	},
	{
		"valid  function  testing  MIN() ",
		`Function TestRun(a1 Uint64, a2 Uint256) Uint64
		 10 IF MIN(a1, a2) == a2 THEN GOTO 50
		 20 RETURN 1
		 50 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "987654321", "a2": "987654320"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(99)},
	},
	{
		"valid  function  testing  ITOA() ",
		`Function TestRun(a1 Uint64, a2 Uint256) Uint64
		 10 IF ITOA(a1) == "987654321" THEN GOTO 50
		 20 RETURN 1
		 50 IF ITOA(a2) == "0x3ade68b1" THEN GOTO 90
		 60 RETURN 1
		 90 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "987654321", "a2": "987654321"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(99)},
	},
	{
		"valid  function  testing  UINT64() ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 dim s3, s4 as Uint256
		 30 LET s1 = 987654321
		 40 LET s3 = 987654321
		 50 IF s1 == UINT64(s3) THEN GOTO 100
		 60 return 0
		 100 LET s2 = UINT64("987654321")
		 110 IF s2*s2 == s3*UINT64(s3) THEN GOTO 200
		 120 RETURN 1
		 200 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(99)},
	},
	{
		"valid  function  testing  UINT256() ",
		`Function TestRun(a1 Uint64,a2 Uint64) Uint64
		 10 dim s1, s2 as Uint64
		 20 dim s3, s4 as Uint256
		 30 LET s1 = 123456
		 40 LET s3 = UINT256("123456")
		 50 IF s1 == s3 THEN GOTO 100
		 60 return 0
		 100 LET s2 = UINT256("0xffffffffffffffff")
		 110 IF (UINT256(s1)*s2) == (UINT256(123456) * UINT256("0xffffffffffffffff")) THEN GOTO 200
		 120 RETURN 1
		 200 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1", "a2": "1"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(99)},
	},
	{
		"valid  function  testing  SQRT(Uint64) ",
		`Function TestRun(a1 Uint64) Uint64
		 10 RETURN SQRT(a1)
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "1046529"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(1023)},
	},
	{
		"valid  function  testing  SQRT(Uint256) ",
		`Function TestRun(a1 Uint256) Uint256
		 10 DIM square AS Uint256
		 20 LET square = a1 * a1
		 30 IF SQRT(square) == a1 THEN GOTO 100
		 40 RETURN 1
		 100 RETURN 99
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "109876543210"},
		nil,
		Variable{Type: Uint256, ValueUint256: *uint256.NewInt(99)},
	},
	{
		"valid  function  testing  POW(Uint64) ",
		`Function TestRun(a1 Uint64, a2 Uint64) Uint64
		 10 RETURN POW(a1, a2)
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "33"},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(8589934592)},
	},
	{
		"valid  function  testing  POW(Uint256) ",
		`Function TestRun(a1 Uint256, a2 Uint256) Uint256
		 10 RETURN POW(a1, a2)
                 End Function
                 `,
		"TestRun",
		map[string]interface{}{"a1": "2", "a2": "255"},
		nil,
		Variable{Type: Uint256, ValueUint256: *(uint256.NewInt(0).Exp(uint256.NewInt(2), uint256.NewInt(255)))},
	},
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
	{
		"substr()",
		`Function TestRun(input String) String
		 30 return  substr(input,0,5)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: String, ValueString: string("01234")},
	},
	{
		"substr()",
		`Function TestRun(input String) String
		 30 return  substr(input,1,5)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: String, ValueString: string("12345")},
	},
	{
		"substr()",
		`Function TestRun(input String) String
		 30 return  substr(input,1,129)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: String, ValueString: string("123456789")},
	},
	{
		"substr()",
		`Function TestRun(input String) String
		 30 return  substr(input,13,129)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: String, ValueString: string("")},
	},
	{
		"tolower()",
		`Function TestRun(input String) String
		 30 return  tolower(input)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("A0b1C2d3E5f6")},
		nil,
		Variable{Type: String, ValueString: string("a0b1c2d3e5f6")},
	},
	{
		"toupper()",
		`Function TestRun(input String) String
		 30 return  toupper(input)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("A0b1C2d3E5f6")},
		nil,
		Variable{Type: String, ValueString: string("A0B1C2D3E5F6")},
	},
	{
		"subfield()",
		`Function TestRun(input String) String
		 30 return  subfield(input, ":", 3)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("This::is:a:test")},
		nil,
		Variable{Type: String, ValueString: string("a")},
	},
	{
		"subfield()",
		`Function TestRun(input String) String
		 30 return  subfield(input, ":", 5)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("This::is:a:test")},
		nil,
		Variable{Type: String, ValueString: string("")},
	},
	{
		"subfield()",
		`Function TestRun(input String) String
		 30 return  subfield(input, ":", 1)
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("This::is:a:test")},
		nil,
		Variable{Type: String, ValueString: string("")},
	},
	{
		"mapget()",
		`Function TestRun(input String) String
		 10 mapstore("input",input)
		 30 return  mapget("input") + mapget("input")
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: String, ValueString: string("01234567890123456789")},
	},
	{
		"mapget()",
		`Function TestRun(input String) String
		 10 mapstore("input",input)
		 15 mapstore("input",input+input)
		 30 return  mapget("input")
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: String, ValueString: string("01234567890123456789")},
	},

	{
		"mapexists()",
		`Function TestRun(input String) Uint64
		 10 mapstore("input",input)
		 30 return  mapexists("input") + mapexists("input1")
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(1)},
	},
	{
		"mapdelete()",
		`Function TestRun(input String) Uint64
		 10 mapstore("input",input)
		 15 mapdelete("input")
		 30 return  mapexists("input")
         	 End Function`,
		"TestRun",
		map[string]interface{}{"input": string("0123456789")},
		nil,
		Variable{Type: Uint64, ValueUint64: uint64(0)},
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
			BLID: crypto.ZEROHASH, TXID: crypto.ZEROHASH}, RamStore: map[Variable]Variable{}}
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
