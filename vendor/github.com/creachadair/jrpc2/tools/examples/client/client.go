// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

// Program client demonstrates how to set up a JSON-RPC 2.0 client using the
// github.com/creachadair/jrpc2 package.
//
// Usage (communicates with the server example):
//
//   go build github.com/creachadair/jrpc2/tools/examples/client
//   ./client -server :8080
//
// See also examples/server/server.go.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net"
	"sync"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
)

var serverAddr = flag.String("server", "", "Server address")

func add(ctx context.Context, cli *jrpc2.Client, vs ...int) (result int, err error) {
	err = cli.CallResult(ctx, "Math.Add", vs, &result)
	return
}

func div(ctx context.Context, cli *jrpc2.Client, x, y int) (result float64, err error) {
	err = cli.CallResult(ctx, "Math.Div", handler.Obj{"X": x, "Y": y}, &result)
	return
}

func stat(ctx context.Context, cli *jrpc2.Client) (result string, err error) {
	err = cli.CallResult(ctx, "Math.Status", nil, &result)
	return
}

func alert(ctx context.Context, cli *jrpc2.Client, msg string) error {
	return cli.Notify(ctx, "Post.Alert", handler.Obj{"message": msg})
}

func intResult(rsp *jrpc2.Response) int {
	var v int
	if err := rsp.UnmarshalResult(&v); err != nil {
		log.Fatalln("UnmarshalResult:", err)
	}
	return v
}

func main() {
	flag.Parse()
	if *serverAddr == "" {
		log.Fatal("You must provide -server address to connect to")
	}

	conn, err := net.Dial(jrpc2.Network(*serverAddr))
	if err != nil {
		log.Fatalf("Dial %q: %v", *serverAddr, err)
	}
	log.Printf("Connected to %v", conn.RemoteAddr())

	// Start up the client, and enable logging to stderr.
	cli := jrpc2.NewClient(channel.Line(conn, conn), &jrpc2.ClientOptions{
		OnNotify: func(req *jrpc2.Request) {
			var params json.RawMessage
			req.UnmarshalParams(&params)
			log.Printf("[server push] Method %q params %#q", req.Method(), string(params))
		},
	})
	defer cli.Close()
	ctx := context.Background()

	log.Print("\n-- Sending a notification...")
	if err := alert(ctx, cli, "There is a fire!"); err != nil {
		log.Fatalln("Notify:", err)
	}

	log.Print("\n-- Sending some individual requests...")
	if sum, err := add(ctx, cli, 1, 3, 5, 7); err != nil {
		log.Fatalln("Math.Add:", err)
	} else {
		log.Printf("Math.Add result=%d", sum)
	}
	if quot, err := div(ctx, cli, 82, 19); err != nil {
		log.Fatalln("Math.Div:", err)
	} else {
		log.Printf("Math.Div result=%.3f", quot)
	}
	if s, err := stat(ctx, cli); err != nil {
		log.Fatalln("Math.Status:", err)
	} else {
		log.Printf("Math.Status result=%q", s)
	}

	// An error condition (division by zero)
	if quot, err := div(ctx, cli, 15, 0); err != nil {
		log.Printf("Math.Div err=%v", err)
	} else {
		log.Fatalf("Math.Div succeeded unexpectedly: result=%v", quot)
	}

	log.Print("\n-- Sending a batch of requests...")
	var specs []jrpc2.Spec
	for i := 1; i <= 5; i++ {
		x := rand.Intn(100)
		for j := 1; j <= 5; j++ {
			y := rand.Intn(100)
			specs = append(specs, jrpc2.Spec{
				Method: "Math.Mul",
				Params: handler.Obj{"X": x, "Y": y},
			})
		}
	}
	rsps, err := cli.Batch(ctx, specs)
	if err != nil {
		log.Fatalln("Batch:", err)
	}
	for i, rsp := range rsps {
		if err := rsp.Error(); err != nil {
			log.Printf("Req %q %s failed: %v", specs[i].Method, rsp.ID(), err)
			continue
		}
		log.Printf("Req %q %s: result=%d", specs[i].Method, rsp.ID(), intResult(rsp))
	}

	log.Print("\n-- Sending individual concurrent requests...")
	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		x := rand.Intn(100)
		for j := 1; j <= 5; j++ {
			y := rand.Intn(100)
			wg.Add(1)
			go func() {
				defer wg.Done()
				var result int
				if err := cli.CallResult(ctx, "Math.Sub", handler.Obj{"X": x, "Y": y}, &result); err != nil {
					log.Printf("Req (%d-%d) failed: %v", x, y, err)
					return
				}
				log.Printf("Req (%d - %d): result=%d", x, y, result)
			}()
		}
	}
	wg.Wait()
}
