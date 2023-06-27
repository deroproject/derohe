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
//import "reflect"
import (
	"testing"

	"github.com/deroproject/derohe/cryptography/crypto"
)

// run the test
func Test_RND_execution(t *testing.T) {

	var SCID, BLID, TXID crypto.Hash
	SCID[0] = 22
	rnd := Initialize_RND(SCID, BLID, TXID)

	//get a random number
	result_initial := rnd.Random()

	// lets tweak , each parameter by a byte and check whether it affects the output
	SCID[0] = 23
	if Initialize_RND(SCID, BLID, TXID).Random() == result_initial {
		t.Fatalf("RND not dependent on SCID")
	}
	SCID[0] = 0

	BLID[0] = 22
	if Initialize_RND(SCID, BLID, TXID).Random() == result_initial {
		t.Fatalf("RND not dependent on BLID")
	}
	BLID[0] = 0

	TXID[0] = 22
	if Initialize_RND(SCID, BLID, TXID).Random() == result_initial {
		t.Fatalf("RND not dependent on TXID")
	}
	TXID[0] = 22

	if Initialize_RND(SCID, BLID, TXID).Random_MAX(10000) >= 10000 {
		t.Fatalf("RND cannot be generated within a range")
	}

}
