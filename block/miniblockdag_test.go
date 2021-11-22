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
import "crypto/rand"
import "encoding/binary"
import "github.com/deroproject/derohe/cryptography/crypto"

// tests whether the purge is working as it should
func Test_blockmini_purge(t *testing.T) {
	c := CreateMiniBlockCollection()

	for i := 0; i < 10; i++ {
		mbl := MiniBlock{Version: 1, Genesis: true, PastCount: 1}
		rand.Read(mbl.Nonce[:]) // fill with randomness
		binary.BigEndian.PutUint64(mbl.Check[:], uint64(i))
		if err, ok := c.InsertMiniBlock(mbl); !ok {
			t.Fatalf("error inserting miniblock err: %s", err)
		}
	}

	c.PurgeHeight(5) // purge all miniblock  <= height 5

	if len(c.Collection) != 4 {
		t.Fatalf("miniblocks not purged")
	}
	for _, v := range c.Collection {
		if v.Height <= 5 {
			t.Fatalf("purge not working correctly")
		}
	}
}

// tests whether collision is working correctly
// also tests whether genesis blocks returns connected always
func Test_blockmini_collision(t *testing.T) {
	c := CreateMiniBlockCollection()

	mbl := MiniBlock{Version: 1, Genesis: true, PastCount: 1}
	rand.Read(mbl.Nonce[:]) // fill with randomness
	binary.BigEndian.PutUint64(mbl.Check[:], uint64(8))

	if !c.IsConnected(mbl) { // even before inserting it should return connectd
		t.Fatalf("genesis blocks are already connected")
	}

	if err, ok := c.InsertMiniBlock(mbl); !ok {
		t.Fatalf("error inserting miniblock err: %s", err)
	}

	if !c.IsAlreadyInserted(mbl) {
		t.Fatalf("already inserted block not detected")
	}

	if c.IsAlreadyInserted(mbl) != c.IsCollision(mbl) {
		t.Fatalf("already inserted block not detected")
	}

	if !c.IsConnected(mbl) {
		t.Fatalf("genesis blocks are already connected")
	}
	if c.CalculateDistance(mbl) != 0 {
		t.Fatalf("genesis blocks should always be 0 distance")
	}
}

// tests whether the timestamp sorting is working as it should
//
func Test_blockmini_timestampsorting(t *testing.T) {

	for tries := 0; tries < 10000; tries++ {
		c := CreateMiniBlockCollection()

		total_count := 10
		for i := 0; i < total_count; i++ {
			mbl := MiniBlock{Version: 1, Genesis: true, PastCount: 1, Timestamp: uint64(256 - i)}
			//rand.Read(mbl.Nonce[:]) // fill with randomness
			binary.BigEndian.PutUint64(mbl.Check[:], uint64(i))
			if err, ok := c.InsertMiniBlock(mbl); !ok {
				t.Fatalf("error inserting miniblock err: %s", err)
			}

			//t.Logf("presorted %+v", mbl)
		}

		all_genesis := c.GetAllGenesisMiniBlocks()

		if len(all_genesis) != total_count {
			panic("corruption")
		}

		sorted_all_genesis := MiniBlocks_SortByTimeAsc(all_genesis)

		for i := 0; i < len(sorted_all_genesis)-1; i++ {
			//t.Logf("sorted %+v", sorted_all_genesis[i])
			if sorted_all_genesis[i].Timestamp > sorted_all_genesis[i+1].Timestamp {
				t.Fatalf("sorting of Timestamp failed")
			}
		}

		// insert a miniblock which has timestamp collision
		{
			mbl := MiniBlock{Version: 1, Genesis: true, PastCount: 1, Timestamp: uint64(254)}
			binary.BigEndian.PutUint64(mbl.Check[:], uint64(9900))
			if err, ok := c.InsertMiniBlock(mbl); !ok {
				t.Fatalf("error inserting miniblock err: %s", err)
			}

		}

		all_genesis = c.GetAllGenesisMiniBlocks()
		if len(all_genesis) != (total_count + 1) {
			panic("corruption")
		}

		sorted_all_genesis = MiniBlocks_SortByTimeAsc(all_genesis)
		for i := 0; i < len(sorted_all_genesis)-1; i++ {
			//t.Logf("sorted %d %+v", sorted_all_genesis[i].GetMiniID(),sorted_all_genesis[i+1])
			if sorted_all_genesis[i].Timestamp > sorted_all_genesis[i+1].Timestamp {
				t.Fatalf("sorting of Timestamp failed")
			}
		}

		if sorted_all_genesis[total_count-2].Height != 9900 { /// this element will be moved to this, if everything is current
			t.Fatalf("test failed  %+v", sorted_all_genesis[total_count-2])
		}
	}

}

// tests whether the distance sorting is working as it should
func Test_blockmini_distancesorting(t *testing.T) {
	var unsorted []MiniBlock

	total_count := 10
	for i := 0; i < total_count; i++ {
		mbl := MiniBlock{Version: 1, Genesis: true, PastCount: 1, Timestamp: uint64(256 - i), Distance: uint32(256 - i)}
		binary.BigEndian.PutUint64(mbl.Check[:], uint64(i))
		unsorted = append(unsorted, mbl)
	}

	// insert a miniblock which has timestamp and distance collision
	{
		mbl := MiniBlock{Version: 1, Genesis: true, PastCount: 1, Timestamp: uint64(254), Distance: uint32(254)}
		mbl.Height = int64(9900)
		unsorted = append(unsorted, mbl)
	}

	sorted_d := MiniBlocks_SortByDistanceDesc(unsorted)
	for i := 0; i < len(sorted_d)-1; i++ {
		//		t.Logf("sorted %d %d %+v", i, sorted_d[i].GetMiniID(),sorted_d[i])
		if sorted_d[i].Distance < sorted_d[i+1].Distance {
			t.Fatalf("sorting of Distance failed")
		}
	}

	if sorted_d[3].Height != 9900 { /// this element will be moved to this, if everything is current
		t.Fatalf("test failed  %+v", sorted_d[3])
	}
}

// tests whether filtering is working as it should
func Test_MiniBlocks_Filter(t *testing.T) {
	var mbls []MiniBlock

	total_count := uint32(10)
	for i := uint32(0); i < total_count; i++ {
		mbl := MiniBlock{Version: 1, Genesis: true}

		if i%2 == 0 {
			mbl.PastCount = 1
			mbl.Past[0] = i
		} else {
			mbl.PastCount = 2
			mbl.Past[0] = i
			mbl.Past[1] = i + 1
		}
		binary.BigEndian.PutUint64(mbl.Check[:], uint64(i))
		mbls = append(mbls, mbl)
	}

	for i := uint32(0); i < total_count; i++ {
		if i%2 == 0 {
			result := MiniBlocks_Filter(mbls, []uint32{i})
			if len(result) != 1 {
				t.Fatalf("failed filter")
			}
			if result[0].PastCount != 1 || result[0].Past[0] != i {
				t.Fatalf("failed filter")
			}

		} else {
			result := MiniBlocks_Filter(mbls, []uint32{i, i + 1})
			if len(result) != 1 {
				t.Fatalf("failed filter")
			}
			if result[0].PastCount != 2 || result[0].Past[0] != i || result[0].Past[1] != i+1 {
				t.Fatalf("failed filter")
			}
		}
	}

	for i := uint32(0); i < total_count; i++ {
		if i%2 == 0 {
			var tips [1]crypto.Hash
			binary.BigEndian.PutUint32(tips[0][:], i)
			result := MiniBlocks_FilterOnlyGenesis(mbls, tips[:])
			if len(result) != 1 {
				t.Fatalf("failed filter")
			}
			if result[0].PastCount != 1 || result[0].Past[0] != i {
				t.Fatalf("failed filter")
			}

		} else {
			var tips [2]crypto.Hash
			binary.BigEndian.PutUint32(tips[0][:], i)
			binary.BigEndian.PutUint32(tips[1][:], i+1)
			result := MiniBlocks_FilterOnlyGenesis(mbls, tips[:])
			if len(result) != 1 {
				t.Fatalf("failed filter")
			}
			if result[0].PastCount != 2 || result[0].Past[0] != i || result[0].Past[1] != i+1 {
				t.Fatalf("failed filter")
			}
		}
	}

}
