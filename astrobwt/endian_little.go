//go:build amd64 || amd64p32 || 386 || arm || arm64 || mipsle || mips64le || mips64p32le || ppc64le || riscv || riscv64 || wasm || loong64
// +build amd64 amd64p32 386 arm arm64 mipsle mips64le mips64p32le ppc64le riscv riscv64 wasm loong64

package astrobwt

const LittleEndian = true
const BigEndian = false

//see https://github.com/golang/go/blob/master/src/go/build/syslist.go
