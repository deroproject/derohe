package jrpc2_test

import (
	"context"
	"testing"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
)

// Verify that a notification handler will not deadlock with the dispatcher on
// holding the server lock. See: https://github.com/creachadair/jrpc2/issues/27
func TestLockRaceRegression(t *testing.T) {
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
			jrpc2.CancelRequest(ctx, id)
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
