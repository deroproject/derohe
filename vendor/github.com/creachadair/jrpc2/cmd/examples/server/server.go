// Program server demonstrates how to set up a JSON-RPC 2.0 server using the
// github.com/creachadair/jrpc2 package.
//
// Usage (see also the client example):
//
//   go build github.com/creachadair/jrpc2/cmd/examples/server
//   ./server -address :8080
//
// See also cmd/examples/client/client.go.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/metrics"
	"github.com/creachadair/jrpc2/server"
)

// The math type defines several arithmetic methods we can expose via the
// service. The exported methods having appropriate types can be automatically
// exposed to the server by jrpc2.NewService.
type math struct{}

// A binop carries a pair of integers for use as parameters.
type binop struct {
	X, Y int
}

// Add returns the sum of vs, or 0 if len(vs) == 0.
func (math) Add(ctx context.Context, vs []int) int {
	sum := 0
	for _, v := range vs {
		sum += v
	}
	return sum
}

// Sub returns the difference arg.X - arg.Y.
func (math) Sub(ctx context.Context, arg binop) int {
	return arg.X - arg.Y
}

// Mul returns the product arg.X * arg.Y.
func (math) Mul(ctx context.Context, arg binop) int {
	return arg.X * arg.Y
}

// Div converts its arguments to floating point and returns their ratio.
func (math) Div(ctx context.Context, arg binop) (float64, error) {
	if arg.Y == 0 {
		return 0, jrpc2.Errorf(code.InvalidParams, "zero divisor")
	}
	return float64(arg.X) / float64(arg.Y), nil
}

// Status simulates a health check, reporting "OK" to all callers.  It also
// demonstrates the use of server-side push.
func (math) Status(ctx context.Context) (string, error) {
	if err := jrpc2.PushNotify(ctx, "pushback", []string{"hello, friend"}); err != nil {
		return "BAD", err
	}
	return "OK", nil
}

// Alert implements a notification handler that logs its argument.
func Alert(ctx context.Context, a map[string]string) error {
	log.Printf("[ALERT]: %s", a["message"])
	return nil
}

var (
	address  = flag.String("address", "", "Service address")
	maxTasks = flag.Int("max", 1, "Maximum concurrent tasks")
)

func main() {
	flag.Parse()
	if *address == "" {
		log.Fatal("You must provide a network -address to listen on")
	}

	// Bind the methods of the math type to an assigner.
	mux := handler.ServiceMap{
		"Math": handler.NewService(math{}),
		"Post": handler.Map{"Alert": handler.New(Alert)},
	}

	lst, err := net.Listen(jrpc2.Network(*address), *address)
	if err != nil {
		log.Fatalln("Listen:", err)
	}
	log.Printf("Listening at %v...", lst.Addr())
	server.Loop(lst, server.NewStatic(mux), &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			Logger:      log.New(os.Stderr, "[jrpc2.Server] ", log.LstdFlags|log.Lshortfile),
			Concurrency: *maxTasks,
			Metrics:     metrics.New(),
			AllowPush:   true,
		},
	})
}
