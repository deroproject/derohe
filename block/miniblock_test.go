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

import "bytes"
import "testing"
import "crypto/rand"

func Test_blockmini_serde(t *testing.T) {

	var random_data [MINIBLOCK_SIZE]byte

	random_data[0] = 0x41
	var bl, bl2 MiniBlock

	if err := bl2.Deserialize(random_data[:]); err != nil {
		t.Fatalf("error during serdes %x err %s", random_data, err)
	}

	//t.Logf("bl2 %+v\n",bl2)
	//t.Logf("bl2 serialized %x\n",bl2.Serialize())

	if err := bl.Deserialize(bl2.Serialize()); err != nil {
		t.Fatalf("error during serdes %x", random_data)
	}

}

func Test_blockmini_serdes(t *testing.T) {
	for i := 0; i < 10000; i++ {

		var random_data [MINIBLOCK_SIZE]byte

		if _, err := rand.Read(random_data[:]); err != nil {
			t.Fatalf("error reading random number %s", err)
		}
		random_data[0] = 0x41

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
