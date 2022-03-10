//go:build armbe || arm64be || mips || mips64 || ppc64 || s390 || s390x || sparc || sparc64
// +build armbe arm64be mips mips64 ppc64 s390 s390x sparc sparc64

package astrobwt_fast

const LittleEndian = false
const BigEndian = true

// see https://github.com/golang/go/blob/master/src/go/build/syslist.go
