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

package main

import "runtime"
import "sync/atomic"
import "golang.org/x/sys/unix"

var processor int32

// sets thread affinity to avoid cache collision and thread migration
func threadaffinity() {
	var cpuset unix.CPUSet

	lock_on_cpu := atomic.AddInt32(&processor, 1)
	if lock_on_cpu >= int32(runtime.GOMAXPROCS(0)) { // threads are more than cpu, we do not know what to do
		return
	}
	cpuset.Zero()
	cpuset.Set(int(avoidHT(int(lock_on_cpu))))

	unix.SchedSetaffinity(0, &cpuset)
}

func avoidHT(i int) int {
	count := runtime.GOMAXPROCS(0)
	if i < count/2 {
		return i * 2
	} else {
		return (i-count/2)*2 + 1
	}
}
