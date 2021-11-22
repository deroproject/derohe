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

const POINT_SIZE = 33        // this can be optimized to 33 bytes
const FIELDELEMENT_SIZE = 32 // why not have bigger curves

// protocol supports amounts upto this amounts
const MAX_AMOUNT = 18446744073709551616 // 2^64 - 1,

const PROTOCOL_CONSTANT = "DERO"

// checks a number is power of 2
func IsPowerOf2(num int) bool {
	for num >= 2 {
		if num%2 != 0 {
			return false
		}
		num = num / 2
	}
	return num == 1
}

// tell what power a number is
func GetPowerof2(num int) int {

	if num <= 0 {
		panic("number cannot be less than 0")
	}

	if !IsPowerOf2(num) {
		panic(fmt.Sprintf("number(%d) must be power of 2", num))
	}

	power := 0
	calculated := 1
	for ; calculated != num; power++ {
		calculated *= 2
	}
	return power
}
