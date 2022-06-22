package astrobwtv3

import "fmt"

//import "os"
import "math/bits"
import "encoding/binary"
import "crypto/rand"

import "github.com/dchest/siphash"
import "github.com/cespare/xxhash"

//import "github.com/minio/highwayhash"
import "github.com/minio/sha256-simd"
import "github.com/segmentio/fasthash/fnv1a"
import "golang.org/x/crypto/salsa20/salsa"

var _ = fmt.Sprintf
var __ = rand.Read

// go test -bench=Benchmark_Tartarus_POW_128 -cpuprofile /tmp/profile.out ./ && go tool pprof -http :8080 /tmp/profile.out
// for coverage
// go test -coverprofile=/tmp/t.cov ./ && go tool cover -html=/tmp/t.cov && unlink /tmp/t.cov
// run this to generate random code which must be freezed before release
// go run random_code_gen.go -- ./pow.go ./pow.go

const CALCULATE_DISTRIBUTION = false
const REFERENCE_MODE = true

var ops [256]int
var steps = map[uint64]int{}

// this will generate a hash
func AstroBWTv3(input []byte) (outputhash [32]byte) {

	//var static_key = [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	scratch := Pool.Get().(*ScratchData)
	defer Pool.Put(scratch)

	defer func() {
		if r := recover(); r != nil { // if something happens due to RAM issues in miner, we should continue, avoiding crashes if possible
			var buf [16]byte
			rand.Read(buf[:])
			outputhash = sha256.Sum256(buf[:]) // return a random falsified hash which will fail the check
		}
	}()

	var step_3 [256]byte
	var counter [16]byte

	sha_key := sha256.Sum256(input)

	salsa.XORKeyStream(step_3[:], step_3[:], &counter, &sha_key)

	// https://docs.nvidia.com/cuda/ampere-tuning-guide/index.html
	// RC4 requires 256 bytes ( or registers ) on GPU ( assuming only registers are used)
	// each SM has 64K registers,expecting full occupancy, each SM can have 256 threads ( each with 256 registers) simultaneously
	// NVIDIA 3090Ti has 84 SMs, so max runnable instances = 84 * 256 = 21504
	// the below code should reduce GPUS as 84 core machine (running at 1Ghz)

	rc4s := NewCipher(step_3[:]) // new rc4 cipher, pkg from golang src, modified to avoid allocation
	rc4s.XORKeyStream(step_3[:], step_3[:])
	lhash := fnv1a.HashBytes64(step_3[:])
	prev_lhash := lhash // this is used to to randomly switch patterns

	tries := uint64(0)
	// the below for loop is branchy, TODO make it more branchy to avoid GPU/FPGA optimizations
	for { // keep the looop running n number of times,  n >= 1
		tries++

		random_switcher := prev_lhash ^ lhash ^ tries
		//	fmt.Printf("%d random_switcher %d %x\n",tries, random_switcher, random_switcher)

		// see https://github.com/golang/go/issues/5496
		op := byte(random_switcher)

		pos1 := byte(random_switcher >> 8)
		pos2 := byte(random_switcher >> 16)

		if pos1 > pos2 {
			pos1, pos2 = pos2, pos1
		}

		if pos2-pos1 > 32 { // give wave or wavefronts an optimization
			pos2 = pos1 + (pos2-pos1)&0x1f // max update 32 bytes
		}
		//pos_small := pos1 + (pos2-pos1)&0x7 // max update 8 bytes
		//_ = pos_small

		if CALCULATE_DISTRIBUTION {
			ops[op]++
		}

		_ = step_3[pos1:pos2] // bounds check elimination

		switch op {

		case 0:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] *= step_3[i]                                   // *
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				//INSERT_RANDOM_CODE_END
				step_3[pos2], step_3[pos1] = bits.Reverse8(step_3[pos1]), bits.Reverse8(step_3[pos2])
			}
		case 1:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				step_3[i] += step_3[i]                     // +
				//INSERT_RANDOM_CODE_END
			}
		case 2:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 3:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 3)              // rotate  bits by 3
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 4:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] -= (step_3[i] ^ 97)                           // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 5:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 6:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 7:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                   // +
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 8:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 9:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 10:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] *= step_3[i]                     // *
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] *= step_3[i]                     // *
				//INSERT_RANDOM_CODE_END
			}
		case 11:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 12:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 13:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 14:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] *= step_3[i]                   // *
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 15:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 16:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 17:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] *= step_3[i]                     // *
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = ^step_3[i]                     // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 18:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 19:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] += step_3[i]                     // +
				//INSERT_RANDOM_CODE_END
			}
		case 20:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 21:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] += step_3[i]                     // +
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				//INSERT_RANDOM_CODE_END
			}
		case 22:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] *= step_3[i]                     // *
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 23:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				//INSERT_RANDOM_CODE_END
			}
		case 24:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                 // +
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 25:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 26:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                                   // *
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] += step_3[i]                                   // +
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 27:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 28:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] += step_3[i]                     // +
				step_3[i] += step_3[i]                     // +
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 29:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                   // *
				step_3[i] = step_3[i] ^ step_3[pos2]     // XOR
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				step_3[i] += step_3[i]                   // +
				//INSERT_RANDOM_CODE_END
			}
		case 30:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 31:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] *= step_3[i]                                 // *
				//INSERT_RANDOM_CODE_END
			}
		case 32:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 33:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] = bits.Reverse8(step_3[i])                    // reverse bits
				step_3[i] *= step_3[i]                                  // *
				//INSERT_RANDOM_CODE_END
			}
		case 34:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)            // XOR and -
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] -= (step_3[i] ^ 97)            // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 35:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                     // +
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 36:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 37:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				step_3[i] *= step_3[i]                                  // *
				//INSERT_RANDOM_CODE_END
			}
		case 38:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 39:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				//INSERT_RANDOM_CODE_END
			}
		case 40:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 41:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 42:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 3)              // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 43:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2] // AND
				step_3[i] += step_3[i]               // +
				step_3[i] = step_3[i] & step_3[pos2] // AND
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 44:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 45:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 46:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] += step_3[i]                                   // +
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 47:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 48:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 49:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] += step_3[i]                                   // +
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 50:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] += step_3[i]                     // +
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 51:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 52:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 53:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                   // +
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 54:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i]) // reverse bits
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				step_3[i] = ^step_3[i]               // binary NOT operator
				step_3[i] = ^step_3[i]               // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 55:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 56:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 57:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 3)              // rotate  bits by 3
				step_3[i] = bits.Reverse8(step_3[i])                    // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 58:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] += step_3[i]                                 // +
				//INSERT_RANDOM_CODE_END
			}
		case 59:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] *= step_3[i]                                  // *
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 60:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] *= step_3[i]                     // *
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 61:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 62:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] += step_3[i]                                 // +
				//INSERT_RANDOM_CODE_END
			}
		case 63:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				step_3[i] += step_3[i]                                   // +
				//INSERT_RANDOM_CODE_END
			}
		case 64:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] *= step_3[i]                                 // *
				//INSERT_RANDOM_CODE_END
			}
		case 65:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] *= step_3[i]                                 // *
				//INSERT_RANDOM_CODE_END
			}
		case 66:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 67:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 68:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 69:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                   // +
				step_3[i] *= step_3[i]                   // *
				step_3[i] = bits.Reverse8(step_3[i])     // reverse bits
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 70:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 71:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] *= step_3[i]                     // *
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 72:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 73:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 74:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                     // *
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				//INSERT_RANDOM_CODE_END
			}
		case 75:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                                   // *
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 76:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 77:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] += step_3[i]                                   // +
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 78:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.Reverse8(step_3[i])                    // reverse bits
				step_3[i] *= step_3[i]                                  // *
				step_3[i] -= (step_3[i] ^ 97)                           // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 79:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] += step_3[i]                                 // +
				step_3[i] *= step_3[i]                                 // *
				//INSERT_RANDOM_CODE_END
			}
		case 80:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				step_3[i] += step_3[i]                                  // +
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				//INSERT_RANDOM_CODE_END
			}
		case 81:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 82:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]     // XOR
				step_3[i] = ^step_3[i]                   // binary NOT operator
				step_3[i] = ^step_3[i]                   // binary NOT operator
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 83:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 84:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] += step_3[i]                     // +
				//INSERT_RANDOM_CODE_END
			}
		case 85:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 86:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 87:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                 // +
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] += step_3[i]                                 // +
				//INSERT_RANDOM_CODE_END
			}
		case 88:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 89:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                 // +
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 90:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 91:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 92:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				//INSERT_RANDOM_CODE_END
			}
		case 93:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] += step_3[i]                                 // +
				//INSERT_RANDOM_CODE_END
			}
		case 94:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 95:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 96:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 97:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 98:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 99:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 100:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 101:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 102:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				step_3[i] += step_3[i]                     // +
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 103:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] = bits.Reverse8(step_3[i])                    // reverse bits
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 104:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] += step_3[i]                                   // +
				//INSERT_RANDOM_CODE_END
			}
		case 105:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 3)              // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 106:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] *= step_3[i]                                 // *
				//INSERT_RANDOM_CODE_END
			}
		case 107:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 108:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 109:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                                  // *
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 110:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                 // +
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 111:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                   // *
				step_3[i] = bits.Reverse8(step_3[i])     // reverse bits
				step_3[i] *= step_3[i]                   // *
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 112:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 113:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 114:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] = bits.Reverse8(step_3[i])                    // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 115:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = bits.RotateLeft8(step_3[i], 3)              // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 116:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 117:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				//INSERT_RANDOM_CODE_END
			}
		case 118:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] += step_3[i]                     // +
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 119:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 120:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 121:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				step_3[i] += step_3[i]                                   // +
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] *= step_3[i]                                   // *
				//INSERT_RANDOM_CODE_END
			}
		case 122:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 123:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 124:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 125:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] += step_3[i]                                 // +
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 126:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 127:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] *= step_3[i]                   // *
				step_3[i] = step_3[i] & step_3[pos2]     // AND
				step_3[i] = step_3[i] ^ step_3[pos2]     // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 128:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 129:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 130:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 131:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] *= step_3[i]                                   // *
				//INSERT_RANDOM_CODE_END
			}
		case 132:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 133:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 134:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				//INSERT_RANDOM_CODE_END
			}
		case 135:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] += step_3[i]                                 // +
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 136:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 137:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				step_3[i] = bits.Reverse8(step_3[i])                    // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 138:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				step_3[i] += step_3[i]               // +
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 139:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 140:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 141:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] += step_3[i]                                   // +
				//INSERT_RANDOM_CODE_END
			}
		case 142:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 143:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 144:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 145:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 146:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 147:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] *= step_3[i]                                 // *
				//INSERT_RANDOM_CODE_END
			}
		case 148:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 149:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				step_3[i] = bits.Reverse8(step_3[i]) // reverse bits
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				step_3[i] += step_3[i]               // +
				//INSERT_RANDOM_CODE_END
			}
		case 150:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] = step_3[i] & step_3[pos2]     // AND
				//INSERT_RANDOM_CODE_END
			}
		case 151:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                   // +
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] *= step_3[i]                   // *
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 152:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 153:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = ^step_3[i]                     // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 154:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 155:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 156:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 157:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 158:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] += step_3[i]                                   // +
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 159:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)                           // XOR and -
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 160:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 161:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 162:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 163:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 164:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                                   // *
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 165:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] += step_3[i]                                 // +
				//INSERT_RANDOM_CODE_END
			}
		case 166:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] += step_3[i]                                 // +
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 167:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                   // binary NOT operator
				step_3[i] = ^step_3[i]                   // binary NOT operator
				step_3[i] *= step_3[i]                   // *
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 168:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 169:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				//INSERT_RANDOM_CODE_END
			}
		case 170:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				step_3[i] = bits.Reverse8(step_3[i]) // reverse bits
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				step_3[i] *= step_3[i]               // *
				//INSERT_RANDOM_CODE_END
			}
		case 171:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 172:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 173:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                   // binary NOT operator
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] *= step_3[i]                   // *
				step_3[i] += step_3[i]                   // +
				//INSERT_RANDOM_CODE_END
			}
		case 174:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 175:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				step_3[i] *= step_3[i]                     // *
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 176:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] *= step_3[i]                     // *
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 177:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				//INSERT_RANDOM_CODE_END
			}
		case 178:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				step_3[i] += step_3[i]                     // +
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 179:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] += step_3[i]                                 // +
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 180:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 181:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 182:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 183:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]        // +
				step_3[i] -= (step_3[i] ^ 97) // XOR and -
				step_3[i] -= (step_3[i] ^ 97) // XOR and -
				step_3[i] *= step_3[i]        // *
				//INSERT_RANDOM_CODE_END
			}
		case 184:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] *= step_3[i]                     // *
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 185:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 186:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 187:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] += step_3[i]                     // +
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 188:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 189:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 190:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 191:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                  // +
				step_3[i] = bits.RotateLeft8(step_3[i], 3)              // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] >> (step_3[i] & 3)                // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 192:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                   // +
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] += step_3[i]                   // +
				step_3[i] *= step_3[i]                   // *
				//INSERT_RANDOM_CODE_END
			}
		case 193:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 194:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] << (step_3[i] & 3)                // shift left
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				//INSERT_RANDOM_CODE_END
			}
		case 195:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 196:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 197:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] *= step_3[i]                                  // *
				step_3[i] *= step_3[i]                                  // *
				//INSERT_RANDOM_CODE_END
			}
		case 198:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 199:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]               // binary NOT operator
				step_3[i] += step_3[i]               // +
				step_3[i] *= step_3[i]               // *
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 200:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 201:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 202:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 203:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = bits.RotateLeft8(step_3[i], 1)              // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 204:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 205:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = step_3[i] << (step_3[i] & 3)                 // shift left
				step_3[i] += step_3[i]                                   // +
				//INSERT_RANDOM_CODE_END
			}
		case 206:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 207:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 208:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                     // +
				step_3[i] += step_3[i]                     // +
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 209:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 210:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], 5)              // rotate  bits by 5
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 211:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] += step_3[i]                                  // +
				step_3[i] -= (step_3[i] ^ 97)                           // XOR and -
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 212:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)  // rotate  bits by 2
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 213:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                     // +
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 214:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]     // XOR
				step_3[i] -= (step_3[i] ^ 97)            // XOR and -
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				step_3[i] = ^step_3[i]                   // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 215:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2]     // XOR
				step_3[i] = step_3[i] & step_3[pos2]     // AND
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] *= step_3[i]                   // *
				//INSERT_RANDOM_CODE_END
			}
		case 216:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = ^step_3[i]                                  // binary NOT operator
				step_3[i] -= (step_3[i] ^ 97)                           // XOR and -
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				//INSERT_RANDOM_CODE_END
			}
		case 217:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] += step_3[i]                                 // +
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 218:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i]) // reverse bits
				step_3[i] = ^step_3[i]               // binary NOT operator
				step_3[i] *= step_3[i]               // *
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 219:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] & step_3[pos2]                   // AND
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 220:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = step_3[i] << (step_3[i] & 3)   // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 221:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 222:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] = step_3[i] ^ step_3[pos2]     // XOR
				step_3[i] *= step_3[i]                   // *
				//INSERT_RANDOM_CODE_END
			}
		case 223:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)              // rotate  bits by 3
				step_3[i] = step_3[i] ^ step_3[pos2]                    // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] -= (step_3[i] ^ 97)                           // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 224:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 1)             // rotate  bits by 1
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 225:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                     // binary NOT operator
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 226:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i]) // reverse bits
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				step_3[i] *= step_3[i]               // *
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 227:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                   // binary NOT operator
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				step_3[i] -= (step_3[i] ^ 97)            // XOR and -
				step_3[i] = step_3[i] & step_3[pos2]     // AND
				//INSERT_RANDOM_CODE_END
			}
		case 228:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                   // +
				step_3[i] = step_3[i] >> (step_3[i] & 3)                 // shift right
				step_3[i] += step_3[i]                                   // +
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 229:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 230:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                                  // *
				step_3[i] = step_3[i] & step_3[pos2]                    // AND
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 231:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] = step_3[i] ^ step_3[pos2]       // XOR
				step_3[i] = bits.Reverse8(step_3[i])       // reverse bits
				//INSERT_RANDOM_CODE_END
			}
		case 232:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] *= step_3[i]                                 // *
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 233:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				//INSERT_RANDOM_CODE_END
			}
		case 234:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]     // AND
				step_3[i] *= step_3[i]                   // *
				step_3[i] = step_3[i] >> (step_3[i] & 3) // shift right
				step_3[i] = step_3[i] ^ step_3[pos2]     // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 235:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] *= step_3[i]                                 // *
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 236:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				step_3[i] += step_3[i]               // +
				step_3[i] = step_3[i] & step_3[pos2] // AND
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 237:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}
		case 238:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                     // +
				step_3[i] += step_3[i]                     // +
				step_3[i] = bits.RotateLeft8(step_3[i], 3) // rotate  bits by 3
				step_3[i] -= (step_3[i] ^ 97)              // XOR and -
				//INSERT_RANDOM_CODE_END
			}
		case 239:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5) // rotate  bits by 5
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] *= step_3[i]                     // *
				step_3[i] = step_3[i] & step_3[pos2]       // AND
				//INSERT_RANDOM_CODE_END
			}
		case 240:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                   // binary NOT operator
				step_3[i] += step_3[i]                   // +
				step_3[i] = step_3[i] & step_3[pos2]     // AND
				step_3[i] = step_3[i] << (step_3[i] & 3) // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 241:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ step_3[pos2]                     // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 242:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]               // +
				step_3[i] += step_3[i]               // +
				step_3[i] -= (step_3[i] ^ 97)        // XOR and -
				step_3[i] = step_3[i] ^ step_3[pos2] // XOR
				//INSERT_RANDOM_CODE_END
			}
		case 243:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 1)               // rotate  bits by 1
				//INSERT_RANDOM_CODE_END
			}
		case 244:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 245:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] -= (step_3[i] ^ 97)                          // XOR and -
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] >> (step_3[i] & 3)               // shift right
				//INSERT_RANDOM_CODE_END
			}
		case 246:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                     // +
				step_3[i] = bits.RotateLeft8(step_3[i], 1) // rotate  bits by 1
				step_3[i] = step_3[i] >> (step_3[i] & 3)   // shift right
				step_3[i] += step_3[i]                     // +
				//INSERT_RANDOM_CODE_END
			}
		case 247:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 5)             // rotate  bits by 5
				step_3[i] = ^step_3[i]                                 // binary NOT operator
				//INSERT_RANDOM_CODE_END
			}
		case 248:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = ^step_3[i]                                   // binary NOT operator
				step_3[i] -= (step_3[i] ^ 97)                            // XOR and -
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 5)               // rotate  bits by 5
				//INSERT_RANDOM_CODE_END
			}
		case 249:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                    // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)  // rotate  bits by 4
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i])) // rotate  bits by random
				//INSERT_RANDOM_CODE_END
			}
		case 250:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] & step_3[pos2]                     // AND
				step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]))  // rotate  bits by random
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4)   // rotate  bits by 4
				//INSERT_RANDOM_CODE_END
			}
		case 251:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] += step_3[i]                                   // +
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.Reverse8(step_3[i])                     // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				//INSERT_RANDOM_CODE_END
			}
		case 252:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.Reverse8(step_3[i])                   // reverse bits
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 4) // rotate  bits by 4
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] << (step_3[i] & 3)               // shift left
				//INSERT_RANDOM_CODE_END
			}
		case 253:
			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2) // rotate  bits by 2
				step_3[i] = step_3[i] ^ step_3[pos2]                   // XOR
				step_3[i] = bits.RotateLeft8(step_3[i], 3)             // rotate  bits by 3
				//INSERT_RANDOM_CODE_END

				prev_lhash = lhash + prev_lhash
				lhash = xxhash.Sum64(step_3[:pos2]) // more deviations
			}

		case 254, 255: // 0.7% chance of execution every loop
			rc4s = NewCipher(step_3[:]) // use a new key
			//step_3 = highwayhash.Sum(step_3[:], step_3[:])

			for i := pos1; i < pos2; i++ {
				//INSERT_RANDOM_CODE_START
				step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				step_3[i] = step_3[i] ^ bits.RotateLeft8(step_3[i], 2)   // rotate  bits by 2
				step_3[i] = bits.RotateLeft8(step_3[i], 3)               // rotate  bits by 3
				//INSERT_RANDOM_CODE_END
			}

		default:

		}

		if step_3[pos1]-step_3[pos2] < 0x10 { // 6.25 % probability
			prev_lhash = lhash + prev_lhash
			lhash = xxhash.Sum64(step_3[:pos2]) // more deviations
		}

		if step_3[pos1]-step_3[pos2] < 0x20 { // 12.5 % probability
			prev_lhash = lhash + prev_lhash
			lhash = fnv1a.HashBytes64(step_3[:pos2]) // more deviations
		}

		if step_3[pos1]-step_3[pos2] < 0x30 { // 18.75 % probability
			prev_lhash = lhash + prev_lhash
			lhash = siphash.Hash(tries, prev_lhash, step_3[:pos2]) // more deviations
		}

		if step_3[pos1]-step_3[pos2] <= 0x40 { // 25% probablility
			rc4s.XORKeyStream(step_3[:], step_3[:]) // do the rc4
		}

		step_3[255] = step_3[255] ^ step_3[pos1] ^ step_3[pos2]

		copy(scratch.data[(tries-1)*256:], step_3[:]) // copy all the tmp states

		if tries > 260+16 || (step_3[255] >= 0xf0 && tries > 260) { // keep looping until condition is satisfied
			break
		}

	}

	if CALCULATE_DISTRIBUTION {
		steps[tries]++
	}

	// we may discard upto ~ 1KiB data from the stream
	data_len := uint32((tries-4)*256 + (uint64(step_3[253])<<8|uint64(step_3[254]))&0x3ff) // ensure wide  number of variants exists

	//if REFERENCE_MODE {
	text_32_0alloc(scratch.data[:data_len], scratch.sa[:data_len])
	//}

	if LittleEndian {
		scratch.hasher.Reset()
		scratch.hasher.Write(scratch.sa_bytes[:data_len*4])
	} else {
		var s [MAX_LENGTH * 4]byte
		for i, c := range scratch.sa[:data_len] {
			binary.LittleEndian.PutUint32(s[i<<1:], uint32(c))
		}
		scratch.hasher.Reset()
		scratch.hasher.Write(s[:data_len*4])
	}

	_ = scratch.hasher.Sum(outputhash[:0])

	return outputhash
}
