//go:build amd64 || amd64p32 || 386 || arm || arm64 || mipsle || mips64le || mips64p32le || ppc64le || riscv || riscv64 || wasm || loong64
// +build amd64 amd64p32 386 arm arm64 mipsle mips64le mips64p32le ppc64le riscv riscv64 wasm loong64

package astrobwtv3

import "unsafe"
import "math/bits"

const LittleEndian = true
const BigEndian = false

//see https://github.com/golang/go/blob/master/src/go/build/syslist.go

// this is NOT much faster and efficient, however it won't work on appengine
// since it doesn't have the unsafe package.
// Also this would blow up silently if len(b) < 4.
func ReadBigUint32Unsafe(b []byte) uint32 {
	return bits.ReverseBytes32(*(*uint32)(unsafe.Pointer(&b[0])))
}
