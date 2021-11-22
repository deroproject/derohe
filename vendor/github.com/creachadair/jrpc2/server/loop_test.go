package server

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
)

var newChan = channel.Line

// A static test service that returns the same thing each time.
var testService = Static(handler.Map{
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

func newTestSession(t *testing.T) func() Service {
	return func() Service { t.Helper(); return &testSession{t: t} }
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

func mustServe(t *testing.T, lst net.Listener, newService func() Service) <-chan struct{} {
	t.Helper()

	sc := make(chan struct{})
	go func() {
		defer close(sc)
		// Start a server loop to accept connections from the clients. This should
		// exit cleanly once all the clients have finished and the listener closes.
		lst := NetAccepter(lst, newChan)
		if err := Loop(lst, newService, nil); err != nil {
			t.Errorf("Loop: unexpected failure: %v", err)
		}
	}()
	return sc
}

// Test that sequential clients against the same server work sanely.
func TestSeq(t *testing.T) {
	lst := mustListen(t)
	addr := lst.Addr().String()
	sc := mustServe(t, lst, testService)

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
	<-sc
}

// Test that concurrent clients against the same server work sanely.
func TestLoop(t *testing.T) {
	tests := []struct {
		desc string
		cons func() Service
	}{
		{"StaticService", testService},
		{"SessionStateService", newTestSession(t)},
	}
	const numClients = 5
	const numCalls = 5

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			lst := mustListen(t)
			addr := lst.Addr().String()
			sc := mustServe(t, lst, test.cons)

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
			<-sc
		})
	}
}
