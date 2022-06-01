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

package blockchain

import (
	"fmt"

	"github.com/stratumfarm/derohe/cryptography/crypto"
	"github.com/stratumfarm/derohe/transaction"
)

// used to verify complete block which contains expanded transaction
type cbl_verify struct {
	data map[crypto.Hash]map[[33]byte]uint64
}

// tx must be in expanded form
// check and insert cannot should be used 2 time, one time use for check, second time use for insert
func (b *cbl_verify) check(tx *transaction.Transaction, insert_for_future bool) (err error) {
	if tx.IsRegistration() || tx.IsCoinbase() || tx.IsPremine() { // these are not used
		return nil
	}
	if b.data == nil {
		b.data = map[crypto.Hash]map[[33]byte]uint64{}
	}

	height := tx.Height
	for _, p := range tx.Payloads {
		parity := p.Proof.Parity()
		if _, ok := b.data[p.SCID]; !ok { // this scid is being touched for first time, we are good to go
			if !insert_for_future { // if we are not inserting, skip this entire statment
				continue
			}
			b.data[p.SCID] = map[[33]byte]uint64{}
		}
		if p.Statement.RingSize != uint64(len(p.Statement.Publickeylist_compressed)) {
			return fmt.Errorf("TX is not expanded. cannot cbl_verify expected %d  Actual %d", p.Statement.RingSize, len(p.Statement.Publickeylist_compressed))
		}

		for j, pkc := range p.Statement.Publickeylist_compressed {
			if (j%2 == 0) == parity { // this condition is well thought out and works good enough
				if h, ok := b.data[p.SCID][pkc]; ok {
					if h != height {
						return fmt.Errorf("Not possible")
					}
				} else {
					if insert_for_future {
						b.data[p.SCID][pkc] = height
					}
				}

			}
		}
	}

	return nil
}
