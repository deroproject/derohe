// Program server demonstrates how to set up a JSON-RPC 2.0 server using the
// github.com/creachadair/jrpc2 package.
//
// Usage (see also the client example):
//
//   go build github.com/creachadair/jrpc2/tools/examples/server
//   ./server -address :8080
//
// See also examples/client/client.go.
package main

import (
	"context"
	"flag"
	"log"
	"net"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/metrics"
	"github.com/creachadair/jrpc2/server"
)

// A binop carries a pair of integers for use as parameters.
type binop struct {
	X, Y int
}

// Add returns the sum of vs, or 0 if len(vs) == 0.
func Add(ctx context.Context, vs []int) int {
	sum := 0
	for _, v := range vs {
		sum += v
	}
	return sum
}

// Sub returns the difference x - y.
func Sub(ctx context.Context, x, y int) int { return x - y }

// Mul returns the product arg.X * arg.Y.
func Mul(ctx context.Context, x, y int) int { return x * y }

// Div converts its arguments to floating point and returns their ratio.
func Div(ctx context.Context, arg binop) (float64, error) {
	if arg.Y == 0 {
		return 0, jrpc2.Errorf(code.InvalidParams, "zero divisor")
	}
	return float64(arg.X) / float64(arg.Y), nil
}

// Status simulates a health check, reporting "OK" to all callers.  It also
// demonstrates the use of server-side push.
func Status(ctx context.Context) (string, error) {
	if err := jrpc2.ServerFromContext(ctx).Notify(ctx, "pushback", []string{"hello, friend"}); err != nil {
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

	// Bind the services to their given names.
	mux := handler.ServiceMap{
		"Math": handler.Map{
			"Add":    handler.New(Add),
			"Sub":    handler.NewPos(Sub, "X", "Y"),
			"Mul":    handler.NewPos(Mul, "X", "Y"),
			"Div":    handler.New(Div),
			"Status": handler.New(Status),
		},
		"Post": handler.Map{
			"Alert": handler.New(Alert),
		},
	}

	lst, err := net.Listen(jrpc2.Network(*address))
	if err != nil {
		log.Fatalln("Listen:", err)
	}
	log.Printf("Listening at %v...", lst.Addr())
	acc := server.NetAccepter(lst, channel.Line)
	server.Loop(acc, server.Static(mux), &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			Logger:      jrpc2.StdLogger(nil),
			Concurrency: *maxTasks,
			Metrics:     metrics.New(),
			AllowPush:   true,
		},
	})
}
