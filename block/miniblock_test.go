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

package block

import "time"
import "bytes"
import "testing"
import "crypto/rand"

func Test_blockmini_serde(t *testing.T) {

	var random_data [MINIBLOCK_SIZE]byte

	random_data[0] = 0xa1
	for i := byte(1); i < 32; i++ {
		random_data[i] = 0
	}

	var bl, bl2 MiniBlock

	if err := bl2.Deserialize(random_data[:]); err != nil {
		t.Fatalf("error during serdes %x err %s", random_data, err)
	}

	//t.Logf("bl2 %+v\n",bl2)
	//t.Logf("bl2 serialized %x\n",bl2.Serialize())

	if err := bl.Deserialize(bl2.Serialize()); err != nil {
		t.Fatalf("error during serdes %x", random_data)
	}

	//t.Logf("bl1 %+v\n",bl)

}

func Test_blockmini_serdes(t *testing.T) {
	for i := 0; i < 10000; i++ {

		var random_data [MINIBLOCK_SIZE]byte

		if _, err := rand.Read(random_data[:]); err != nil {
			t.Fatalf("error reading random number %s", err)
		}
		random_data[0] = 0xa1

		var bl, bl2 MiniBlock

		if err := bl2.Deserialize(random_data[:]); err != nil {
			t.Fatalf("error during serdes %x", random_data)
		}

		if err := bl.Deserialize(bl2.Serialize()); err != nil {
			t.Fatalf("error during serdes %x", random_data)
		}

		if bl.GetHash() != bl2.GetHash() {
			t.Fatalf("error during serdes %x", random_data)
		}

		if !bytes.Equal(bl.Serialize(), bl2.Serialize()) {
			t.Fatalf("error during serdes %x", random_data)
		}

	}

}

// test all invalid edge cases, which will return error
func Test_Representable_Time(t *testing.T) {

	var bl, bl2 MiniBlock
	bl.Version = 1
	bl.PastCount = 2
	bl.Timestamp = 0xffffffffffff
	serialized := bl.Serialize()

	if err := bl2.Deserialize(serialized); err != nil {
		t.Fatalf("error during serdes")
	}

	if bl.Timestamp != bl2.Timestamp {
		t.Fatalf("timestamp corruption")
	}

	timestamp := time.Unix(0, int64(bl.Timestamp*uint64(time.Millisecond)))

	if timestamp.Year() != 2121 {
		t.Fatalf("corruption in timestamp representing year 2121")
	}

	t.Logf("time representable is %s\n", timestamp.UTC())
}
