package astrobwtv3

import "unsafe"
import "hash"
import "sync"

//import "fmt"
import "encoding/binary"
import "github.com/minio/sha256-simd"

const MAX_LENGTH uint32 = (256 * 384) - 1 // this is the maximum

// see here to improve the algorithms more https://github.com/y-256/libdivsufsort/blob/wiki/SACA_Benchmarks.md
// this optimized algorithm is used only  in the miner and not in the blockchain

type ScratchData struct {
	hasher              hash.Hash
	data                [MAX_LENGTH + 64]uint8
	stage1_result       *[MAX_LENGTH + 1]uint16
	stage1_result_bytes *[(MAX_LENGTH) * 2]uint8
	indices             [MAX_LENGTH + 1]uint32 // 256 KB
	tmp_indices         [MAX_LENGTH + 1]uint32 // 256 KB
	sa                  [MAX_LENGTH]int32
	sa_bytes            *[(MAX_LENGTH) * 4]uint8
}

var Pool = sync.Pool{New: func() interface{} {
	var d ScratchData
	d.hasher = sha256.New()
	d.stage1_result = ((*[MAX_LENGTH + 1]uint16)(unsafe.Pointer(&d.indices[0])))
	d.stage1_result_bytes = ((*[(MAX_LENGTH) * 2]byte)(unsafe.Pointer(&d.indices[0])))
	d.sa_bytes = ((*[(MAX_LENGTH) * 4]byte)(unsafe.Pointer(&d.sa[0])))

	return &d
}}

func fix(v []byte, indices []uint32, i int) {
	prev_t := indices[i]
	t := indices[i+1]

	// ReadBigUint32Unsafe  can be replaced with this   binary.BigEndian.Uint32
	data_a := binary.BigEndian.Uint32(v[((t)&0xffff)+2:])
	if data_a <= binary.BigEndian.Uint32(v[((prev_t)&0xffff)+2:]) {
		t2 := prev_t
		j := i
		_ = indices[j+1]
		for {
			indices[j+1] = prev_t
			j--
			if j < 0 {
				break
			}
			prev_t = indices[j]
			if (t^prev_t) <= 0xffff && data_a <= binary.BigEndian.Uint32(v[((prev_t)&0xffff)+2:]) {
				continue
			} else {
				break
			}
		}
		indices[j+1] = t
		t = t2
	}
}

// basically
func sort_indices(N uint32, v []byte, output []uint16, d *ScratchData) {

	var byte_counters [2][256]uint16
	var counters [2][256]uint16

	v[N] = 0   // make sure extra byte accessed is zero
	v[N+1] = 0 // make sure extra byte accessed is zero

	indices := d.indices[:]
	tmp_indices := d.tmp_indices[:]

	for _, c := range v[:N] {
		byte_counters[1][c]++
	}
	byte_counters[0] = byte_counters[1]
	byte_counters[0][v[0]]--

	counters[0][0] = uint16(byte_counters[0][0])
	counters[1][0] = uint16(byte_counters[1][0]) - 1

	c0 := counters[0][0]
	c1 := counters[1][0]

	for i := 1; i < 256; i++ {
		c0 += uint16(byte_counters[0][i])
		c1 += uint16(byte_counters[1][i])

		counters[0][i] = c0
		counters[1][i] = c1
	}

	counters0 := counters[0][:]

	{ // handle the last byte separately
		byte0 := uint32(v[N-1])
		tmp_indices[counters0[0]] = byte0<<24 | uint32(N-1)
		counters0[0]--
	}

	for i := int(N - 1); i >= 1; i-- {
		byte0 := uint32(v[i-1])
		byte1 := uint32(v[i]) // here we can access extra byte from input array so make sure its zero
		tmp_indices[counters0[v[i]]] = byte0<<24 | byte1<<16 | uint32(i-1)
		counters0[v[i]]--
	}

	counters1 := counters[1][:]
	_ = tmp_indices[N-1]
	for i := int(N - 1); i >= 0; i-- {
		data := tmp_indices[i]
		tmp := counters1[data>>24]
		counters1[data>>24]--
		indices[tmp] = data
	}

	for i := 1; i < int(N); i++ { // no BC here
		if indices[i-1]&0xffff0000 == indices[i]&0xffff0000 {
			fix(v, indices, i-1)
		}
	}

	// after fixing, convert indices to output
	_ = output[N]
	for i, c := range indices[:N] {
		output[i] = uint16(c)
	}
}

func text_32_0alloc(text []byte, sa []int32) {
	if int(int32(len(text))) != len(text) || len(text) != len(sa) {
		panic("suffixarray: misuse of text_16")
	}
	for i := range sa {
		sa[i] = 0
	}
	var memory [2 * 256]int32
	sais_8_32(text, 256, sa, memory[:])
}
