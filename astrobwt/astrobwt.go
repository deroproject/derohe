package astrobwt

import "fmt"
import "os"
import "runtime/debug"
import "crypto/rand"
import "encoding/binary"
import "golang.org/x/crypto/sha3"
import "golang.org/x/crypto/salsa20/salsa"

// NOTE: "unsafe" import removed — unsafe.Pointer cast was replaced with
// safe binary.LittleEndian.PutUint16 serialization.

// see here to improve the algorithms more https://github.com/y-256/libdivsufsort/blob/wiki/SACA_Benchmarks.md

var x = fmt.Sprintf

const stage1_length int = 9973 // it is a prime

func POW16(inputdata []byte) (outputhash [32]byte) {

	defer func() {
		if r := recover(); r != nil { // if something happens due to RAM issues in miner, we should continue, system will crash sooner or later
			fmt.Fprintf(os.Stderr, "[ASTROBWT] POW16 panic recovered: %v\n%s\n", r, debug.Stack())
			var buf [16]byte
			rand.Read(buf[:])
			outputhash = sha3.Sum256(buf[:]) // return a falsified hash which will fail the check
		}
	}()

	var counter [16]byte

	key := sha3.Sum256(inputdata)

	var stage1 [stage1_length]byte // stages are taken from it
	salsa.XORKeyStream(stage1[:stage1_length], stage1[:stage1_length], &counter, &key)

	var sa [stage1_length]int16
	text_16_0alloc(stage1[:], sa[:])

	// SECURITY: Always use safe serialization (binary.LittleEndian.PutUint16)
	// instead of unsafe.Pointer cast which is undefined behavior on some platforms.
	var s [stage1_length * 2]byte
	for i := range sa {
		binary.LittleEndian.PutUint16(s[i<<1:], uint16(sa[i]))
	}
	outputhash = sha3.Sum256(s[:])
	return
}

func text_16_0alloc(text []byte, sa []int16) {
	if int(int16(len(text))) != len(text) || len(text) != len(sa) {
		panic("suffixarray: misuse of text_16")
	}
	var memory [2 * 256]int16
	sais_8_16(text, 256, sa, memory[:])
}

func POW32(inputdata []byte) (outputhash [32]byte) {
	var sa16 [stage1_length]int16
	var counter [16]byte
	key := sha3.Sum256(inputdata)

	var stage1 [stage1_length]byte // stages are taken from it
	salsa.XORKeyStream(stage1[:stage1_length], stage1[:stage1_length], &counter, &key)
	var sa [stage1_length]int32
	text_32_0alloc(stage1[:], sa[:])

	for i := range sa {
		sa16[i] = int16(sa[i])
	}

	// SECURITY: Always use safe serialization (binary.LittleEndian.PutUint16)
	// instead of unsafe.Pointer cast which is undefined behavior on some platforms.
	var s [stage1_length * 2]byte
	for i := range sa16 {
		binary.LittleEndian.PutUint16(s[i<<1:], uint16(sa16[i]))
	}
	outputhash = sha3.Sum256(s[:])
	return
}

func text_32_0alloc(text []byte, sa []int32) {
	if int(int16(len(text))) != len(text) || len(text) != len(sa) {
		panic("suffixarray: misuse of text_16")
	}
	var memory [2 * 256]int32
	sais_8_32(text, 256, sa, memory[:])
}
