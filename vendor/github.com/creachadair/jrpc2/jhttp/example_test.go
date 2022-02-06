// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package jhttp_test

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"strings"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jhttp"
)

func Example() {
	// Set up a bridge exporting a simple service.
	b := jhttp.NewBridge(handler.Map{
		"Test": handler.New(func(ctx context.Context, ss []string) string {
			return strings.Join(ss, " ")
		}),
	}, nil)
	defer b.Close()

	// The bridge can be used as the handler for an HTTP server.
	hsrv := httptest.NewServer(b)
	defer hsrv.Close()

	// Set up a client using an HTTP channel, and use it to call the test
	// service exported by the bridge.
	ch := jhttp.NewChannel(hsrv.URL, nil)
	cli := jrpc2.NewClient(ch, nil)

	var result string
	if err := cli.CallResult(context.Background(), "Test", []string{
		"full", "plate", "and", "packing", "steel",
	}, &result); err != nil {
		log.Fatalf("Call failed: %v", err)
	}

	fmt.Println("Result:", result)
	// Output:
	// Result: full plate and packing steel
}
