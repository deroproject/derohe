package astrobwt

import "fmt"
import "unsafe"
import "crypto/rand"
import "encoding/binary"
import "golang.org/x/crypto/sha3"
import "golang.org/x/crypto/salsa20/salsa"

// see here to improve the algorithms more https://github.com/y-256/libdivsufsort/blob/wiki/SACA_Benchmarks.md

var x = fmt.Sprintf

const stage1_length int = 9973 // it is a prime

func POW16(inputdata []byte) (outputhash [32]byte) {

	defer func() {
		if r := recover(); r != nil { // if something happens due to RAM issues in miner, we should continue, system will crash sooner or later
			var buf [16]byte
			rand.Read(buf[:])
			outputhash = sha3.Sum256(buf[:]) // return a falsified has which will fail the check
		}
	}()

	var counter [16]byte

	key := sha3.Sum256(inputdata)

	var stage1 [stage1_length]byte // stages are taken from it
	salsa.XORKeyStream(stage1[:stage1_length], stage1[:stage1_length], &counter, &key)

	var sa [stage1_length]int16
	text_16_0alloc(stage1[:], sa[:])

	if LittleEndian {
		var s *[stage1_length * 2]byte = (*[stage1_length * 2]byte)(unsafe.Pointer(&sa))
		outputhash = sha3.Sum256(s[:])
		return
	} else {
		var s [stage1_length * 2]byte
		for i := range sa {
			binary.LittleEndian.PutUint16(s[i<<1:], uint16(sa[i]))
		}
		outputhash = sha3.Sum256(s[:])
		return
	}
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

	if LittleEndian {
		var s *[stage1_length * 2]byte = (*[stage1_length * 2]byte)(unsafe.Pointer(&sa16))
		outputhash = sha3.Sum256(s[:])
		return
	} else {
		var s [stage1_length * 2]byte
		for i := range sa {
			binary.LittleEndian.PutUint16(s[i<<1:], uint16(sa[i]))
		}
		outputhash = sha3.Sum256(s[:])
		return
	}

	return
}

func text_32_0alloc(text []byte, sa []int32) {
	if int(int16(len(text))) != len(text) || len(text) != len(sa) {
		panic("suffixarray: misuse of text_16")
	}
	var memory [2 * 256]int32
	sais_8_32(text, 256, sa, memory[:])
}
