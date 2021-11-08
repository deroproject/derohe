// Program http demonstrates how to set up a JSON-RPC 2.0 server using the
// github.com/creachadair/jrpc2 package with an HTTP transport.
//
// Usage (see also the client example):
//
//   go build github.com/creachadair/jrpc2/tools/examples/http
//   ./http -listen :8080
//
// The server accepts RPCs on http://<address>/rpc.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/creachadair/jrpc2/metrics"
)

var listenAddr = flag.String("listen", "", "Service address")

func main() {
	flag.Parse()
	if *listenAddr == "" {
		log.Fatal("You must provide a non-empty -listen address")
	}

	// Start an HTTP bridge with a single trivial method.
	bridge := jhttp.NewBridge(handler.Map{
		"Ping": handler.New(ping),
	}, &jhttp.BridgeOptions{
		Server: &jrpc2.ServerOptions{
			Logger:  jrpc2.StdLogger(nil),
			Metrics: metrics.New(),
		},
	})
	defer bridge.Close()

	http.Handle("/rpc", bridge)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func ping(ctx context.Context, msg ...string) string {
	return "OK: " + strings.Join(msg, "|")
}
