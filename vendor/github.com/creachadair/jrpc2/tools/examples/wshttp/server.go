// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

// Program wshttp demonstrates how to set up a JSON-RPC 20 server using
// the github.com/creachadair/jrpc2 package with a Websocket transport.
//
// Usage:
//   go build github.com/creachadair/jrpc2/tools/examples/wshttp
//   ./wshttp -listen :8080
//
// The server accepts RPC connections on ws://<address>/rpc.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/wschannel"
)

var listenAddr = flag.String("listen", "", "Service address")

func main() {
	flag.Parse()
	if *listenAddr == "" {
		log.Fatal("You must provide a non-empty -listen address")
	}

	lst := wschannel.NewListener(nil)
	hs := &http.Server{Addr: *listenAddr, Handler: http.DefaultServeMux}
	http.Handle("/rpc", lst)
	go hs.ListenAndServe()

	svc := server.Static(handler.Map{
		"Reverse": handler.New(reverse),
	})

	log.Printf("Listening at ws://%s/rpc", *listenAddr)
	ctx := context.Background()
	err := server.Loop(ctx, accepter{lst}, svc, &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			Logger: jrpc2.StdLogger(nil),
		},
	})
	hs.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Loop exited: %v", err)
	}
}

func reverse(_ context.Context, ss []string) []string {
	for i, j := 0, len(ss)-1; i < j; i++ {
		ss[i], ss[j] = ss[j], ss[i]
		j--
	}
	return ss
}

type accepter struct{ *wschannel.Listener }

func (a accepter) Accept(ctx context.Context) (channel.Channel, error) {
	return a.Listener.Accept(ctx)
}
