package astrobwt

import "time"
import "math/rand"
import "testing"

// see https://www.geeksforgeeks.org/burrows-wheeler-data-transform-algorithm/
// see https://www.geeksforgeeks.org/suffix-tree-application-4-build-linear-time-suffix-array/

func TestSuffixArray(t *testing.T) {
	s := "abcabxabcd"
	result32 := []int32{0, 6, 3, 1, 7, 4, 2, 8, 9, 5}

	var sa32 [10]int32
	var sa16 [10]int16
	text_32([]byte(s), sa32[:])
	text_16([]byte(s), sa16[:])

	for i := range result32 {
		if result32[i] != sa32[i] || result32[i] != int32(sa16[i]) {
			t.Fatalf("suffix array failed")
		}
	}
}

/*
func TestSuffixArrayOptimized(t *testing.T) {
	s := "abcabxabcdaaaaaaa"
	result := []int16{0,6,3,1,7,4,2,8,9,5}

	var output [10]int16
	//var sa_bytes *[10*4]uint8 = (*[stage1_length*4]uint8)(unsafe.Pointer(&sa))
	sort_indices_local(10,[]byte(s),output[:])
	t.Logf("output %+v\n",output[:])

    for i := range result {
		if result[i] != output[i] {
			t.Fatalf("suffix array failed")
		}
	}
}
*/

func TestPows(t *testing.T) {

	for loop_var := 0; loop_var < 1; loop_var++ {

		seed := time.Now().UnixNano()
		//seed = 1635948770488138379
		rand.Seed(seed)

		var input [stage1_length + 16]byte

		rand.Read(input[:stage1_length])

		result16 := POW16(input[:stage1_length])
		result32 := POW32(input[:stage1_length])
		//resultopt := POW_optimized(input[:stage1_length])

		if result16 != result32 {
			t.Fatalf("pow test failed, seed %d %x %x ", seed, result16, result32)
		}

	}
}

/*func TestSuffixArrays(t *testing.T) {

	//200 length seed 1635933734608607364
	//100 length seed 1635933812384665346
	//20 length seed 1635933934317660796
	//10 length seed 1635933991384310043
	//5 length seed 1635942855521802761

	//for loop_var :=0 ; loop_var < 100000;loop_var++ {
	{
	seed := time.Now().UnixNano()
	seed = 1635942855521802761
	rand.Seed(seed)



	var input  [stage1_length+16]byte

	var result_sa16  [stage1_length]int16
	var result_sa32  [stage1_length]int32
	var result_optimized  [stage1_length+16]int16


	rand.Read(input[:stage1_length])


	text_16(input[:stage1_length], result_sa16[:])
	text_32(input[:stage1_length], result_sa32[:])
	sort_indices_local(stage1_length,input[:],result_optimized[:])

	t.Logf("inputt %+v\n", input)
	t.Logf("output16 %+v\n", result_sa16)
	t.Logf("outputoo %+v\n", result_optimized[:stage1_length])

	diff_count := 0
	for i := range result_sa16 {
	if  result_sa16[i] != result_optimized[i] {
		diff_count++
	}}

	//t.Logf("difference count %d ",diff_count)

    for i := range result_sa16 {
		if int32(result_sa16[i]) != result_sa32[i]  || result_sa16[i] != result_optimized[i] {
			t.Fatalf("suffix array internal failed %d, seed %d",i,seed)
		}
	}
}

}
*/

var cases [][]byte

func init() {
	rand.Seed(1)
	alphabet := "abcdefghjijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890"
	n := len(alphabet)
	_ = n
	scales := []int{stage1_length}
	cases = make([][]byte, len(scales))
	for i, scale := range scales {
		l := scale
		buf := make([]byte, int(l))
		for j := 0; j < int(l); j++ {
			buf[j] = byte(rand.Uint32() & 0xff) //alphabet[rand.Intn(n)]
		}
		cases[i] = buf
	}
	//POW16([]byte{0x99})

}

func BenchmarkPOW16(t *testing.B) {
	rand.Read(cases[0][:])
	for i := 0; i < t.N; i++ {
		_ = POW16(cases[0][:])
	}
}
func BenchmarkPOW32(t *testing.B) {
	rand.Read(cases[0][:])
	for i := 0; i < t.N; i++ {
		_ = POW32(cases[0][:])
	}
}

/*
func BenchmarkOptimized(t *testing.B) {
	rand.Read(cases[0][:])
	for i := 0; i < t.N; i++ {
		_ = POW_optimized(cases[0][:])
	}
}
*/
