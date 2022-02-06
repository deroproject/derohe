// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package jrpc2_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/server"
	"github.com/fortytw2/leaktest"
	"github.com/google/go-cmp/cmp"
)

// Static type assertions.
var (
	_ code.ErrCoder = (*jrpc2.Error)(nil)
)

var testOK = handler.New(func(ctx context.Context) (string, error) {
	return "OK", nil
})

var testService = handler.Map{
	// Verify that we can bind methods of a value.
	"Add": handler.New((dummy{}).Add),
	"Mul": handler.New((dummy{}).Mul),
	"Max": handler.New((dummy{}).Max),

	// Verify that we can bind free functions.
	"Nil":  handler.New(methodNil),
	"Ctx":  handler.New(methodCtx),
	"Ping": handler.New(methodPing),
}

type dummy struct{}

// Add is a request-based method.
func (dummy) Add(_ context.Context, req *jrpc2.Request) (interface{}, error) {
	if req.IsNotification() {
		return nil, errors.New("ignoring notification")
	}
	var vals []int
	if err := req.UnmarshalParams(&vals); err != nil {
		return nil, err
	}
	var sum int
	for _, v := range vals {
		sum += v
	}
	return sum, nil
}

// Mul uses its own explicit parameter type.
func (dummy) Mul(_ context.Context, req struct{ X, Y int }) (int, error) {
	return req.X * req.Y, nil
}

// Max takes a slice of arguments.
func (dummy) Max(_ context.Context, vs []int) (int, error) {
	if len(vs) == 0 {
		return 0, jrpc2.Errorf(code.InvalidParams, "cannot compute max of no elements")
	}
	max := vs[0]
	for _, v := range vs[1:] {
		if v > max {
			max = v
		}
	}
	return max, nil
}

// methodNil does not require any parameters.
func methodNil(_ context.Context) (int, error) { return 42, nil }

// methodCtx validates that its context includes the request.
func methodCtx(ctx context.Context, req *jrpc2.Request) (int, error) {
	if creq := jrpc2.InboundRequest(ctx); creq != req {
		return 0, fmt.Errorf("wrong req in context %p ≠ %p", creq, req)
	}
	return 1, nil
}

// methodPing responds only to notifications.
func methodPing(ctx context.Context, req *jrpc2.Request) error {
	if !req.IsNotification() {
		return errors.New("called Ping expecting a response")
	}
	return nil
}

var callTests = []struct {
	method string
	params interface{}
	want   int
}{
	{"Test.Add", []int{}, 0},
	{"Test.Add", []int{1, 2, 3}, 6},
	{"Test.Mul", struct{ X, Y int }{7, 9}, 63},
	{"Test.Mul", struct{ X, Y int }{}, 0},
	{"Test.Max", []int{3, 1, 8, 4, 2, 0, -5}, 8},
	{"Test.Ctx", nil, 1},
	{"Test.Nil", nil, 42},
	{"Test.Nil", json.RawMessage("null"), 42},
}

func TestServerInfo_methodNames(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.ServiceMap{
		"Test": testService,
	}, nil)
	defer loc.Close()
	s := loc.Server

	// Verify that the assigner got the names it was supposed to.
	got, want := s.ServerInfo().Methods, []string{
		"Test.Add", "Test.Ctx", "Test.Max", "Test.Mul", "Test.Nil", "Test.Ping",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Wrong method names: (-want, +got)\n%s", diff)
	}
}

func TestClient_Call(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.ServiceMap{
		"Test": testService,
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{Concurrency: 16},
	})
	defer loc.Close()
	c := loc.Client
	ctx := context.Background()

	// Verify that individual sequential requests work.
	for _, test := range callTests {
		rsp, err := c.Call(ctx, test.method, test.params)
		if err != nil {
			t.Errorf("Call %q %v: unexpected error: %v", test.method, test.params, err)
			continue
		}
		var got int
		if err := rsp.UnmarshalResult(&got); err != nil {
			t.Errorf("Unmarshaling result: %v", err)
			continue
		}
		if got != test.want {
			t.Errorf("Call %q %v: got %v, want %v", test.method, test.params, got, test.want)
		}
		if err := c.Notify(ctx, test.method, test.params); err != nil {
			t.Errorf("Notify %q %v: unexpected error: %v", test.method, test.params, err)
		}
	}
}

func TestClient_CallResult(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.ServiceMap{
		"Test": testService,
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{Concurrency: 16},
	})
	defer loc.Close()
	c := loc.Client
	ctx := context.Background()

	// Verify also that the CallResult wrapper works.
	for _, test := range callTests {
		var got int
		if err := c.CallResult(ctx, test.method, test.params, &got); err != nil {
			t.Errorf("CallResult %q %v: unexpected error: %v", test.method, test.params, err)
			continue
		}
		if got != test.want {
			t.Errorf("CallResult %q %v: got %v, want %v", test.method, test.params, got, test.want)
		}
	}
}

func TestClient_Batch(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.ServiceMap{
		"Test": testService,
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{Concurrency: 16},
	})
	defer loc.Close()
	c := loc.Client
	ctx := context.Background()

	// Verify that a batch request works.
	specs := make([]jrpc2.Spec, len(callTests)+1)
	specs[0] = jrpc2.Spec{
		Method: "Test.Ping",
		Params: []string{"hey"},
		Notify: true,
	}
	for i, test := range callTests {
		specs[i+1] = jrpc2.Spec{
			Method: test.method,
			Params: test.params,
			Notify: false,
		}
	}
	batch, err := c.Batch(ctx, specs)
	if err != nil {
		t.Fatalf("Batch failed: %v", err)
	}
	if len(batch) != len(callTests) {
		t.Errorf("Wrong number of responses: got %d, want %d", len(batch), len(callTests))
	}
	for i, rsp := range batch {
		if err := rsp.Error(); err != nil {
			t.Errorf("Response %d failed: %v", i+1, err)
			continue
		}
		var got int
		if err := rsp.UnmarshalResult(&got); err != nil {
			t.Errorf("Umarshaling result %d: %v", i+1, err)
			continue
		}
		if got != callTests[i].want {
			t.Errorf("Response %d (%q): got %v, want %v", i+1, rsp.ID(), got, callTests[i].want)
		}
	}
}

// Verify that notifications respect order of arrival.
func TestServer_notificationOrder(t *testing.T) {
	defer leaktest.Check(t)()

	var last int32

	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(_ context.Context, req *jrpc2.Request) error {
			var seq int32
			if err := req.UnmarshalParams(&handler.Args{&seq}); err != nil {
				t.Errorf("Invalid test parameters: %v", err)
				return err
			}
			if old := atomic.SwapInt32(&last, seq); old != seq-1 {
				t.Errorf("Request out of sequence at #%d: got %d, want %d", seq, old, seq-1)
			}
			return nil
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{Concurrency: 16},
	})

	for i := 1; i < 10; i++ {
		if err := loc.Client.Notify(context.Background(), "Test", []int{i}); err != nil {
			t.Errorf("Test notification failed: %v", err)
		}
	}
	if err := loc.Close(); err != nil {
		t.Logf("Warning: error at server exit: %v", err)
	}
}

// Verify that a method that returns only an error (no result payload) is set
// up and handled correctly.
func TestHandler_errorOnly(t *testing.T) {
	defer leaktest.Check(t)()

	const errMessage = "not enough strings"
	loc := server.NewLocal(handler.Map{
		"ErrorOnly": handler.New(func(_ context.Context, ss []string) error {
			if len(ss) == 0 {
				return jrpc2.Errorf(1, errMessage)
			}
			t.Logf("ErrorOnly succeeds on input %q", ss)
			return nil
		}),
	}, nil)
	defer loc.Close()
	c := loc.Client
	ctx := context.Background()

	t.Run("CallExpectingError", func(t *testing.T) {
		rsp, err := c.Call(ctx, "ErrorOnly", []string{})
		if err == nil {
			t.Errorf("ErrorOnly: got %+v, want error", rsp)
		} else if e, ok := err.(*jrpc2.Error); !ok {
			t.Errorf("ErrorOnly: got %v, want *Error", err)
		} else if e.Code != 1 || e.Message != errMessage {
			t.Errorf("ErrorOnly: got (%s, %s), want (1, %s)", e.Code, e.Message, errMessage)
		}
	})
	t.Run("CallExpectingOK", func(t *testing.T) {
		rsp, err := c.Call(ctx, "ErrorOnly", []string{"aiutami!"})
		if err != nil {
			t.Errorf("ErrorOnly: unexpected error: %v", err)
		}
		// Per https://www.jsonrpc.org/specification#response_object, a "result"
		// field is required on success, so verify that it is set null.
		var got json.RawMessage
		if err := rsp.UnmarshalResult(&got); err != nil {
			t.Fatalf("Failed to unmarshal result data: %v", err)
		} else if r := string(got); r != "null" {
			t.Errorf("ErrorOnly response: got %q, want null", r)
		}
	})
}

// Verify that a timeout set on the client context is respected and reports
// back to the caller as an error.
func TestClient_contextTimeout(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{
		"Stall": handler.New(func(ctx context.Context) (bool, error) {
			t.Log("Stalling...")
			select {
			case <-ctx.Done():
				t.Logf("Stall context done: err=%v", ctx.Err())
				return true, nil
			case <-time.After(5 * time.Second):
				return false, errors.New("stall timed out")
			}
		}),
	}, nil)
	defer loc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	got, err := loc.Client.Call(ctx, "Stall", nil)
	if err == nil {
		t.Errorf("Stall: got %+v, wanted error", got)
	} else if err != context.DeadlineExceeded {
		t.Errorf("Stall: got error %v, want %v", err, context.DeadlineExceeded)
	} else {
		t.Logf("Successfully cancelled after %v", time.Since(start))
	}
}

// Verify that stopping the server terminates in-flight requests.
func TestServer_stopCancelsHandlers(t *testing.T) {
	defer leaktest.Check(t)()

	started := make(chan struct{})
	stopped := make(chan error, 1)
	loc := server.NewLocal(handler.Map{
		"Hang": handler.New(func(ctx context.Context) (bool, error) {
			close(started) // signal that the method handler is running
			<-ctx.Done()
			return true, ctx.Err()
		}),
	}, nil)
	defer loc.Close()
	s, c := loc.Server, loc.Client

	// Call the server. The method will hang until its context is cancelled,
	// which should happen when the server stops.
	go func() {
		defer close(stopped)
		_, err := c.Call(context.Background(), "Hang", nil)
		stopped <- err
	}()

	// Wait until the client method is running so we know we are testing at the
	// right time, i.e., with a request in flight.
	<-started
	s.Stop()
	select {
	case <-time.After(30 * time.Second):
		t.Error("Timed out waiting for service handler to fail")
	case err := <-stopped:
		if ec := code.FromError(err); ec != code.Cancelled {
			t.Errorf("Client error: got %v (%v), wanted code %v", err, ec, code.Cancelled)
		}
	}
}

// Test that a handler can cancel an in-flight request.
func TestServer_CancelRequest(t *testing.T) {
	defer leaktest.Check(t)()

	ready := make(chan struct{})
	loc := server.NewLocal(handler.Map{
		"Stall": handler.New(func(ctx context.Context) error {
			close(ready)
			t.Log("Stall handler: waiting for context cancellation")
			<-ctx.Done()
			return ctx.Err()
		}),
		"Test": handler.New(func(ctx context.Context, req *jrpc2.Request) error {
			var id string
			if err := req.UnmarshalParams(&handler.Args{&id}); err != nil {
				return err
			}
			t.Logf("Test handler: cancelling %q...", id)
			jrpc2.ServerFromContext(ctx).CancelRequest(id)
			return nil
		}),
	}, nil)
	defer loc.Close()

	ctx := context.Background()

	// Start a call in the background that will stall until cancelled.
	errc := make(chan error, 1)
	go func() {
		_, err := loc.Client.Call(ctx, "Stall", nil)
		errc <- err
		close(errc)
	}()

	// Wait until the handler is in progress.
	<-ready

	// Call the test method to cancel the stalled method, and verify that we got
	// back the expected error.
	if _, err := loc.Client.Call(ctx, "Test", []string{"1"}); err != nil {
		t.Errorf("Test call failed: %v", err)
	}

	err := <-errc
	got := code.FromError(err)
	if got != code.Cancelled {
		t.Errorf("Stall: got %v (%v), want %v", err, got, code.Cancelled)
	} else {
		t.Logf("Cancellation succeeded, got expected error: %v", err)
	}
}

// Test that an error with data attached to it is correctly propagated back
// from the server to the client, in a value of concrete type *Error.
func TestError_withData(t *testing.T) {
	defer leaktest.Check(t)()

	const errCode = -32000
	const errData = `{"caroline":452}`
	const errMessage = "error thingy"
	loc := server.NewLocal(handler.Map{
		"Err": handler.New(func(_ context.Context) (int, error) {
			return 17, jrpc2.Errorf(errCode, errMessage).WithData(json.RawMessage(errData))
		}),
		"Push": handler.New(func(ctx context.Context) (bool, error) {
			return false, jrpc2.ServerFromContext(ctx).Notify(ctx, "PushBack", nil)
		}),
		"Code": handler.New(func(ctx context.Context) error {
			return code.Code(12345).Err()
		}),
	}, &server.LocalOptions{
		Client: &jrpc2.ClientOptions{
			OnNotify: func(req *jrpc2.Request) {
				t.Errorf("Client received unexpected push: %#v", req)
			},
		},
	})
	defer loc.Close()
	c := loc.Client

	if got, err := c.Call(context.Background(), "Err", nil); err == nil {
		t.Errorf("Call(Push): got %#v, wanted error", got)
	} else if e, ok := err.(*jrpc2.Error); ok {
		if e.Code != errCode {
			t.Errorf("Error code: got %d, want %d", e.Code, errCode)
		}
		if e.Message != errMessage {
			t.Errorf("Error message: got %q, want %q", e.Message, errMessage)
		}
		if s := string(e.Data); s != errData {
			t.Errorf("Error data: got %q, want %q", s, errData)
		}
	} else {
		t.Fatalf("Call(Err): unexpected error: %v", err)
	}

	if got, err := c.Call(context.Background(), "Push", nil); err == nil {
		t.Errorf("Call(Push): got %#v, wanted error", got)
	}

	if got, err := c.Call(context.Background(), "Code", nil); err == nil {
		t.Errorf("Call(Code): got %#v, wanted error", got)
	} else if s, exp := err.Error(), "[12345] error code 12345"; s != exp {
		t.Errorf("Call(Code): got error %q, want %q", s, exp)
	}
}

// Test that a client correctly reports bad parameters.
func TestClient_badCallParams(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(_ context.Context, v interface{}) error {
			return jrpc2.Errorf(129, "this should not be reached")
		}),
	}, nil)
	defer loc.Close()

	rsp, err := loc.Client.Call(context.Background(), "Test", "bogus")
	if err == nil {
		t.Errorf("Call(Test): got %+v, wanted error", rsp)
	} else if got, want := code.FromError(err), code.InvalidRequest; got != want {
		t.Errorf("Call(Test): got code %v, want %v", got, want)
	}
}

// Verify that metrics are correctly propagated to server info.
func TestServer_serverInfoMetrics(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{
		"Metricize": handler.New(func(ctx context.Context) (bool, error) {
			m := jrpc2.ServerFromContext(ctx).Metrics()
			if m == nil {
				t.Error("Request context does not contain a metrics writer")
				return false, nil
			}
			m.Count("counters-written", 1)
			m.Count("counters-written", 2)

			// Max value trackers are not accumulative.
			m.SetMaxValue("max-metric-value", 1)
			m.SetMaxValue("max-metric-value", 5)
			m.SetMaxValue("max-metric-value", 3)
			m.SetMaxValue("max-metric-value", -30337)

			// Counters are accumulative, and negative deltas subtract.
			m.Count("zero-sum", 0)
			m.Count("zero-sum", 15)
			m.Count("zero-sum", -16)
			m.Count("zero-sum", 1)
			return true, nil
		}),
	}, nil)
	s, c := loc.Server, loc.Client

	ctx := context.Background()
	if _, err := c.Call(ctx, "Metricize", nil); err != nil {
		t.Fatalf("Call(Metricize) failed: %v", err)
	}
	if got := s.ServerInfo().Counter["rpc.serversActive"]; got != 1 {
		t.Errorf("Metric rpc.serversActive: got %d, want 1", got)
	}
	loc.Close()

	info := s.ServerInfo()
	tests := []struct {
		input map[string]int64
		name  string
		want  int64 // use < 0 to test for existence only
	}{
		{info.Counter, "rpc.requests", 1},
		{info.Counter, "counters-written", 3},
		{info.Counter, "zero-sum", 0},
		{info.Counter, "rpc.bytesRead", -1},
		{info.Counter, "rpc.bytesWritten", -1},
		{info.Counter, "rpc.serversActive", 0},
		{info.MaxValue, "max-metric-value", 5},
		{info.MaxValue, "rpc.bytesRead", -1},
		{info.MaxValue, "rpc.bytesWritten", -1},
	}
	for _, test := range tests {
		got, ok := test.input[test.name]
		if !ok {
			t.Errorf("Metric %q is not defined, but was expected", test.name)
			continue
		}
		if test.want >= 0 && got != test.want {
			t.Errorf("Wrong value for metric %q: got %d, want %d", test.name, got, test.want)
		}
	}
}

// Ensure that a correct request not sent via the *Client type will still
// elicit a correct response from the server. Here we simulate a "different"
// client by writing requests directly into the channel.
func TestServer_nonLibraryClient(t *testing.T) {
	defer leaktest.Check(t)()

	srv, cli := channel.Direct()
	s := jrpc2.NewServer(handler.Map{
		"X": testOK,
		"Y": handler.New(func(context.Context) (interface{}, error) {
			return nil, nil
		}),
	}, nil).Start(srv)
	defer func() {
		cli.Close()
		if err := s.Wait(); err != nil {
			t.Errorf("Server wait: unexpected error %v", err)
		}
	}()

	const invalidIDMessage = `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid request ID"}}`
	tests := []struct {
		input, want string
	}{
		// Missing version marker (and therefore wrong).
		{`{"id":0}`,
			`{"jsonrpc":"2.0","id":0,"error":{"code":-32600,"message":"incorrect version marker"}}`},

		// Version marker is present, but wrong.
		{`{"jsonrpc":"1.5","id":1}`,
			`{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"incorrect version marker"}}`},

		// No method was specified.
		{`{"jsonrpc":"2.0","id":2}`,
			`{"jsonrpc":"2.0","id":2,"error":{"code":-32600,"message":"empty method name"}}`},

		// The method specified doesn't exist.
		{`{"jsonrpc":"2.0", "id": 3, "method": "NoneSuch"}`,
			`{"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"no such method","data":"NoneSuch"}}`},

		// The parameters are of the wrong form.
		{`{"jsonrpc":"2.0", "id": 4, "method": "X", "params": "bogus"}`,
			`{"jsonrpc":"2.0","id":4,"error":{"code":-32600,"message":"parameters must be array or object"}}`},

		// The parameters are absent, but as null.
		{`{"jsonrpc": "2.0", "id": 6, "method": "X", "params": null}`,
			`{"jsonrpc":"2.0","id":6,"result":"OK"}`},

		// Correct requests.
		{`{"jsonrpc":"2.0","id": 5, "method": "X"}`, `{"jsonrpc":"2.0","id":5,"result":"OK"}`},
		{`{"jsonrpc":"2.0","id":21,"method":"Y"}`, `{"jsonrpc":"2.0","id":21,"result":null}`},
		{`{"jsonrpc":"2.0","id":0,"method":"X"}`, `{"jsonrpc":"2.0","id":0,"result":"OK"}`},
		{`{"jsonrpc":"2.0","id":-0,"method":"X"}`, `{"jsonrpc":"2.0","id":-0,"result":"OK"}`},
		{`{"jsonrpc":"2.0","id":-1,"method":"X"}`, `{"jsonrpc":"2.0","id":-1,"result":"OK"}`},
		{`{"jsonrpc":"2.0","id":-600,"method":"Y"}`, `{"jsonrpc":"2.0","id":-600,"result":null}`},

		// A batch of correct requests.
		{`[{"jsonrpc":"2.0", "id":"a1", "method":"X"}, {"jsonrpc":"2.0", "id":"a2", "method": "X"}]`,
			`[{"jsonrpc":"2.0","id":"a1","result":"OK"},{"jsonrpc":"2.0","id":"a2","result":"OK"}]`},
		{`{"jsonrpc":"2.0", "id":-25, "method":"X"}`, `{"jsonrpc":"2.0","id":-25,"result":"OK"}`},

		// Extra fields on an otherwise-correct request.
		{`{"jsonrpc":"2.0","id": 7, "method": "Z", "params":[], "bogus":true}`,
			`{"jsonrpc":"2.0","id":7,"error":{"code":-32600,"message":"extra fields in request","data":["bogus"]}}`},

		// An empty batch request should report a single error object.
		{`[]`, `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"empty request batch"}}`},

		// An invalid batch request should report a single error object.
		{`[1]`, `[{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"request is not a JSON object"}}]`},

		// A batch of invalid requests returns a batch of errors.
		{`[{"jsonrpc": "2.0", "id": 6, "method":"bogus"}]`,
			`[{"jsonrpc":"2.0","id":6,"error":{"code":-32601,"message":"no such method","data":"bogus"}}]`},

		// Batch requests return batch responses, even for a singleton.
		{`[{"jsonrpc": "2.0", "id": 7, "method": "X"}]`, `[{"jsonrpc":"2.0","id":7,"result":"OK"}]`},

		// Notifications are not reflected in a batch response.
		{`[{"jsonrpc": "2.0", "method": "note"}, {"jsonrpc": "2.0", "id": 8, "method": "X"}]`,
			`[{"jsonrpc":"2.0","id":8,"result":"OK"}]`},

		// Invalid structure for a version is reported, with and without ID.
		{`{"jsonrpc": false}`,
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"invalid version key"}}`},
		{`{"jsonrpc": false, "id": 747}`,
			`{"jsonrpc":"2.0","id":747,"error":{"code":-32700,"message":"invalid version key"}}`},

		// Invalid structure for a method name is reported, with and without ID.
		{`{"jsonrpc":"2.0", "method": [false]}`,
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"invalid method name"}}`},
		{`{"jsonrpc":"2.0", "method": [false], "id": 252}`,
			`{"jsonrpc":"2.0","id":252,"error":{"code":-32700,"message":"invalid method name"}}`},

		// A broken batch request should report a single top-level error.
		{`[{"jsonrpc":"2.0", "method":"A", "id": 1}, {"jsonrpc":"2.0"]`, // N.B. syntax error
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"invalid request value"}}`},

		// A broken single request should report a top-level error.
		{`{"bogus"][++`,
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"invalid request value"}}`},

		// Various invalid ID checks.
		{`{"jsonrpc":"2.0", "id":[], "method":"X"}`, invalidIDMessage},    // invalid ID: array
		{`{"jsonrpc":"2.0", "id":["q"], "method":"X"}`, invalidIDMessage}, // "
		{`{"jsonrpc":"2.0", "id":{}, "method":"X"}`, invalidIDMessage},    // invalid ID: object
		{`{"jsonrpc":"2.0", "id":true, "method":"X"}`, invalidIDMessage},  // invalid ID: Boolean
		{`{"jsonrpc":"2.0", "id":false, "method":"X"}`, invalidIDMessage}, // "
	}
	for _, test := range tests {
		if err := cli.Send([]byte(test.input)); err != nil {
			t.Fatalf("Send %#q failed: %v", test.input, err)
		}
		raw, err := cli.Recv()
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		if got := string(raw); got != test.want {
			t.Errorf("Simulated call %#q: got %#q, want %#q", test.input, got, test.want)
		}
	}
}

// Verify that server-side push notifications work.
func TestServer_Notify(t *testing.T) {
	defer leaktest.Check(t)()

	// Set up a server and client with server-side notification support.  Here
	// we're just capturing the name of the notification method, as a sign we
	// got the right thing.
	var notes []string
	loc := server.NewLocal(handler.Map{
		"NoteMe": handler.New(func(ctx context.Context) (bool, error) {
			// When this method is called, it posts a notification back to the
			// client before returning.
			if err := jrpc2.ServerFromContext(ctx).Notify(ctx, "method", nil); err != nil {
				t.Errorf("Push Notify unexpectedly failed: %v", err)
				return false, err
			}
			return true, nil
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{
			AllowPush: true,
		},
		Client: &jrpc2.ClientOptions{
			OnNotify: func(req *jrpc2.Request) {
				notes = append(notes, req.Method())
				t.Logf("OnNotify handler saw method %q", req.Method())
			},
		},
	})
	s, c := loc.Server, loc.Client
	ctx := context.Background()

	// Post an explicit notification.
	if err := s.Notify(ctx, "explicit", nil); err != nil {
		t.Errorf("Notify explicit: unexpected error: %v", err)
	}

	// Call the method that posts a notification.
	if _, err := c.Call(ctx, "NoteMe", nil); err != nil {
		t.Errorf("Call NoteMe: unexpected error: %v", err)
	}

	// Shut everything down to be sure the callbacks have settled.
	// Sort the results since the order of arrival may vary.
	loc.Close()
	sort.Strings(notes)

	want := []string{"explicit", "method"}
	if diff := cmp.Diff(want, notes); diff != "" {
		t.Errorf("Server notifications: (-want, +got)\n%s", diff)
	}
}

// Verify that server-side callbacks can time out.
func TestServer_callbackTimeout(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context) error {
			tctx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
			defer cancel()
			rsp, err := jrpc2.ServerFromContext(ctx).Callback(tctx, "hey", nil)
			if err == context.DeadlineExceeded {
				t.Logf("Callback correctly failed: %v", err)
				return nil
			} else if err != nil {
				return fmt.Errorf("unexpected error: %v", err)
			}
			return fmt.Errorf("got rsp=%+v, want error", rsp)
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{AllowPush: true},

		// N.B. Client does not have a callback handler, so calls will be ignored
		// and no response will be generated.
	})
	defer loc.Close()
	ctx := context.Background()

	if _, err := loc.Client.Call(ctx, "Test", nil); err != nil {
		t.Errorf("Call failed: %v", err)
	}
}

// Verify that server-side callbacks work.
func TestServer_Callback(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{
		"CallMeMaybe": handler.New(func(ctx context.Context) error {
			if _, err := jrpc2.ServerFromContext(ctx).Callback(ctx, "succeed", nil); err != nil {
				t.Errorf("Callback failed: %v", err)
			}

			if rsp, err := jrpc2.ServerFromContext(ctx).Callback(ctx, "fail", nil); err == nil {
				t.Errorf("Callback did not fail: got %v, want error", rsp)
			}
			return nil
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{AllowPush: true},
		Client: &jrpc2.ClientOptions{
			OnCallback: func(ctx context.Context, req *jrpc2.Request) (interface{}, error) {
				t.Logf("OnCallback invoked for method %q", req.Method())
				switch req.Method() {
				case "succeed":
					return true, nil
				case "fail":
					return false, errors.New("here is your requested error")
				}
				panic("broken test: you should not see this")
			},
		},
	})
	defer loc.Close()
	ctx := context.Background()

	// Call the method that posts a callback.
	if _, err := loc.Client.Call(ctx, "CallMeMaybe", nil); err != nil {
		t.Fatalf("Call CallMeMaybe: unexpected error: %v", err)
	}

	// Post an explicit callback.
	if _, err := loc.Server.Callback(ctx, "succeed", nil); err != nil {
		t.Errorf("Callback explicit: unexpected error: %v", err)
	}
}

// Verify that a server push after the client closes does not trigger a panic.
func TestServer_pushAfterClose(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(make(handler.Map), &server.LocalOptions{
		Server: &jrpc2.ServerOptions{AllowPush: true},
	})
	loc.Client.Close()
	ctx := context.Background()
	if err := loc.Server.Notify(ctx, "whatever", nil); err != jrpc2.ErrConnClosed {
		t.Errorf("Notify(whatever): got %v, want %v", err, jrpc2.ErrConnClosed)
	}
	if rsp, err := loc.Server.Callback(ctx, "whatever", nil); err != jrpc2.ErrConnClosed {
		t.Errorf("Callback(whatever): got %v, %v; want %v", rsp, err, jrpc2.ErrConnClosed)
	}
}

// Verify that an OnCancel hook is called when expected.
func TestClient_onCancelHook(t *testing.T) {
	defer leaktest.Check(t)()

	hooked := make(chan struct{}) // closed when hook notification is finished

	loc := server.NewLocal(handler.Map{
		// Block until explicitly cancelled or a long timeout expires.
		"Stall": handler.New(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				t.Logf("Method unblocked; returning err=%v", ctx.Err())
				return ctx.Err()
			case <-time.After(10 * time.Second): // shouldn't happen
				t.Error("Timeout waiting for server cancellation")
			}
			return nil
		}),

		// Cancel the specified request (notification only).
		"computerSaysNo": handler.New(func(ctx context.Context, ids []string) error {
			defer close(hooked)
			if req := jrpc2.InboundRequest(ctx); !req.IsNotification() {
				return jrpc2.Errorf(code.MethodNotFound, "no such method %q", req.Method())
			}
			srv := jrpc2.ServerFromContext(ctx)
			for _, id := range ids {
				srv.CancelRequest(id)
				t.Logf("In cancellation handler, cancelled request id=%v", id)
			}
			return nil
		}),
	}, &server.LocalOptions{
		Client: &jrpc2.ClientOptions{
			OnCancel: func(cli *jrpc2.Client, rsp *jrpc2.Response) {
				t.Logf("OnCancel hook called with id=%q, err=%v", rsp.ID(), rsp.Error())
				cli.Notify(context.Background(), "computerSaysNo", []string{rsp.ID()})
			},
		},
	})

	// Call a method on the server that will stall until its context terminates.
	// On the client side, set a deadline to expire the caller's context.
	// The cancellation hook will notify the server to unblock the method.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	got, err := loc.Client.Call(ctx, "Stall", nil)
	if err == nil {
		t.Errorf("Stall: got %+v, wanted error", got)
	} else if err != context.DeadlineExceeded {
		t.Errorf("Stall: got error %v, want %v", err, context.Canceled)
	}
	<-hooked
	loc.Client.Close()
	if err := loc.Server.Wait(); err != nil {
		t.Errorf("Server exit status: %v", err)
	}
}

// Verify that client callback handlers are cancelled when the client stops.
func TestClient_closeEndsCallbacks(t *testing.T) {
	defer leaktest.Check(t)()

	ready := make(chan struct{})
	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context) error {
			// Call back to the client and block indefinitely until it returns.
			srv := jrpc2.ServerFromContext(ctx)
			_, err := srv.Callback(ctx, "whatever", nil)
			return err
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{AllowPush: true},
		Client: &jrpc2.ClientOptions{
			OnCallback: handler.New(func(ctx context.Context) error {
				// Signal the test that the callback handler is running.  When the
				// client is closed, it should terminate ctx and allow this to
				// return. If that doesn't happen, time out and fail.
				close(ready)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(10 * time.Second):
					return errors.New("context not cancelled before timeout")
				}
			}),
		},
	})
	go func() {
		rsp, err := loc.Client.Call(context.Background(), "Test", nil)
		if err == nil {
			t.Errorf("Client call: got %+v, wanted error", rsp)
		}
	}()
	<-ready
	loc.Client.Close()
	loc.Server.Wait()
}

// Verify that it is possible for multiple callback handlers to execute
// concurrently.
func TestClient_concurrentCallbacks(t *testing.T) {
	defer leaktest.Check(t)()

	ready1 := make(chan struct{})
	ready2 := make(chan struct{})
	release := make(chan struct{})

	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context) []string {
			srv := jrpc2.ServerFromContext(ctx)

			// Call two callbacks concurrently, wait until they are both running,
			// then ungate them and wait for them both to reply. Return their
			// responses back to the test for validation.
			ss := make([]string, 2)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				rsp, err := srv.Callback(ctx, "C1", nil)
				if err != nil {
					t.Errorf("Callback C1 failed: %v", err)
				} else {
					rsp.UnmarshalResult(&ss[0])
				}
			}()
			go func() {
				defer wg.Done()
				rsp, err := srv.Callback(ctx, "C2", nil)
				if err != nil {
					t.Errorf("Callback C2 failed: %v", err)
				} else {
					rsp.UnmarshalResult(&ss[1])
				}
			}()
			<-ready1       // C1 is ready
			<-ready2       // C2 is ready
			close(release) // allow all callbacks to proceed
			wg.Wait()      // wait for all callbacks to be done
			return ss
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{AllowPush: true},
		Client: &jrpc2.ClientOptions{
			OnCallback: handler.Func(func(ctx context.Context, req *jrpc2.Request) (interface{}, error) {
				// A trivial callback that reports its method name.
				// The name is used to select which invocation we are serving.
				switch req.Method() {
				case "C1":
					close(ready1)
				case "C2":
					close(ready2)
				default:
					return nil, fmt.Errorf("unexpected method %q", req.Method())
				}
				<-release
				return req.Method(), nil
			}),
		},
	})
	defer loc.Close()

	var got []string
	if err := loc.Client.CallResult(context.Background(), "Test", nil, &got); err != nil {
		t.Errorf("Call Test failed: %v", err)
	}
	want := []string{"C1", "C2"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Wrong callback results: (-want, +got)\n%s", diff)
	}
}

// Verify that a callback can successfully call "up" into the server.
func TestClient_callbackUpCall(t *testing.T) {
	defer leaktest.Check(t)()

	const pingMessage = "kittens!"

	var probe string
	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context) error {
			// Call back to the client, and propagate its response.
			srv := jrpc2.ServerFromContext(ctx)
			_, err := srv.Callback(ctx, "whatever", nil)
			return err
		}),
		"Ping": handler.New(func(context.Context) string {
			// This method is called by the client-side callback.
			return pingMessage
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{AllowPush: true},
		Client: &jrpc2.ClientOptions{
			OnCallback: handler.New(func(ctx context.Context) error {
				// Call back up into the server.
				cli := jrpc2.ClientFromContext(ctx)
				return cli.CallResult(ctx, "Ping", nil, &probe)
			}),
		},
	})

	if _, err := loc.Client.Call(context.Background(), "Test", nil); err != nil {
		t.Errorf("Call Test failed: %v", err)
	}
	loc.Close()
	if probe != pingMessage {
		t.Errorf("Probe response: got %q, want %q", probe, pingMessage)
	}
}

// Verify that the context encoding/decoding hooks work.
func TestContextPlumbing(t *testing.T) {
	defer leaktest.Check(t)()

	want := time.Now().Add(10 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), want)
	defer cancel()

	loc := server.NewLocal(handler.Map{
		"X": handler.New(func(ctx context.Context) (bool, error) {
			got, ok := ctx.Deadline()
			if !ok {
				return false, errors.New("no deadline was set")
			} else if !got.Equal(want) {
				return false, fmt.Errorf("deadline: got %v, want %v", got, want)
			}
			t.Logf("Got expected deadline: %v", got)
			return true, nil
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{DecodeContext: jctx.Decode},
		Client: &jrpc2.ClientOptions{EncodeContext: jctx.Encode},
	})
	defer loc.Close()

	if _, err := loc.Client.Call(ctx, "X", nil); err != nil {
		t.Errorf("Call X failed: %v", err)
	}
}

// Verify that calling a wrapped method which takes no parameters, but in which
// the caller provided parameters, will correctly report an error.
func TestHandler_noParams(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{"Test": testOK}, nil)
	defer loc.Close()

	var rsp string
	if err := loc.Client.CallResult(context.Background(), "Test", []int{1, 2, 3}, &rsp); err == nil {
		t.Errorf("Call(Test): got %q, wanted error", rsp)
	} else if ec := code.FromError(err); ec != code.InvalidParams {
		t.Errorf("Call(Test): got code %v, wanted %v", ec, code.InvalidParams)
	}
}

// Verify that the rpc.serverInfo handler and client wrapper work together.
func TestRPCServerInfo(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(handler.Map{"Test": testOK}, nil)
	defer loc.Close()

	si, err := jrpc2.RPCServerInfo(context.Background(), loc.Client)
	if err != nil {
		t.Errorf("RPCServerInfo failed: %v", err)
	}
	{
		got, want := si.Methods, []string{"Test"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Wrong method names: (-want, +got)\n%s", diff)
		}
	}
}

func TestNetwork(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"", "unix"},
		{":", "unix"},

		{"nothing", "unix"},        // no colon
		{"like/a/file", "unix"},    // no colon
		{"no-port:", "unix"},       // empty port
		{"file/with:port", "unix"}, // slashes in host
		{"path/with:404", "unix"},  // slashes in host
		{"mangled:@3", "unix"},     // non-alphanumerics in port

		{":80", "tcp"},            // numeric port
		{":dumb-crud", "tcp"},     // service name
		{"localhost:80", "tcp"},   // host and numeric port
		{"localhost:http", "tcp"}, // host and service name
	}
	for _, test := range tests {
		got, addr := jrpc2.Network(test.input)
		if got != test.want {
			t.Errorf("Network(%q) type: got %q, want %q", test.input, got, test.want)
		}
		if addr != test.input {
			t.Errorf("Network(%q) addr: got %q, want %q", test.input, addr, test.input)
		}
	}
}

// Verify that the context passed to an assigner has the correct structure.
func TestHandler_assignContext(t *testing.T) {
	defer leaktest.Check(t)()

	loc := server.NewLocal(assignFunc(func(ctx context.Context, method string) jrpc2.Handler {
		req := jrpc2.InboundRequest(ctx)
		if req == nil {
			t.Errorf("No inbound request for assignment of %q", method)
		} else if req.Method() != method {
			t.Errorf("Assign inbound: got %q, want %q", req.Method(), method)
		} else {
			t.Logf("Inbound request id=%v method=%q OK", req.ID(), req.Method())
		}
		return testOK
	}), nil)
	defer loc.Close()

	ctx := context.Background()
	var got string
	if err := loc.Client.CallResult(ctx, "NerbleFleeger", nil, &got); err != nil {
		t.Errorf("CallResult unexpectedly failed: %v", err)
	} else if got != "OK" {
		t.Errorf("CallResult: got %q, want %q", got, "OK")
	}
}

type assignFunc func(context.Context, string) jrpc2.Handler

func (a assignFunc) Assign(ctx context.Context, m string) jrpc2.Handler { return a(ctx, m) }

func TestServer_WaitStatus(t *testing.T) {
	defer leaktest.Check(t)()

	check := func(t *testing.T, stat jrpc2.ServerStatus, closed, stopped bool, wantErr error) {
		t.Helper()
		if got, want := stat.Success(), wantErr == nil; got != want {
			t.Errorf("Status success: got %v, want %v", got, want)
		}
		if got := stat.Closed; got != closed {
			t.Errorf("Status closed: got %v, want %v", got, closed)
		}
		if got := stat.Stopped; got != stopped {
			t.Errorf("Status stopped: got %v, want %v", got, stopped)
		}
		if stat.Err != wantErr {
			t.Errorf("Status error: got %v, want %v", stat.Err, wantErr)
		}
	}
	t.Run("ChannelClosed", func(t *testing.T) {
		loc := server.NewLocal(handler.Map{"OK": testOK}, nil)
		loc.Client.Close()
		check(t, loc.Server.WaitStatus(), true, false, nil)
	})

	t.Run("ServerStopped", func(t *testing.T) {
		loc := server.NewLocal(handler.Map{"OK": testOK}, nil)
		loc.Server.Stop()
		check(t, loc.Server.WaitStatus(), false, true, nil)
	})

	t.Run("ChannelFailed", func(t *testing.T) {
		wantErr := errors.New("failed")
		ch := buggyChannel{data: "bogus", err: wantErr}
		srv := jrpc2.NewServer(handler.Map{"OK": testOK}, nil).Start(ch)
		check(t, srv.WaitStatus(), false, false, wantErr)
	})
}

type buggyChannel struct {
	data string
	err  error
}

func (buggyChannel) Send([]byte) error       { panic("should not be called") }
func (b buggyChannel) Recv() ([]byte, error) { return []byte(b.data), b.err }
func (buggyChannel) Close() error            { return nil }

func TestRequest_strictFields(t *testing.T) {
	defer leaktest.Check(t)()

	type other struct {
		C bool `json:"charlie"`
	}
	type params struct {
		A string `json:"alpha"`
		B int    `json:"bravo"`
		other
	}
	loc := server.NewLocal(handler.Map{
		"Strict": handler.New(func(ctx context.Context, req *jrpc2.Request) (string, error) {
			var ps params
			if err := req.UnmarshalParams(jrpc2.StrictFields(&ps)); err != nil {
				return "", err
			}
			return ps.A, nil
		}),
		"Normal": handler.New(func(ctx context.Context, req *jrpc2.Request) (string, error) {
			var ps params
			if err := req.UnmarshalParams(&ps); err != nil {
				return "", err
			}
			return ps.A, nil
		}),
	}, nil)
	defer loc.Close()
	ctx := context.Background()

	tests := []struct {
		method string
		params interface{}
		code   code.Code
		want   string
	}{
		{"Strict", handler.Obj{"alpha": "aiuto"}, code.NoError, "aiuto"},
		{"Strict", handler.Obj{"alpha": "selva me", "charlie": true}, code.NoError, "selva me"},
		{"Strict", handler.Obj{"alpha": "OK", "nonesuch": true}, code.InvalidParams, ""},
		{"Normal", handler.Obj{"alpha": "OK", "nonesuch": true}, code.NoError, "OK"},
	}
	for _, test := range tests {
		name := test.method + "/"
		if test.code == code.NoError {
			name += "OK"
		} else {
			name += test.code.String()
		}
		t.Run(name, func(t *testing.T) {
			var res string
			err := loc.Client.CallResult(ctx, test.method, test.params, &res)
			if err == nil && test.code != code.NoError {
				t.Errorf("CallResult: got %+v, want error code %v", res, test.code)
			} else if err != nil {
				if c := code.FromError(err); c != test.code {
					t.Errorf("CallResult: got error %v, wanted code %v", err, test.code)
				}
			} else if res != test.want {
				t.Errorf("CallResult: got %#q, want %#q", res, test.want)
			}
		})
	}
}

func TestResponse_strictFields(t *testing.T) {
	defer leaktest.Check(t)()

	type result struct {
		A string `json:"alpha"`
	}
	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context, req *jrpc2.Request) handler.Obj {
			return handler.Obj{"alpha": "OK", "bravo": "not OK"}
		}),
	}, nil)
	defer loc.Close()
	ctx := context.Background()

	res, err := loc.Client.Call(ctx, "Test", nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	t.Run("Normal", func(t *testing.T) {
		var got result
		if err := res.UnmarshalResult(&got); err != nil {
			t.Errorf("UnmarshalResult failed: %v", err)
		} else if got.A != "OK" {
			t.Errorf("Result: got %#q, want OK", got.A)
		}
	})
	t.Run("Strict", func(t *testing.T) {
		var got result
		if err := res.UnmarshalResult(jrpc2.StrictFields(&got)); err == nil {
			t.Errorf("UnmarshalResult: got %#v, wanted error", got)
		}
	})
}

func TestServerFromContext(t *testing.T) {
	defer leaktest.Check(t)()

	var got *jrpc2.Server
	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context) error {
			got = jrpc2.ServerFromContext(ctx)
			return nil
		}),
	}, nil)
	if _, err := loc.Client.Call(context.Background(), "Test", nil); err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if err := loc.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if got != loc.Server {
		t.Errorf("ServerFromContext: got %p, want %p", got, loc.Server)
	}
}

func TestServer_newContext(t *testing.T) {
	defer leaktest.Check(t)()

	// Prepare a context with a test value attached to it, that the handler can
	// extract to verify that the base context was plumbed in correctly.
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("test"), 42)

	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context) error {
			val := ctx.Value(ctxKey("test"))
			if val == nil {
				t.Error("Test value is not present in context")
			} else if v, ok := val.(int); !ok || v != 42 {
				t.Errorf("Wrong test value: got %+v, want %v", val, 42)
			}
			return nil
		}),
	}, &server.LocalOptions{
		Server: &jrpc2.ServerOptions{
			// Use the test context constructed above as the base request context.
			NewContext: func() context.Context { return ctx },
		},
	})
	defer loc.Close()
	if _, err := loc.Client.Call(context.Background(), "Test", nil); err != nil {
		t.Errorf("Call failed: %v", err)
	}
}
