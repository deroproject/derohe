// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package server_test

import (
	"context"
	"flag"
	"sync"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
	"github.com/fortytw2/leaktest"
)

var doDebug = flag.Bool("debug", false, "Enable server and client debugging logs")

func testOpts(t *testing.T) *server.LocalOptions {
	if !*doDebug {
		return nil
	}
	return &server.LocalOptions{
		Client: &jrpc2.ClientOptions{Logger: func(s string) { t.Log(s) }},
		Server: &jrpc2.ServerOptions{Logger: func(s string) { t.Log(s) }},
	}
}

func TestLocal(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(make(handler.Map), testOpts(t))
	ctx := context.Background()
	si, err := jrpc2.RPCServerInfo(ctx, loc.Client)
	if err != nil {
		t.Fatalf("rpc.serverInfo failed: %v", err)
	}

	// A couple coherence checks on the server info.
	if nr := si.Counter["rpc.requests"]; nr != 1 {
		t.Errorf("rpc.serverInfo reports %d requests, wanted 1", nr)
	}
	if len(si.Methods) != 0 {
		t.Errorf("rpc.serverInfo reports methods %+q, wanted []", si.Methods)
	}

	// Close the client and wait for the server to stop.
	if err := loc.Close(); err != nil {
		t.Errorf("Server wait: got %v, want nil", err)
	}
}

// Test that concurrent callers to a local service do not deadlock.
func TestLocalConcurrent(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(context.Context) error { return nil }),
	}, testOpts(t))

	const numCallers = 20

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(numCallers)
	for i := 0; i < numCallers; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := loc.Client.Call(ctx, "Test", nil)
			if err != nil {
				t.Errorf("Caller %d failed: %v", i, err)
			}
		}()
	}
	wg.Wait()
	if err := loc.Close(); err != nil {
		t.Errorf("Server close: %v", err)
	}
}
