// Copyright 2017-2021 DERO Project. All rights reserved.
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

package walletapi

import "runtime"
import "fmt"
import "sort"
import "math/big"
import "encoding/binary"

//import "github.com/mattn/go-isatty"
//import "github.com/cheggaaa/pb/v3"

import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/cryptography/bn256"

// this file implements balance decoder whih has to be bruteforced
// balance is a 64 bit field and total effort is 2^64
// but in reality, the balances are well distributed with the expectation that no one will ever be able to collect over 2 ^ 40
// However, 2^40 is solvable in less than 1 sec on a single core, with around 8 MB RAM, see below
// these tables are sharable between wallets

//type PreComputeTable [16*1024*1024]uint64 // each table is 128 MB in size

// table size cannot be more than 1<<24
const TABLE_SIZE = 1 << 19

type PreComputeTable []uint64 // each table is 2^TABLE_SIZE * 8  bytes  in size
// 2^15 * 8 = 32768 *8 = 256 Kib
// 2^16 * 8 =            512 Kib
// 2^17 * 8 =              1 Mib
// 2^18 * 8 =              2 Mib
// 2^19 * 8 =              4 Mib
// 2^20 * 8 =              8 Mib

type LookupTable []PreComputeTable // default is 1 table, if someone who owns 2^48 coins or more needs more speed, it is possible

// IntSlice attaches the methods of Interface to []int, sorting in increasing order.

//type UintSlice []uint64

func (p PreComputeTable) Len() int           { return len(p) }
func (p PreComputeTable) Less(i, j int) bool { return p[i] < p[j] }
func (p PreComputeTable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

var precompute_table_ready = make(chan int)

// with some more smartness table can be condensed more to contain 16.3% more entries within the same size
func Initialize_LookupTable(count int, table_size int) *LookupTable {
	t := make([]PreComputeTable, count, count)

	if table_size&0xff != 0 {
		panic("table size must be multiple of 256")
	}

	//terminal := isatty.IsTerminal(os.Stdout.Fd())

	var acc bn256.G1 // avoid allocations every loop
	acc.ScalarMult(crypto.G, new(big.Int).SetUint64(0))

	small_table := make([]*bn256.G1, 256, 256)
	for k := range small_table {
		small_table[k] = new(bn256.G1)
	}

	var compressed [33]byte

	for i := range t {
		t[i] = make([]uint64, table_size, table_size)

		//bar := pb.New(table_size)
		//if terminal {
		//	bar.Start()
		//}
		for j := 0; j < table_size; j += 256 {

			for k := range small_table {
				small_table[k].Set(&acc)
				acc.Add(small_table[k], crypto.G)
			}
			(bn256.G1Array(small_table)).MakeAffine() // precompute everything ASAP

			//if terminal {
			//	bar.Add(256)
			//}

			for k := range small_table {
				// convert acc to compressed point and extract last 5 bytes
				//compressed := small_table[k].EncodeCompressed()
				small_table[k].EncodeCompressedToBuf(compressed[:])

				// replace last bytes by j in coded form
				compressed[32] = byte(uint64(j+k) & 0xff)
				compressed[31] = byte((uint64(j+k) >> 8) & 0xff)
				compressed[30] = byte((uint64(j+k) >> 16) & 0xff)

				(t)[i][j+k] = binary.BigEndian.Uint64(compressed[25:])
			}

			//  if j < 300  {
			//      fmt.Printf("%d[%d]th entry %x\n",i,j, (t)[i][j])
			// }

			if j%1000 == 0 && runtime.GOOS == "js" {
				fmt.Printf("completed %f (j %d)\n", float32(j)*100/float32(len((t)[i])), j)
				runtime.Gosched() // gives others opportunity to run
			}
		}

		//fmt.Printf("sorting start\n")
		sort.Sort(t[i])
		//fmt.Printf("sortingcomplete\n")
		//bar.Finish()
	}
	//fmt.Printf("lookuptable complete\n")
	t1 := LookupTable(t)

	Balance_lookup_table = &t1
	close(precompute_table_ready)
	return &t1
}

// convert point to balance
func (t *LookupTable) Lookup(p *bn256.G1, previous_balance uint64) (balance uint64) {

	// now this big part must be searched in the precomputation lookup table

	//fmt.Printf("decoding balance now\n",)
	var acc bn256.G1

	<-precompute_table_ready // wait till precompute table is ready
	// check if previous balance is still sane though it may have mutated

	acc.ScalarMult(crypto.G, new(big.Int).SetUint64(previous_balance))
	if acc.String() == p.String() {
		return previous_balance

	}

	work_per_loop := new(bn256.G1)

	balance_part := uint64(0)

	balance_per_loop := uint64(len((*t)[0]) * len(*t))

	_ = balance_per_loop

	pcopy := new(bn256.G1).Set(p)

	work_per_loop.ScalarMult(crypto.G, new(big.Int).SetUint64(balance_per_loop))
	work_per_loop = new(bn256.G1).Neg(work_per_loop)

	loop_counter := 0

	//  fmt.Printf("jumping into loop %d\n", loop_counter)
	for { // it is an infinite loop

		// fmt.Printf("loop counter %d  balance %d\n", loop_counter, balance)

		if loop_counter != 0 {
			pcopy = new(bn256.G1).Add(pcopy, work_per_loop)
		}
		loop_counter++

		//if loop_counter >= 10 {
		//    break;
		// }

		compressed := pcopy.EncodeCompressed()

		compressed[32] = 0
		compressed[31] = 0
		compressed[30] = 0

		big_part := binary.BigEndian.Uint64(compressed[25:])

		for i := range *t {
			index := sort.Search(len((*t)[i]), func(j int) bool { return ((*t)[i][j] & 0xffffffffff000000) >= big_part })
		check_again:
			if index < len((*t)[i]) && ((*t)[i][index]&0xffffffffff000000) == big_part {

				balance_part = ((*t)[i][index]) & 0xffffff
				acc.ScalarMult(crypto.G, new(big.Int).SetUint64(balance+balance_part))

				if acc.String() == p.String() { // since we may have part collisions, make sure full point is checked

					balance += balance_part
					// fmt.Printf("balance found  %d part(%d) index %d   big part %x\n",balance,balance_part, index, big_part );

					return balance

				}

				// we have failed since it was partial collision, make sure that we can try if possible, another collision
				// this code can be removed if no duplications exist in the first 5 bytes, but a probablity exists for the same
				index++
				goto check_again

			} else {
				// x is not present in data,
				// but i is the index where it would be inserted.

				balance += uint64(len((*t)[i]))
			}

		}

		// from the point we must decrease balance per loop

	}

	panic(fmt.Sprintf("balance not yet found, work done  %x", balance))
	return balance
}
