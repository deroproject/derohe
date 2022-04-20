package astrobwt_fast

import "unsafe"
import "hash"
import "sync"
import "crypto/rand"
import "encoding/binary"
import "golang.org/x/crypto/sha3"
import "golang.org/x/crypto/salsa20/salsa"

const stage1_length uint32 = 9973 // it is a prime

// see here to improve the algorithms more https://github.com/y-256/libdivsufsort/blob/wiki/SACA_Benchmarks.md
// this optimized algorithm is used only  in the miner and not in the blockchain

type ScratchData struct {
	hasher              hash.Hash
	stage1              [stage1_length + 64]byte // 10 KB stages are taken from it
	stage1_result       *[stage1_length + 1]uint16
	stage1_result_bytes *[(stage1_length) * 2]uint8
	indices             [stage1_length + 1]uint32 // 40 KB
	tmp_indices         [stage1_length + 1]uint32 // 40 KB
}

var Pool = sync.Pool{New: func() interface{} {
	var d ScratchData
	d.hasher = sha3.New256()
	d.stage1_result = ((*[stage1_length + 1]uint16)(unsafe.Pointer(&d.indices[0])))
	d.stage1_result_bytes = ((*[(stage1_length) * 2]byte)(unsafe.Pointer(&d.indices[0])))

	return &d
}}

func POW_optimized(inputdata []byte, data *ScratchData) (outputhash [32]byte) {

	defer func() {
		if r := recover(); r != nil { // if something happens due to RAM issues in miner, we should continue, system will crash sooner or later
			var buf [16]byte
			rand.Read(buf[:])
			outputhash = sha3.Sum256(buf[:]) // return a falsified has which will fail the check
		}
	}()

	var key [32]byte
	for i := range data.stage1 {
		data.stage1[i] = 0
	}

	var counter [16]byte

	data.hasher.Reset()
	data.hasher.Write(inputdata)
	_ = data.hasher.Sum(key[:0])

	salsa.XORKeyStream(data.stage1[:stage1_length], data.stage1[:stage1_length], &counter, &key)
	sort_indices(stage1_length, data.stage1[:stage1_length+40], data.stage1_result[:], data) // extra 40 bytes since we may read them, but we never write them

	if LittleEndian {
		data.hasher.Reset()
		data.hasher.Write(data.stage1_result_bytes[:])
		_ = data.hasher.Sum(key[:0])
	} else {
		var s [stage1_length * 2]byte
		for i, c := range data.stage1_result {
			binary.LittleEndian.PutUint16(s[i<<1:], c)
		}
		data.hasher.Reset()
		data.hasher.Write(s[:])
		_ = data.hasher.Sum(key[:0])
	}

	copy(outputhash[:], key[:])
	return
}

func fix(v []byte, indices []uint32, i int) {
	prev_t := indices[i]
	t := indices[i+1]

	data_a := binary.BigEndian.Uint32(v[((t)&0xffff)+2:])
	if data_a < binary.BigEndian.Uint32(v[((prev_t)&0xffff)+2:]) {
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
			if (t^prev_t) <= 0xffff && data_a < binary.BigEndian.Uint32(v[((prev_t)&0xffff)+2:]) {
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

	var byte_counters [2][256]byte
	var counters [2][256]uint16

	v[N] = 0 // make sure extra byte accessed is zero

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
	for i := int(N); i >= 1; i-- {
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
