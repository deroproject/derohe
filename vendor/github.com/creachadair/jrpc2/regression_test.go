// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package jrpc2_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
	"github.com/fortytw2/leaktest"
	"github.com/google/go-cmp/cmp"
)

// Verify that a notification handler will not deadlock with the dispatcher on
// holding the server lock. See: https://github.com/creachadair/jrpc2/issues/27
func TestLockRaceRegression(t *testing.T) {
	defer leaktest.Check(t)()

	hdone := make(chan struct{})
	local := server.NewLocal(handler.Map{
		// Do some busy-work and then try to get the server lock, in this case
		// via the CancelRequest helper.
		"Kill": handler.New(func(ctx context.Context, req *jrpc2.Request) error {
			defer close(hdone) // signal we passed the deadlock point

			var id string
			if err := req.UnmarshalParams(&handler.Args{&id}); err != nil {
				return err
			}
			jrpc2.ServerFromContext(ctx).CancelRequest(id)
			return nil
		}),

		// Block indefinitely, just to give the dispatcher something to do.
		"Stall": handler.New(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}),
	}, nil)
	defer local.Close()

	ctx := context.Background()
	local.Client.Notify(ctx, "Kill", handler.Args{"1"})

	go local.Client.Call(ctx, "Stall", nil)

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Notification handler is probably deadlocked")
	case <-hdone:
		t.Log("Notification handler completed successfully")
	}
}

// Verify that if a callback handler panics, the client will report an error
// back to the server. See https://github.com/creachadair/jrpc2/issues/41.
func TestOnCallbackPanicRegression(t *testing.T) {
	defer leaktest.Check(t)()

	const panicString = "the devil you say"

	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context) error {
			rsp, err := jrpc2.ServerFromContext(ctx).Callback(ctx, "Poke", nil)
			if err == nil {
				t.Errorf("Callback unexpectedly succeeded: %#q", rsp.ResultString())
			} else if !strings.HasSuffix(err.Error(), panicString) {
				t.Errorf("Callback reported unexpected error: %v", err)
			} else {
				t.Logf("Callback reported expected error: %v", err)
			}
			return nil
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{
			AllowPush: true,
		},
		Client: &jrpc2.ClientOptions{
			OnCallback: func(ctx context.Context, req *jrpc2.Request) (interface{}, error) {
				t.Log("Entering callback handler; about to panic")
				panic(panicString)
			},
		},
	})
	defer loc.Close()

	if _, err := loc.Client.Call(context.Background(), "Test", nil); err != nil {
		t.Errorf("Call unexpectedly failed: %v", err)
	}
}

// Verify that a duplicate request ID that arrives while a task is in flight
// does not cause the existing task to be cancelled.
func TestDuplicateIDCancellation(t *testing.T) {
	defer leaktest.Check(t)()

	tctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This channel is closed when the test method is running.
	ready := make(chan struct{})

	cch, sch := channel.Direct()
	srv := jrpc2.NewServer(handler.Map{
		"Test": handler.New(func(ctx context.Context, req *jrpc2.Request) error {
			t.Logf("Test method running, request ID %q", req.ID())
			close(ready)
			select {
			case <-tctx.Done():
				t.Log("Request ending normally (test signal)")
			case <-ctx.Done():
				t.Error("Request was unexpected cancelled by the server")
			}
			return nil
		}),
	}, nil).Start(sch)

	send := func(s string) {
		if err := cch.Send([]byte(s)); err != nil {
			t.Errorf("Send %#q failed: %v", s, err)
		}
	}
	expect := func(s string) {
		bits, err := cch.Recv()
		if err != nil {
			t.Errorf("Recv failed: %v", err)
		} else if got := string(bits); got != s {
			t.Errorf("Recv: got %#q, want %#q", got, s)
		}
	}

	const duplicateReq = `{"jsonrpc":"2.0", "id":1, "method":"Test"}`

	// Send the first request and wait for the handler to start.
	send(duplicateReq)
	<-ready

	// Send the duplicate, which should report an error.
	send(duplicateReq)
	expect(`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"duplicate request ID","data":"1"}}`)

	// Unblock the handler, which should now complete. If the duplicate request
	// caused the handler to cancel, it will have logged an error to fail the test.
	cancel()
	expect(`{"jsonrpc":"2.0","id":1,"result":null}`)

	cch.Close()
	srv.Wait()
}

func TestCheckBatchDuplicateID(t *testing.T) {
	defer leaktest.Check(t)()

	srv, cli := channel.Direct()
	s := jrpc2.NewServer(handler.Map{
		"Test": testOK,
	}, nil).Start(srv)
	defer func() {
		cli.Close()
		if err := s.Wait(); err != nil {
			t.Errorf("Server wait: unexpected error: %v", err)
		}
	}()

	// A batch of requests containing two calls with the same ID.
	const input = `[
  {"jsonrpc": "2.0", "id": 1, "method": "Test"},
  {"jsonrpc": "2.0", "id": 1, "method": "Test"},
  {"jsonrpc": "2.0", "id": 2, "method": "Test"}
]
`
	const errorReply = `{` +
		`"jsonrpc":"2.0",` +
		`"id":1,` +
		`"error":{"code":-32600,"message":"duplicate request ID","data":"1"}` +
		`}`
	const want = `[` + errorReply + `,` + errorReply + `,` + `{"jsonrpc":"2.0","id":2,"result":"OK"}]`

	if err := cli.Send([]byte(input)); err != nil {
		t.Fatalf("Send %d bytes failed: %v", len(input), err)
	}
	rsp, err := cli.Recv()
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}
	if diff := cmp.Diff(want, string(rsp)); diff != "" {
		t.Errorf("Server response: (-want, +got)\n%s", diff)
	}
}
