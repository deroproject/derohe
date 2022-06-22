//go:build armbe || arm64be || mips || mips64 || ppc64 || s390 || s390x || sparc || sparc64
// +build armbe arm64be mips mips64 ppc64 s390 s390x sparc sparc64

package astrobwtv3

const LittleEndian = false
const BigEndian = true

// see https://github.com/golang/go/blob/master/src/go/build/syslist.go

// this is NOT much faster and more efficient, however it won't work on appengine
// since it doesn't have the unsafe package.
// Also this would blow up silently if len(b) < 4.
func ReadBigUint32Unsafe(b []byte) uint32 {
	return (*(*uint32)(unsafe.Pointer(&b[0])))
}
