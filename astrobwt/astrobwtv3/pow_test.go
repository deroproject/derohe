package astrobwtv3

import "os"
import "fmt"
import "math/rand"
import "testing"

var cases [][]byte

func init_basic() {
	rand.Seed(1)
	alphabet := "abcdefghjijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890"
	n := len(alphabet)
	_ = n
	scales := []int{64}
	cases = make([][]byte, len(scales))
	for i, scale := range scales {
		l := scale
		buf := make([]byte, int(l))
		for j := 0; j < int(l); j++ {
			buf[j] = byte(rand.Uint32() & 0xff) //alphabet[rand.Intn(n)]
		}
		cases[i] = buf
	}
}

type PowTest struct {
	out string
	in  string
}

// can be used as referenc vectors
var random_pow_tests = []PowTest{
	{"54e2324ddacc3f0383501a9e5760f85d63e9bc6705e9124ca7aef89016ab81ea", "a"},
	{"faeaff767be60134f0bcc5661b5f25413791b4df8ad22ff6732024d35ec4e7d0", "ab"},
	{"715c3d8c61a967b7664b1413f8af5a2a9ba0005922cb0ba4fac8a2d502b92cd6", "abc"},
	{"74cc16efc1aac4768eb8124e23865da4c51ae134e29fa4773d80099c8bd39ab8", "abcd"},
	{"d080d0484272d4498bba33530c809a02a4785368560c5c3eac17b5dacd357c4b", "abcde"},
	{"ecd6e38831e64304071517c2eaa2f8a90333643849b672e81ab72c41d1237e9c", "abcdef"},
	{"18dae03cfaedbe50859e19f599ea39aa541058a8c536b5697032d484c3747d74", "abcdefg"},
	{"f96191b7e39568301449d75d42d05090e41e3f79a462819473a62b1fcc2d0997", "abcdefgh"},
	{"8c76af6a57dfed744d5b7467fa822d9eb8536a851884aa7d8e3657028d511322", "abcdefghi"},
	{"f838568c38f83034b2ff679d5abf65245bd2be1b27c197ab5fbac285061cf0a7", "abcdefghij"},
}

func TestAstroBWTv3(t *testing.T) {
	for i := range random_pow_tests {
		g := random_pow_tests[i]
		s := fmt.Sprintf("%x", AstroBWTv3([]byte(g.in)))
		if s != g.out {
			t.Fatalf("Pow function: pow(%s) = %s want %s", g.in, s, g.out)
		}
	}
}

func Benchmark_AstroBWTv3_2(b *testing.B) {
	benchmark_AstroBWTv3(b, 2)
}
func Benchmark_AstroBWTv3_4(b *testing.B) {
	benchmark_AstroBWTv3(b, 4)
}
func Benchmark_AstroBWTv3_8(b *testing.B) {
	benchmark_AstroBWTv3(b, 8)
}
func Benchmark_AstroBWTv3_16(b *testing.B) {
	benchmark_AstroBWTv3(b, 16)
}
func Benchmark_AstroBWTv3_32(b *testing.B) {
	benchmark_AstroBWTv3(b, 32)
}
func Benchmark_AstroBWTv3_48(b *testing.B) {
	benchmark_AstroBWTv3(b, 48)
}
func Benchmark_AstroBWTv3_64(b *testing.B) {
	benchmark_AstroBWTv3(b, 64)
}
func Benchmark_AstroBWTv3_128(b *testing.B) {
	benchmark_AstroBWTv3(b, 128)
}
func benchmark_AstroBWTv3(b *testing.B, length int) {
	b.ReportAllocs()
	init_basic()

	var inputs [1024][]byte
	for i := 0; i < 1024; i++ {
		inputs[i] = make([]byte, length, length)
		rand.Read(inputs[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AstroBWTv3(inputs[i%1024])
	}

	if CALCULATE_DISTRIBUTION {
		// use sort -gr /tmp/distribution_tries.csv | less
		file, err := os.Create("/tmp/distribution_ops.csv")
		if err == nil {
			for k, v := range ops {
				fmt.Fprintf(file, "%d:%d\n", v, k)
			}
			file.Close()
		}

		file, err = os.Create("/tmp/distribution_tries.csv")
		if err == nil {
			for k, v := range steps {
				fmt.Fprintf(file, "%d:%d\n", v, k)
			}
			file.Close()
		}
	}

}

/*
func Benchmark_SQRT(b *testing.B) {
	b.ReportAllocs()

	var buf []byte

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculatesqrt(rand.Uint64(),rand.Uint64(),buf)
	}
}
*/
