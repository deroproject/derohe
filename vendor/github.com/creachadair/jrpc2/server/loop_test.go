// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package server_test

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
	"github.com/fortytw2/leaktest"
)

var newChan = channel.Line

// A static test service that returns the same thing each time.
var testStatic = server.Static(handler.Map{
	"Test": handler.New(func(context.Context) (string, error) {
		return "OK", nil
	}),
})

// A service with session state, to exercise start/finish plumbing.
type testSession struct {
	t     *testing.T
	init  bool
	nCall int
}

func newTestSession(t *testing.T) func() server.Service {
	return func() server.Service { t.Helper(); return &testSession{t: t} }
}

func (t *testSession) Assigner() (jrpc2.Assigner, error) {
	if t.init {
		t.t.Error("Service has already been initialized")
	}
	t.init = true
	return handler.Map{
		"Test": handler.New(func(context.Context) (string, error) {
			if !t.init {
				t.t.Error("Handler called before service initialized")
			}
			t.nCall++
			return "OK", nil
		}),
	}, nil
}

func (t *testSession) Finish(assigner jrpc2.Assigner, stat jrpc2.ServerStatus) {
	if _, ok := assigner.(handler.Map); !ok {
		t.t.Errorf("Finished assigner: got %+v, want handler.Map", assigner)
	}
	if !t.init {
		t.t.Error("Service finished without being initialized")
	}
	if !stat.Success() {
		t.t.Errorf("Finish unsuccessful: %v", stat.Err)
	}
}

func mustListen(t *testing.T) net.Listener {
	t.Helper()

	lst, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	return lst
}

func mustDial(t *testing.T, addr string) *jrpc2.Client {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial %q: %v", addr, err)
	}
	return jrpc2.NewClient(newChan(conn, conn), nil)
}

func mustServe(t *testing.T, ctx context.Context, lst net.Listener, newService func() server.Service) <-chan error {
	t.Helper()

	acc := server.NetAccepter(lst, newChan)
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		// Start a server loop to accept connections from the clients. This should
		// exit cleanly once all the clients have finished and the listener closes.
		errc <- server.Loop(ctx, acc, newService, nil)
	}()
	return errc
}

// Test that sequential clients against the same server work sanely.
func TestSeq(t *testing.T) {
	defer leaktest.Check(t)()

	lst := mustListen(t)
	addr := lst.Addr().String()
	errc := mustServe(t, context.Background(), lst, testStatic)

	for i := 0; i < 5; i++ {
		cli := mustDial(t, addr)
		var rsp string
		if err := cli.CallResult(context.Background(), "Test", nil, &rsp); err != nil {
			t.Errorf("[client %d] Test call: unexpected error: %v", i, err)
		} else if rsp != "OK" {
			t.Errorf("[client %d]: Test call: got %q, want OK", i, rsp)
		}
		cli.Close()
	}
	lst.Close()
	if err := <-errc; err != nil {
		t.Errorf("Server exit failed: %v", err)
	}
}

// Test that context plumbing works properly.
func TestLoop_cancelContext(t *testing.T) {
	defer leaktest.Check(t)()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lst := mustListen(t)
	defer lst.Close()
	errc := mustServe(t, ctx, lst, testStatic)

	time.AfterFunc(50*time.Millisecond, cancel)
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("Loop exit reported error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("Loop did not exit in a timely manner after cancellation")
	}
}

// Test that cancelling a loop stops its servers.
func TestLoop_cancelServers(t *testing.T) {
	defer leaktest.Check(t)()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{})

	lst := mustListen(t)
	errc := mustServe(t, ctx, lst, server.Static(handler.Map{
		"Test": handler.New(func(ctx context.Context) error {
			// Signal readiness then block until cancelled.
			// The server will cancel this method when stopped.
			close(ready)
			<-ctx.Done()
			return ctx.Err()
		}),
	}))
	cli := mustDial(t, lst.Addr().String())
	defer cli.Close()

	// Issue a call to the server that will block until the server cancels the
	// handler at shutdown. If the server blocks after cancellation, it means it
	// is not correctly stopping its active servers.
	go cli.Call(context.Background(), "Test", nil)

	<-ready
	cancel() // this should stop the loop and the server

	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("Loop result: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Loop did not exit in a timely manner after cancellation")
	}
}

// Test that concurrent clients against the same server work sanely.
func TestLoop(t *testing.T) {
	defer leaktest.Check(t)()

	tests := []struct {
		desc string
		cons func() server.Service
	}{
		{"StaticService", testStatic},
		{"SessionStateService", newTestSession(t)},
	}
	const numClients = 5
	const numCalls = 5

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			lst := mustListen(t)
			addr := lst.Addr().String()
			errc := mustServe(t, context.Background(), lst, test.cons)

			// Start a bunch of clients, each of which will dial the server and make
			// some calls at random intervals to tickle the race detector.
			var wg sync.WaitGroup
			for i := 0; i < numClients; i++ {
				wg.Add(1)
				i := i
				go func() {
					defer wg.Done()
					cli := mustDial(t, addr)
					defer cli.Close()

					for j := 0; j < numCalls; j++ {
						time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
						var rsp string
						if err := cli.CallResult(context.Background(), "Test", nil, &rsp); err != nil {
							t.Errorf("[client %d]: Test call %d: unexpected error: %v", i, j+1, err)
						} else if rsp != "OK" {
							t.Errorf("[client %d]: Test call %d: got %q, want OK", i, j+1, rsp)
						}
					}
				}()
			}

			// Wait for the clients to be finished and then close the listener so that
			// the service loop will stop.
			wg.Wait()
			lst.Close()
			if err := <-errc; err != nil {
				t.Errorf("Server exit failed: %v", err)
			}
		})
	}
}
