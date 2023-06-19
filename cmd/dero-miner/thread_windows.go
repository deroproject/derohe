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

import (
	"math/bits"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var libkernel32 uintptr
var setThreadAffinityMask uintptr

func doLoadLibrary(name string) uintptr {
	lib, _ := syscall.LoadLibrary(name)
	return uintptr(lib)
}

func doGetProcAddress(lib uintptr, name string) uintptr {
	addr, _ := syscall.GetProcAddress(syscall.Handle(lib), name)
	return uintptr(addr)
}

func syscall3(trap, nargs, a1, a2, a3 uintptr) uintptr {
	ret, _, _ := syscall.Syscall(trap, nargs, a1, a2, a3)
	return ret
}

func init() {
	libkernel32 = doLoadLibrary("kernel32.dll")
	setThreadAffinityMask = doGetProcAddress(libkernel32, "SetThreadAffinityMask")
}

var processor int32

// currently we suppport upto 64 cores
func SetThreadAffinityMask(hThread syscall.Handle, dwThreadAffinityMask uint) *uint32 {
	ret1 := syscall3(setThreadAffinityMask, 2,
		uintptr(hThread),
		uintptr(dwThreadAffinityMask),
		0)
	return (*uint32)(unsafe.Pointer(ret1))
}

// CurrentThread returns the handle for the current thread.
// It is a pseudo handle that does not need to be closed.
func CurrentThread() syscall.Handle { return syscall.Handle(^uintptr(2 - 1)) }

// sets thread affinity to avoid cache collision and thread migration
func threadaffinity() {
	lock_on_cpu := atomic.AddInt32(&processor, 1)
	if lock_on_cpu >= int32(runtime.GOMAXPROCS(0)) { // threads are more than cpu, we do not know what to do
		return
	}

	if lock_on_cpu >= bits.UintSize {
		return
	}
	var cpuset uint
	cpuset = 1 << uint(avoidHT(int(lock_on_cpu)))
	SetThreadAffinityMask(CurrentThread(), cpuset)
}

func avoidHT(i int) int {
	count := runtime.GOMAXPROCS(0)
	if i < count/2 {
		return i * 2
	} else {
		return (i-count/2)*2 + 1
	}
}
