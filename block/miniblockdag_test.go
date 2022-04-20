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

//import "bytes"
import "testing"

// tests whether the purge is working as it should
func Test_blockmini_purge(t *testing.T) {
	c := CreateMiniBlockCollection()

	for i := 0; i < 10; i++ {
		mbl := MiniBlock{Version: 1, Height: uint64(i), PastCount: 1}
		if err, ok := c.InsertMiniBlock(mbl); !ok {
			t.Fatalf("error inserting miniblock err: %s", err)
		}
	}

	c.PurgeHeight(nil, 5) // purge all miniblock  <= height 5

	if c.Count() != 4 {
		t.Fatalf("miniblocks not purged")
	}
	for _, mbls := range c.Collection {
		for _, mbl := range mbls {
			if mbl.Height <= 5 {
				t.Fatalf("purge not working correctly")
			}
		}
	}
}

// tests whether collision is working correctly
// also tests whether genesis blocks returns connected always
func Test_blockmini_collision(t *testing.T) {
	c := CreateMiniBlockCollection()

	mbl := MiniBlock{Version: 1, PastCount: 1}

	if err, ok := c.InsertMiniBlock(mbl); !ok {
		t.Fatalf("error inserting miniblock err: %s", err)
	}

	if !c.IsAlreadyInserted(mbl) {
		t.Fatalf("already inserted block not detected")
	}

	if c.IsAlreadyInserted(mbl) != c.IsCollision(mbl) {
		t.Fatalf("already inserted block not detected")
	}
}
