// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

// Program adder demonstrates a trivial JSON-RPC server that communicates over
// the process's stdin and stdout.
//
// Usage:
//    go build github.com/creachadair/jrpc2/tools/examples/adder
//    ./adder
//
// Queries to try (copy and paste):
//    {"jsonrpc":"2.0", "id":1, "method":"Add", "params":[1,2,3]}
//    {"jsonrpc":"2.0", "id":2, "method":"rpc.serverInfo"}
//
package main

import (
	"context"
	"log"
	"os"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
)

// Add will be exported as a method named "Add".
func Add(ctx context.Context, vs []int) int {
	sum := 0
	for _, v := range vs {
		sum += v
	}
	return sum
}

func main() {
	// Set up the server to respond to "Add" by calling the add function.
	s := jrpc2.NewServer(handler.Map{
		"Add": handler.New(Add),
	}, nil)

	// Start the server on a channel comprising stdin/stdout.
	s.Start(channel.Line(os.Stdin, os.Stdout))
	log.Print("Server started")

	// Wait for the server to exit, and report any errors.
	if err := s.Wait(); err != nil {
		log.Printf("Server exited: %v", err)
	}
}
