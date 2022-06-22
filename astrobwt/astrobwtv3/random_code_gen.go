//go:build ignore
// +build ignore

// run this code to overwrite existing sections
//go run random_code_gen.go -- ./pow.go /tmp/test.go

package main

import "fmt"
import "math/rand"
import "os"
import "regexp"

var random_lines = []string{
	"step_3[i] += step_3[i] // +",
	"step_3[i] -= (step_3[i] ^ 97)// XOR and - ",
	"step_3[i] *= step_3[i] // *",
	"step_3[i] = step_3[i]^step_3[pos2] // XOR",
	"step_3[i] =  ^step_3[i]  // binary NOT operator",
	"step_3[i] = step_3[i] & step_3[pos2] // AND",
	"step_3[i] = step_3[i] << (step_3[i]&3) // shift left",
	"step_3[i] = step_3[i] >> (step_3[i]&3) // shift right",
	"step_3[i] = bits.Reverse8(step_3[i]) // reverse bits",
	"step_3[i] = step_3[i] ^ byte(bits.OnesCount8(step_3[i])) // ones count bits",
	"step_3[i] = bits.RotateLeft8(step_3[i], int(step_3[i]) ) // rotate  bits by random",
	"step_3[i] = bits.RotateLeft8(step_3[i], 1 ) // rotate  bits by 1",
	"step_3[i] = step_3[i]^bits.RotateLeft8(step_3[i], 2 ) // xor rotate  bits by 2",
	"step_3[i] = bits.RotateLeft8(step_3[i], 3 ) // rotate  bits by 3",
	"step_3[i] = step_3[i]^bits.RotateLeft8(step_3[i], 4 ) // xor rotate  bits by 4",
	"step_3[i] = bits.RotateLeft8(step_3[i], 5 ) // rotate  bits by 5",
}

func main() {

	if len(os.Args) == 4 && os.Args[1] == "--" {
		os.Args = append(os.Args[:1], os.Args[1+1:]...)
	}
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "this program needs 2 arguments input_file output_file current args ([%+v]\n", os.Args)
		os.Exit(-1)
	}

	inputfile, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open inputfile %s\n", err)
		os.Exit(-1)
	}

	pattern := regexp.MustCompile(`(?s)INSERT_RANDOM_CODE_START.*?INSERT_RANDOM_CODE_END`)

	//fmt.Printf("%q\n", pattern.Split(string(inputfile), -1)) // ["b" "n" "n" ""]

	parts := pattern.Split(string(inputfile), -1)

	output := ""

	rand.Seed(99)
	for i := range parts {
		if i == 0 {
			output += fmt.Sprintf("%s", parts[i])
			continue
		}
		output += fmt.Sprintf("INSERT_RANDOM_CODE_START\n")
		for i := 0; i < 4; i++ {
			output += fmt.Sprintf("%s\n", random_lines[rand.Intn(len(random_lines))])
		}
		output += fmt.Sprintf("//INSERT_RANDOM_CODE_END")
		output += fmt.Sprintf("%s", parts[i])

	}

	if err = os.WriteFile(os.Args[2], []byte(output), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot write outputfile %s\n", err)
		os.Exit(-1)
	}
}
