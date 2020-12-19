package jrpc2

// This file contains tests that need to inspect the internal details of the
// implementation to verify that the results are correct.

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/code"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseRequests(t *testing.T) {
	tests := []struct {
		input string
		want  []*Request
		err   error
	}{
		// An empty batch is valid and produces no results.
		{`[]`, nil, nil},

		// An empty single request is invalid but returned anyway.
		{`{}`, []*Request{{}}, ErrInvalidVersion},

		// A valid notification.
		{`{"jsonrpc":"2.0", "method": "foo", "params":[1, 2, 3]}`, []*Request{{
			method: "foo",
			params: json.RawMessage(`[1, 2, 3]`),
		}}, nil},

		// A valid request, with nil parameters.
		{`{"jsonrpc":"2.0", "method": "foo", "id":10332, "params":null}`, []*Request{{
			id: json.RawMessage("10332"), method: "foo",
		}}, nil},

		// A valid mixed batch.
		{`[ {"jsonrpc": "2.0", "id": 1, "method": "A", "params": {}},
          {"jsonrpc": "2.0", "params": [5], "method": "B"} ]`, []*Request{
			{method: "A", id: json.RawMessage(`1`), params: json.RawMessage(`{}`)},
			{method: "B", params: json.RawMessage(`[5]`)},
		}, nil},

		// An invalid batch.
		{`[{"id": 37, "method": "complain", "params":[]}]`, []*Request{
			{method: "complain", id: json.RawMessage(`37`), params: json.RawMessage(`[]`)},
		}, ErrInvalidVersion},

		// A broken request.
		{`{`, nil, Errorf(code.ParseError, "invalid request message")},

		// A broken batch.
		{`["bad"{]`, nil, Errorf(code.ParseError, "invalid request batch")},
	}
	for _, test := range tests {
		got, err := ParseRequests([]byte(test.input))
		if !errEQ(err, test.err) {
			t.Errorf("ParseRequests(%#q): got error %v, want %v", test.input, err, test.err)
			continue
		}

		diff := cmp.Diff(test.want, got, cmp.AllowUnexported(Request{}), cmpopts.EquateEmpty())
		if diff != "" {
			t.Errorf("ParseRequests(%#q): wrong result (-want, +got):\n%s", test.input, diff)
		}
	}
}

func errEQ(x, y error) bool {
	if x == nil {
		return y == nil
	} else if y == nil {
		return false
	}
	return code.FromError(x) == code.FromError(y) && x.Error() == y.Error()
}

func TestUnmarshalParams(t *testing.T) {
	type xy struct {
		X int  `json:"x"`
		Y bool `json:"y"`
	}

	tests := []struct {
		input   string
		want    interface{}
		pstring string
		code    code.Code
	}{
		// If parameters are set, the target should be updated.
		{`{"jsonrpc":"2.0", "id":1, "method":"X", "params":[1,2]}`, []int{1, 2}, "[1,2]", code.NoError},

		// If parameters are null, the target should not be modified.
		{`{"jsonrpc":"2.0", "id":2, "method":"Y", "params":null}`, "", "", code.NoError},

		// If parameters are not set, the target should not be modified.
		{`{"jsonrpc":"2.0", "id":2, "method":"Y"}`, 0, "", code.NoError},

		// Unmarshaling should work into a struct as long as the fields match.
		{`{"jsonrpc":"2.0", "id":3, "method":"Z", "params":{}}`, xy{}, "{}", code.NoError},
		{`{"jsonrpc":"2.0", "id":4, "method":"Z", "params":{"x":17}}`, xy{X: 17}, `{"x":17}`, code.NoError},
		{`{"jsonrpc":"2.0", "id":5, "method":"Z", "params":{"x":23, "y":true}}`,
			xy{X: 23, Y: true}, `{"x":23, "y":true}`, code.NoError},
		{`{"jsonrpc":"2.0", "id":6, "method":"Z", "params":{"x":23, "z":"wat"}}`,
			xy{X: 23}, `{"x":23, "z":"wat"}`, code.NoError},
	}
	for _, test := range tests {
		req, err := ParseRequests([]byte(test.input))
		if err != nil {
			t.Errorf("Parsing request %#q failed: %v", test.input, err)
		} else if len(req) != 1 {
			t.Fatalf("Wrong number of requests: got %d, want 1", len(req))
		}

		// Allocate a zero of the expected type to unmarshal into.
		target := reflect.New(reflect.TypeOf(test.want)).Interface()
		{
			err := req[0].UnmarshalParams(target)
			if got := code.FromError(err); got != test.code {
				t.Errorf("UnmarshalParams error: got code %d, want %d [%v]", got, test.code, err)
			}
			if err != nil {
				continue
			}
		}

		// Dereference the target to get the value to compare.
		got := reflect.ValueOf(target).Elem().Interface()
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("Parameters(%#q): wrong result (-want, +got):\n%s", test.input, diff)
		}

		// Check that the parameter string matches.
		if got := req[0].ParamString(); got != test.pstring {
			t.Errorf("ParamString(%#q): got %q, want %q", test.input, got, test.pstring)
		}
	}
}

type hmap map[string]Handler

func (h hmap) Assign(_ context.Context, method string) Handler { return h[method] }
func (h hmap) Names() []string                                 { return nil }

// Verify that if the client context terminates during a request, the client
// will terminate and report failure.
func TestClientCancellation(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan bool, 1)
	cpipe, spipe := channel.Direct()
	srv := NewServer(hmap{
		"Hang": methodFunc(func(ctx context.Context, _ *Request) (interface{}, error) {
			close(started) // signal that the method handler is running
			defer close(stopped)

			t.Log("Waiting for context completion...")
			select {
			case <-ctx.Done():
				t.Logf("Server context cancelled: err=%v", ctx.Err())
				stopped <- true
				return true, ctx.Err()
			case <-time.After(10 * time.Second):
				return false, nil
			}
		}),
	}, nil).Start(spipe)
	c := NewClient(cpipe, nil)
	defer func() {
		c.Close()
		srv.Wait()
	}()

	// Start a call that will hang around until a timer expires or an explicit
	// cancellation is received.
	ctx, cancel := context.WithCancel(context.Background())
	req, err := c.req(ctx, "Hang", nil)
	if err != nil {
		t.Fatalf("c.req(Hang) failed: %v", err)
	}
	rsps, err := c.send(ctx, jmessages{req})
	if err != nil {
		t.Fatalf("c.send(Hang) failed: %v", err)
	}

	// Wait for the handler to start so that we don't race with calling the
	// handler on the server side, then cancel the context client-side.
	<-started
	cancel()

	// The call should fail client side, in the usual way for a cancellation.
	rsp := rsps[0]
	rsp.wait()
	if err := rsp.Error(); err != nil {
		if err.code != code.Cancelled {
			t.Errorf("Response error for %q: got %v, want %v", rsp.ID(), err, code.Cancelled)
		}
	} else {
		t.Errorf("Response for %q: unexpectedly succeeded", rsp.ID())
	}

	// The server handler should have reported a cancellation.
	if ok := <-stopped; !ok {
		t.Error("Server context was not cancelled")
	}
}

func TestSpecialMethods(t *testing.T) {
	s := NewServer(hmap{
		"rpc.nonesuch": methodFunc(func(context.Context, *Request) (interface{}, error) {
			return "OK", nil
		}),
		"donkeybait": methodFunc(func(context.Context, *Request) (interface{}, error) {
			return true, nil
		}),
	}, nil)
	ctx := context.Background()
	for _, name := range []string{rpcServerInfo, rpcCancel, "donkeybait"} {
		if got := s.assign(ctx, name); got == nil {
			t.Errorf("s.assign(%s): no method assigned", name)
		}
	}
	if got := s.assign(ctx, "rpc.nonesuch"); got != nil {
		t.Errorf("s.assign(rpc.nonesuch): got %v, want nil", got)
	}
}

// Verify that the option to remove the special behaviour of rpc.* methods can
// be correctly disabled by the server options.
func TestDisableBuiltin(t *testing.T) {
	s := NewServer(hmap{
		"rpc.nonesuch": methodFunc(func(context.Context, *Request) (interface{}, error) {
			return "OK", nil
		}),
	}, &ServerOptions{DisableBuiltin: true})
	ctx := context.Background()

	// With builtins disabled, the default rpc.* methods should not get assigned.
	for _, name := range []string{rpcServerInfo, rpcCancel} {
		if got := s.assign(ctx, name); got != nil {
			t.Errorf("s.assign(%s): got %+v, wanted nil", name, got)
		}
	}

	// However, user-assigned methods with this prefix should now work.
	if got := s.assign(ctx, "rpc.nonesuch"); got == nil {
		t.Error("s.assign(rpc.nonesuch): missing assignment")
	}
}

// Verify that a batch request gets a batch reply, even if it is only a single
// request. The Client never sends requests like that, but the server needs to
// cope with it correctly.
func TestBatchReply(t *testing.T) {
	cpipe, spipe := channel.Direct()
	srv := NewServer(hmap{
		"test": methodFunc(func(_ context.Context, req *Request) (interface{}, error) {
			t.Logf("Called %q", req.Method())
			return req.Method() + " OK", nil
		}),
	}, nil).Start(spipe)
	defer func() { cpipe.Close(); srv.Wait() }()

	tests := []struct {
		input, want string
	}{
		// A single-element batch gets returned as a batch.
		{`[{"jsonrpc":"2.0", "id":1, "method":"test"}]`,
			`[{"jsonrpc":"2.0","id":1,"result":"test OK"}]`},

		// A single-element non-batch gets returned as a single reply.
		{`{"jsonrpc":"2.0", "id":2, "method":"test"}`,
			`{"jsonrpc":"2.0","id":2,"result":"test OK"}`},
	}
	for _, test := range tests {
		if err := cpipe.Send([]byte(test.input)); err != nil {
			t.Errorf("Send failed: %v", err)
		}
		rsp, err := cpipe.Recv()
		if err != nil {
			t.Errorf("Recv failed: %v", err)
		}
		if got := string(rsp); got != test.want {
			t.Errorf("Batch reply:\n got %#q\nwant %#q", got, test.want)
		}
	}
}

func TestMarshalResponse(t *testing.T) {
	tests := []struct {
		id     string
		err    *Error
		result string
		want   string
	}{
		{"", nil, "", `{"jsonrpc":"2.0"}`},
		{"null", nil, "", `{"jsonrpc":"2.0","id":null}`},
		{"123", Errorf(code.ParseError, "failed").(*Error), "",
			`{"jsonrpc":"2.0","id":123,"error":{"code":-32700,"message":"failed"}}`},
		{"456", nil, `{"ok":true,"values":[4,5,6]}`,
			`{"jsonrpc":"2.0","id":456,"result":{"ok":true,"values":[4,5,6]}}`},
	}
	for _, test := range tests {
		rsp := &Response{id: test.id, err: test.err}
		if test.err == nil {
			rsp.result = json.RawMessage(test.result)
		}

		got, err := json.Marshal(rsp)
		if err != nil {
			t.Errorf("Marshaling %+v: unexpected error: %v", rsp, err)
		} else if s := string(got); s != test.want {
			t.Errorf("Marshaling %+v: got %#q, want %#q", rsp, s, test.want)
		}
	}
}
