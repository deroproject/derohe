// Program jcl is a client program for the demonstration shell-server defined
// in jsh.go.
//
// It implements a trivial command-line reader and dispatcher that sends
// commands via JSON-RPC to the server and prints the responses.  Unlike a real
// shell there is no job control or input redirection; command lines are read
// directly from stdin and packaged as written.
//
// If a line ends in "\" the backslash is stripped off and the next line is
// concatenated to the current line.
//
// If the last token on the command line is "<<" the reader accumulates all
// subsequent lines until a "." on a line by itself as input for the command.
// Escape a plain "." by doubling it "..".
//
// Use the command ":stderr" to toggle reporting of stderr from commands.
//
// Usage:
//    go build github.com/creachadair/jrpc2/cmd/examples/jcl
//    ./jcl -server :8080
//
// See also cmd/examples/jsh/jsh.go.
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"bitbucket.org/creachadair/shell"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/jctx"
)

var (
	serverAddr  = flag.String("server", "", "Server address")
	wantStderr  = flag.Bool("stderr", false, "Capture stderr from commands")
	callTimeout = flag.Duration("timeout", 0, "Call timeout (0 means none)")
)

func main() {
	flag.Parse()
	if *serverAddr == "" {
		log.Fatal("You must provide a non-empty --server address")
	}

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Dialing %q: %v", *serverAddr, err)
	}
	log.Printf("Connected to %s...", conn.RemoteAddr())
	defer conn.Close()

	cli := jrpc2.NewClient(channel.RawJSON(conn, conn), &jrpc2.ClientOptions{
		EncodeContext: jctx.Encode,
	})
	in := bufio.NewScanner(os.Stdin)
	for {
		req, err := readCommand(in)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("ERROR: %v", err)
		}

		ctx := context.Background()
		var cancel context.CancelFunc = func() {}
		if *callTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, *callTimeout)
		}
		var result RunResult
		if err := cli.CallResult(ctx, "Run", req, &result); err != nil {
			fmt.Fprintf(os.Stderr, "# Error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "# Succeeded: %v\n", result.Success)
			os.Stdout.Write(result.Output)
		}
		cancel()
	}
	fmt.Fprintln(os.Stderr, "Bye!")
}

// RunReq is a request to invoke a program.
type RunReq struct {
	Args   []string `json:"args"`   // The command line to execute
	Input  []byte   `json:"input"`  // If nonempty, becomes the standard input of the subprocess
	Stderr bool     `json:"stderr"` // Whether to capture stderr from the subprocess
}

// RunResult is the result of executing a program.
type RunResult struct {
	Success bool   `json:"success"`          // Whether the process succeeded (exit status 0)
	Output  []byte `json:"output,omitempty"` // The output from the process
}

func readCommand(in *bufio.Scanner) (*RunReq, error) {
	for {
		cmd, err := readArgs(in)
		if err != nil {
			return nil, err
		}

		// Burst the line into tokens.
		args, ok := shell.Split(strings.Join(cmd, " "))
		if !ok {
			log.Printf("? Invalid command: unbalanced string quotes")
			continue
		} else if len(args) == 0 {
			continue
		}
		if len(args) == 1 && args[0] == ":stderr" {
			*wantStderr = !*wantStderr
			fmt.Fprintf(os.Stderr, "Request stderr: %v\n", *wantStderr)
			continue
		}

		// Check for an input marker, e.g., "<<" or "<<filename".
		args, input, err := readInput(in, args)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		return &RunReq{
			Args:   args,
			Input:  input,
			Stderr: *wantStderr,
		}, nil
	}
}

func readArgs(in *bufio.Scanner) ([]string, error) {
	// Read a command line, allowing continuations.
	fmt.Fprint(os.Stderr, "> ")
	var cmd []string
	for in.Scan() {
		line := in.Text()
		trim := strings.TrimSuffix(line, "\\")
		cmd = append(cmd, trim)
		if trim == line {
			break
		}
		fmt.Fprint(os.Stderr, "+ ")
	}
	if err := in.Err(); err != nil {
		return nil, err
	} else if len(cmd) == 0 {
		return nil, io.EOF
	}
	return cmd, nil
}

func readInput(in *bufio.Scanner, args []string) ([]string, []byte, error) {
	n := len(args) - 1
	if trim := strings.TrimPrefix(args[n], "<<"); trim != args[n] {
		args = args[:n]
		if trim != "" {
			data, err := ioutil.ReadFile(trim)
			if err != nil {
				log.Fatalf("Error reading: %v", err)
			}
			return args, data, nil
		}
		var buf bytes.Buffer
		fmt.Fprint(os.Stderr, "* ")
	moreInput:
		for in.Scan() {
			switch in.Text() {
			case ".":
				break moreInput
			case "..":
				buf.WriteString(".\n")
			default:
				fmt.Fprintln(&buf, in.Text())
			}
			fmt.Fprint(os.Stderr, "* ")
		}
		if err := in.Err(); err != nil {
			return nil, nil, fmt.Errorf("reading input: %v", err)
		}
		return args, buf.Bytes(), nil
	}
	return args, nil, nil
}
