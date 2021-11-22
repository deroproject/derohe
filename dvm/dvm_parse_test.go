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

import "testing"
import "fmt"

var evalList = []struct {
	Name string
	Code string
	Perr error // parse error
}{
	// some basics
	{
		"demo 1",
		`Function HelloWorld(s Uint64) Uint64
                 900 Return s
                 End Function
                 `,
		nil,
	}, {
		"function beginning skipped",
		`Functionq HelloWorld(s Uint64) Uint64
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	},
	{
		"Invalid function name",
		`Function 1Hello(s Uint64) Uint64
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Invalid argument variable name",
		`Function HelloWorld(1s Uint64) Uint64
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Invalid variable type",
		`Function HelloWorld(a1 Uint64, a2 String1,) Uint64
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Invalid line number minimum",
		`Function HelloWorld(s Uint64) Uint64
                 0 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Invalid line number maximum",
		`Function HelloWorld(s Uint64) Uint64
                 18446744073709551615 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Invalid line number duplicate",
		`Function HelloWorld(s Uint64) Uint64
                 10 Return s
                 10 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Invalid line number descending",
		`Function HelloWorld(s Uint64) Uint64
                 10 Return s
                 5 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Function End missing",
		`Function HelloWorld(s Uint64) Uint64
                 10 dim 1s as Uint64
                 20 Return s
                  Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Function Nesting now allowed",
		`Function HelloWorld(s Uint64) Uint64
                 10 dim 1s as Uint64
                 20 Return s
                 Function HelloWorld()
                 
                 End Function
                 `,
		fmt.Errorf("dummy"),
	},
	{
		"Function \" \" between arguments",
		`Function HelloWorld(s Uint64  s2 String ) Uint64
                 10 dim s as Uint64
                 20 Return s              
                 End Function
                 `,
		nil,
	},
	{
		"Function , between arguments",
		`Function HelloWorld(s Uint64 , s2 String ) Uint64
                 10 dim s as Uint64
                 20 Return s              
                 End Function
                 `,
		nil,
	},
	{
		"Missing end function",
		`Function HelloWorld(s Uint64) Uint64
                 900 Return s
                 
                 `,
		fmt.Errorf("dummy"),
	}, {
		"negative line number",
		`Function HelloWorld(s Uint64) Uint64
                 -900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"negative line number",
		`Function HelloWorld(s Uint64) Uint64
                 -900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"REM  line number",
		`Function HelloWorld(s Uint64) Uint64
                 REM  line number
                 900 Return s
                 End Function
                 `,
		nil,
	}, {
		"//  comments are skipped",
		`Function HelloWorld(s Uint64) Uint64
                 //  line number
                 900 Return s
                 End Function
                 `,
		nil,
	}, {
		"invalid function name",
		`Function ` + string([]byte{0x80, 0x80, 0x80, 0x80, 0x80}) + `HelloWorld(s Uint64) Uint64
                 //  line number
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"function name missin",
		`Function 
                 //  line number
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"function name missin (",
		`Function HelloWorld s Uint64) Uint64
                 //  line number
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"function name missin )",
		`Function HelloWorld (
                 //  line number
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"function name missin argument typ)",
		`Function HelloWorld ( s1 
                 //  line number
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"function name unknonw argument type",
		`Function HelloWorld ( s1 monkey ) 
                 //  line number
                 900 Return 
                 End Function
                 `,
		fmt.Errorf("dummy"),
	}, {
		"Invalid return type",
		`Function HelloWorld(s Uint64) Stream
                 900 Return s
                 End Function
                 `,
		fmt.Errorf("dummy"),
	},
}

// run the test
func TestEval(t *testing.T) {
	for _, test := range evalList {
		_, _, err := ParseSmartContract(test.Code)
		switch {
		case test.Perr == nil && err == nil: // pass
		case test.Perr != nil && err != nil: // pass
		case test.Perr == nil && err != nil:
			fallthrough
		case test.Perr != nil && err == nil:
			t.Fatalf("Error while parsing smart contract \"%s\"\nExpected %s\nActual %s\n", test.Name, test.Perr, err)
		}
	}
}
