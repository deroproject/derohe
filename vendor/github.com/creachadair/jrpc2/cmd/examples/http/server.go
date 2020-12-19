// Program http demonstrates how to set up a JSON-RPC 2.0 server using the
// github.com/creachadair/jrpc2 package with an HTTP transport.
//
// Usage (see also the client example):
//
//   go build github.com/creachadair/jrpc2/cmd/examples/http
//   ./http -port 8080
//
// The server accepts RPCs on http://localhost:<port>/rpc.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/creachadair/jrpc2/metrics"
	"github.com/creachadair/jrpc2/server"
)

var port = flag.Int("port", 0, "Service port")

func main() {
	flag.Parse()
	if *port <= 0 {
		log.Fatal("You must provide a positive -port to listen on")
	}

	// Start a local server with a single trivial method and bridge it to HTTP.
	local := server.NewLocal(handler.Map{
		"Ping": handler.New(func(ctx context.Context, msg ...string) string {
			return "OK: " + strings.Join(msg, ", ")
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{
			Logger:  log.New(os.Stderr, "[jhttp.Bridge] ", log.LstdFlags|log.Lshortfile),
			Metrics: metrics.New(),
		},
	})
	http.Handle("/rpc", jhttp.NewBridge(local.Client))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
