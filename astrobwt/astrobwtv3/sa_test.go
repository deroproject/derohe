package astrobwtv3

import "testing"

// see https://www.geeksforgeeks.org/burrows-wheeler-data-transform-algorithm/
// see https://www.geeksforgeeks.org/suffix-tree-application-4-build-linear-time-suffix-array/

func TestSuffixArray(t *testing.T) {
	s := "abcabxabcd"
	result32 := []int32{0, 6, 3, 1, 7, 4, 2, 8, 9, 5}

	var sa32 [10]int32
	var sa16 [10]int64
	text_32([]byte(s), sa32[:])
	text_64([]byte(s), sa16[:])

	for i := range result32 {
		if result32[i] != sa32[i] || result32[i] != int32(sa16[i]) {
			t.Fatalf("suffix array failed")
		}
	}
}

/*
func TestSuffixArrayFast(t *testing.T) {
	scratch := Pool.Get().(*ScratchData)
	s := "abcabxabcd"
	result32 := []int32{0, 6, 3, 1, 7, 4, 2, 8, 9, 5}

	var rawdata [MAX_LENGTH]byte
	copy(rawdata[:], []byte(s))
	var sa16 [10]int16
	text_16([]byte(s), sa16[:])

	data_len := uint32(len(s))
	sort_indices(uint32(data_len), rawdata[:data_len+4], scratch.stage1_result[:], scratch) // extra 40 bytes since we may read them, but we never write them

	//t.Logf("stage1_result %+v\n", scratch.stage1_result[:16])
	for i := range result32 {
		if scratch.stage1_result[i] != uint16(sa16[i]) {
			t.Fatalf("suffix array failed")
		}
	}
}

func TestSuffixArrayAllZeroes(t *testing.T) {
	scratch := Pool.Get().(*ScratchData)

	var rawdata [MAX_LENGTH]byte
	data_len := uint32(5)

	//for i := range rawdata[:data_len]{
	//	rawdata[i] = byte(i)
	//}

	var sa32 [MAX_LENGTH]int32
	text_32(rawdata[:data_len], sa32[:data_len])

	var sa16 [MAX_LENGTH]int16
	text_16(rawdata[:data_len], sa16[:data_len])

	//rawdata[data_len] = 0
	sort_indices(uint32(data_len), rawdata[:data_len+40], scratch.stage1_result[:], scratch) // extra 40 bytes since we may read them, but we never write them

	//t.Logf("stage1_result %+v\n", scratch.stage1_result[:16])
	for i := range sa32[:data_len] {

		if scratch.stage1_result[i] != uint16(sa16[i]) {
			t.Logf("true %+v", sa16[:data_len])
			t.Logf("fast %+v", scratch.stage1_result[:data_len])
			t.Fatalf("suffix array failed")
		}

		if scratch.stage1_result[i] != uint16(sa32[i]) {
			t.Logf("true %+v", sa32[:data_len])
			t.Logf("fast %+v", scratch.stage1_result[:data_len])
			t.Fatalf("suffix array failed")
		}
	}
}

func TestSuffixArrayZeroes(t *testing.T) {
	scratch := Pool.Get().(*ScratchData)

	var rawdata [MAX_LENGTH]byte
	data_len := uint32(MAX_LENGTH - 0x44)

	for i := range rawdata[:data_len] {
		rawdata[i] = byte(i)
	}

	var sa32 [MAX_LENGTH]int32
	text_32(rawdata[:data_len], sa32[:data_len])

	//	var sa16 [MAX_LENGTH]int16
	//	text_16(rawdata[:data_len], sa16[:data_len])

	//rawdata[data_len] = 0
	sort_indices(uint32(data_len), rawdata[:data_len+40], scratch.stage1_result[:], scratch) // extra 40 bytes since we may read them, but we never write them

	//t.Logf("stage1_result %+v\n", scratch.stage1_result[:16])
	for i := range sa32[:data_len] {

		//		if scratch.stage1_result[i] != uint16(sa16[i])  {
		//			t.Logf("true %+v", sa16[:data_len])
		//			t.Logf("fast %+v", scratch.stage1_result[:data_len])
		//			t.Fatalf("suffix array failed")
		//		}

		if scratch.stage1_result[i] != uint16(sa32[i]) {
			t.Logf("true %+v", sa32[:data_len])
			t.Logf("fast %+v", scratch.stage1_result[:data_len])
			t.Fatalf("suffix array failed")
		}
	}
}

func TestSuffixArrayIncremental(t *testing.T) {
	scratch := Pool.Get().(*ScratchData)

	var rawdata [MAX_LENGTH + 40]byte
	data_len := uint32(MAX_LENGTH - 1)

	for i := range rawdata[:data_len] {
		rawdata[i] = byte(i)
	}

	var sa32 [MAX_LENGTH]int32
	text_32(rawdata[:data_len], sa32[:data_len])

	//rawdata[data_len] = 0
	sort_indices(uint32(data_len), rawdata[:data_len+40], scratch.stage1_result[:], scratch) // extra 40 bytes since we may read them, but we never write them

	//t.Logf("stage1_result %+v\n", scratch.stage1_result[:16])
	for i := range sa32[:data_len] {

		if scratch.stage1_result[i] != uint16(sa32[i]) {
			t.Logf("true %+v", sa32[:data_len])
			t.Logf("fast %+v", scratch.stage1_result[:data_len])
			t.Fatalf("suffix array failed")
		}
	}
}
*/
