package astrobwt_fast

import "crypto/rand"

//import "strings"
import "testing"

//import "encoding/hex"

import "github.com/deroproject/derohe/astrobwt"

func TestPOW_optimized_v1(t *testing.T) {
	scratch := Pool.Get().(*ScratchData)

	for i := 0; i < 4000; i++ {
		buf := make([]byte, 400, 400)
		rand.Read(buf)

		expected_output := astrobwt.POW16(buf[:])
		actual_output := POW_optimized(buf[:], scratch)

		if string(expected_output[:]) != string(actual_output[:]) {
			t.Fatalf("Test failed: POW and POW_optimized returns different for i=%d buf %x", i, buf)
		}

	}
}

func BenchmarkFastSA(t *testing.B) {
	var buf [stage1_length + 40]byte
	rand.Read(buf[:])
	var output [10000]uint16

	scratch := Pool.Get().(*ScratchData)

	rand.Read(buf[:stage1_length])
	for i := 0; i < t.N; i++ {
		sort_indices(stage1_length, buf[:], output[:], scratch)
		// t.Logf("Ran algo %d", i)
	}
}

func BenchmarkPOW_optimized(t *testing.B) {
	var buf [128]byte
	rand.Read(buf[:])
	scratch := Pool.Get().(*ScratchData)
	for i := 0; i < t.N; i++ {
		_ = POW_optimized(buf[:], scratch)
	}
}
