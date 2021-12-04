package jrpc2_test

import (
	"context"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/server"
)

func BenchmarkRoundTrip(b *testing.B) {
	// Benchmark the round-trip call cycle for a method that does no useful
	// work, as a proxy for overhead for client and server maintenance.
	voidService := handler.Map{
		"void": handler.Func(func(context.Context, *jrpc2.Request) (interface{}, error) {
			return nil, nil
		}),
	}
	ctxClient := &jrpc2.ClientOptions{EncodeContext: jctx.Encode}
	tests := []struct {
		desc string
		cli  *jrpc2.ClientOptions
		srv  *jrpc2.ServerOptions
	}{
		{"C01-CTX-B", nil, &jrpc2.ServerOptions{DisableBuiltin: true, Concurrency: 1}},
		{"C01-CTX+B", nil, &jrpc2.ServerOptions{Concurrency: 1}},
		{"C04-CTX-B", nil, &jrpc2.ServerOptions{DisableBuiltin: true, Concurrency: 4}},
		{"C04-CTX+B", nil, &jrpc2.ServerOptions{Concurrency: 4}},
		{"C12-CTX-B", nil, &jrpc2.ServerOptions{DisableBuiltin: true, Concurrency: 12}},
		{"C12-CTX+B", nil, &jrpc2.ServerOptions{Concurrency: 12}},

		{"C01+CTX-B", ctxClient,
			&jrpc2.ServerOptions{DecodeContext: jctx.Decode, DisableBuiltin: true, Concurrency: 1},
		},
		{"C01+CTX+B", ctxClient,
			&jrpc2.ServerOptions{DecodeContext: jctx.Decode, Concurrency: 1},
		},
		{"C04+CTX-B", ctxClient,
			&jrpc2.ServerOptions{DecodeContext: jctx.Decode, DisableBuiltin: true, Concurrency: 4},
		},
		{"C04+CTX+B", ctxClient,
			&jrpc2.ServerOptions{DecodeContext: jctx.Decode, Concurrency: 4},
		},
		{"C12+CTX-B", ctxClient,
			&jrpc2.ServerOptions{DecodeContext: jctx.Decode, DisableBuiltin: true, Concurrency: 4},
		},
		{"C12+CTX+B", ctxClient,
			&jrpc2.ServerOptions{DecodeContext: jctx.Decode, Concurrency: 12},
		},
	}
	for _, test := range tests {
		b.Run(test.desc, func(b *testing.B) {
			loc := server.NewLocal(voidService, &server.LocalOptions{
				Client: test.cli,
				Server: test.srv,
			})
			defer loc.Close()
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := loc.Client.Call(ctx, "void", nil); err != nil {
					b.Fatalf("Call void failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkParseRequests(b *testing.B) {
	reqs := []struct {
		desc, input string
	}{
		{"Minimal", `{"jsonrpc":"2.0","id":1,"method":"Foo.Bar","params":null}`},
		{"Medium", `{
  "jsonrpc": "2.0",
  "id": 23593,
  "method": "Four square meals in one day",
  "params": [
     "year",
     1994,
     {"month": "July", "day": 26},
     true
  ]
}`},
		{"Batch", `[{"jsonrpc":"2.0","id":1,"method":"Abel","params":[1,3,5]},
        {"jsonrpc":"2.0","id":2,"method":"Baker","params":{"x":99}},
        {"jsonrpc":"2.0","id":3,"method":"Charlie","params":["foo",19,true]},
        {"jsonrpc":"2.0","id":4,"method":"Delta","params":{}},
        {"jsonrpc":"2.0","id":5,"method":"Echo","params":[]}]`},
	}
	for _, req := range reqs {
		msg := []byte(req.input)
		b.Run(req.desc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := jrpc2.ParseRequests(msg)
				if err != nil {
					b.Fatalf("ParseRequests %#q failed: %v", req.input, err)
				}
			}
		})
	}
}
