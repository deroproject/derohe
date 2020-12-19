// Program jsh exposes a trivial command-shell functionality via JSON-RPC for
// demonstration purposes.
//
// Usage:
//    go build github.com/creachadair/jrpc2/cmd/examples/jsh
//    ./jsh -port 8080
//
// See also cmd/examples/jcl/jcl.go.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/server"
)

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

// Run invokes the specified process and returns the result. It is not an RPC
// error if the process returns a nonzero exit status, unless the process fails
// to start at all.
func Run(ctx context.Context, req *RunReq) (*RunResult, error) {
	if len(req.Args) == 0 || req.Args[0] == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing command name")
	}
	if req.Args[0] == "cd" {
		if len(req.Args) != 2 {
			return nil, jrpc2.Errorf(code.InvalidParams, "wrong arguments for cd")
		}
		return &RunResult{
			Success: os.Chdir(req.Args[1]) == nil,
		}, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, req.Args[0], req.Args[1:]...)
	if len(req.Input) != 0 {
		cmd.Stdin = bytes.NewReader(req.Input)
	}
	run := cmd.Output
	if req.Stderr {
		run = cmd.CombinedOutput
	}
	out, err := run()
	success := err == nil
	if err != nil {
		if ex, ok := err.(*exec.ExitError); ok && ex.Success() {
			success = true
		} else {
			return nil, err
		}
	}
	return &RunResult{
		Success: success,
		Output:  out,
	}, nil
}

var (
	port    = flag.Int("port", 0, "Service port")
	logging = flag.Bool("log", false, "Enable verbose logging")

	lw *log.Logger
)

func main() {
	flag.Parse()
	if *port <= 0 {
		log.Fatal("You must specify a positive --port value")
	} else if *logging {
		lw = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
	}

	lst, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalln("Listen:", err)
	}
	log.Printf("Listening for connections at %s...", lst.Addr())

	server.Loop(lst, server.NewStatic(handler.Map{
		"Run": handler.New(Run),
	}), &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			AllowV1:       true,
			Logger:        lw,
			DecodeContext: jctx.Decode,
		},
	})
}
