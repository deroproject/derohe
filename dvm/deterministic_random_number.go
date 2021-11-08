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

import "encoding/binary"
import "golang.org/x/crypto/salsa20/salsa"

import "github.com/deroproject/derohe/cryptography/crypto"

/* this file implements a deterministic random number generator
   the random number space is quite large but still unattackable, since the seeds are random
   the seeds depend on the BLID  and the TXID, if an attacker can somehow control the TXID,
   he will not be able to control BLID
*/

type RND struct {
	Key [32]byte
	Pos uint64 // we will wrap around in 2^64 times but this is per TX,BLOCK,SCID
}

// make sure 2 SCs cannot  ever generate same series of random numbers
func Initialize_RND(SCID, BLID, TXID crypto.Hash) (r *RND) {
	r = &RND{}
	tmp := crypto.Keccak256(SCID[:], BLID[:], TXID[:])
	copy(r.Key[:], tmp[:])
	r.Pos = 1 // we start at 1 to eliminate an edge case
	return    // TODO we must reinitialize using blid and other parameters
}

func (r *RND) Random() uint64 {

	var out [32]byte
	var in [16]byte
	var key [32]byte

	copy(key[:], r.Key[:])

	binary.BigEndian.PutUint64(in[:], r.Pos)

	salsa.HSalsa20(&out, &in, &key, &in)

	deterministic_value := binary.BigEndian.Uint64(out[:])
	r.Pos++

	return deterministic_value
}

// range to (input-1)
func (r *RND) Random_MAX(input uint64) uint64 {
	if input == 0 {
		panic("RNG cannot generate RND with 0 as max")
	}
	return r.Random() % input
}
