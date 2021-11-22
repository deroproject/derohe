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

package crypto

import "fmt"
import "encoding/hex"

const KeyLength = 32

// Key can be a Scalar or a Point
type Key [KeyLength]byte

func (k Key) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%x", k[:])), nil
}

func (k *Key) UnmarshalText(data []byte) (err error) {
	byteSlice, _ := hex.DecodeString(string(data))
	if len(byteSlice) != 32 {
		return fmt.Errorf("Incorrect key size")
	}
	copy(k[:], byteSlice)
	return
}

func (k Key) String() string {
	return fmt.Sprintf("%x", k[:])
}

func (p *Key) FromBytes(b [KeyLength]byte) {
	*p = b
}

func (p *Key) ToBytes() (result [KeyLength]byte) {
	result = [KeyLength]byte(*p)
	return
}

// convert a hex string to a key
func HexToKey(h string) (result Key) {
	byteSlice, _ := hex.DecodeString(h)
	if len(byteSlice) != 32 {
		panic("Incorrect key size")
	}
	copy(result[:], byteSlice)
	return
}

func HexToHash(h string) (result Hash) {
	byteSlice, _ := hex.DecodeString(h)
	if len(byteSlice) != 32 {
		panic("Incorrect key size")
	}
	copy(result[:], byteSlice)
	return
}
