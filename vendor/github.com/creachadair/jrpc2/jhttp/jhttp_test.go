package jhttp

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
)

func TestBridge(t *testing.T) {
	// Set up a JSON-RPC server to answer requests bridged from HTTP.
	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context, ss ...string) (string, error) {
			return strings.Join(ss, " "), nil
		}),
	}, nil)
	defer loc.Close()

	// Bridge HTTP to the JSON-RPC server.
	b := NewBridge(loc.Client)
	defer b.Close()

	// Create an HTTP test server to call into the bridge.
	hsrv := httptest.NewServer(b)
	defer hsrv.Close()

	// Verify that a valid POST request succeeds.
	t.Run("PostOK", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "application/json", strings.NewReader(`{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "Test",
  "params": ["a", "foolish", "consistency", "is", "the", "hobgoblin"]
}
`))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST response code: got %v, want %v", got, want)
		}
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			t.Errorf("Reading POST body: %v", err)
		}

		const want = `{"jsonrpc":"2.0","id":1,"result":"a foolish consistency is the hobgoblin"}`
		if got := string(body); got != want {
			t.Errorf("POST body: got %#q, want %#q", got, want)
		}
	})

	// Verify that the bridge will accept a batch.
	t.Run("PostBatchOK", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "application/json", strings.NewReader(`[
  {"jsonrpc":"2.0", "id": 3, "method": "Test", "params": ["first"]},
  {"jsonrpc":"2.0", "id": 7, "method": "Test", "params": ["among", "equals"]}
]
`))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST response code: got %v, want %v", got, want)
		}
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			t.Errorf("Reading POST body: %v", err)
		}

		const want = `[{"jsonrpc":"2.0","id":3,"result":"first"},` +
			`{"jsonrpc":"2.0","id":7,"result":"among equals"}]`
		if got := string(body); got != want {
			t.Errorf("POST body: got %#q, want %#q", got, want)
		}
	})

	// Verify that a GET request reports an error.
	t.Run("GetFail", func(t *testing.T) {
		rsp, err := http.Get(hsrv.URL)
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		if got, want := rsp.StatusCode, http.StatusMethodNotAllowed; got != want {
			t.Errorf("GET status: got %v, want %v", got, want)
		}
	})

	// Verify that a POST with the wrong content type fails.
	t.Run("PostInvalidType", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "text/plain", strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		}
		if got, want := rsp.StatusCode, http.StatusUnsupportedMediaType; got != want {
			t.Errorf("POST status: got %v, want %v", got, want)
		}
	})

	// Verify that a POST that generates a JSON-RPC error succeeds.
	t.Run("PostErrorReply", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "application/json", strings.NewReader(`{
  "id": 1,
  "jsonrpc": "2.0"
}
`))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST status: got %v, want %v", got, want)
		}
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			t.Errorf("Reading POST body: %v", err)
		}

		const exp = `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"empty method name"}}`
		if got := string(body); got != exp {
			t.Errorf("POST body: got %#q, want %#q", got, exp)
		}
	})

	// Verify that a notification returns an empty success.
	t.Run("PostNotification", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "application/json", strings.NewReader(`{
  "jsonrpc": "2.0",
  "method": "TakeNotice",
  "params": []
}`))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusNoContent; got != want {
			t.Errorf("POST status: got %v, want %v", got, want)
		}
		body, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			t.Errorf("Reading POST body: %v", err)
		}
		if got := string(body); got != "" {
			t.Errorf("POST body: got %q, want empty", got)
		}
	})
}

func TestChannel(t *testing.T) {
	loc := server.NewLocal(handler.Map{
		"Test": handler.New(func(ctx context.Context, arg json.RawMessage) (int, error) {
			return len(arg), nil
		}),
	}, nil)
	defer loc.Close()

	b := NewBridge(loc.Client)
	defer b.Close()
	hsrv := httptest.NewServer(b)
	defer hsrv.Close()

	ctx := context.Background()
	ch := NewChannel(hsrv.URL)
	cli := jrpc2.NewClient(ch, nil)

	tests := []struct {
		params interface{}
		want   int
	}{
		{nil, 0},
		{[]string{"foo"}, 7},
		{map[string]int{"hi": 3}, 8},
	}
	for _, test := range tests {
		var got int
		if err := cli.CallResult(ctx, "Test", test.params, &got); err != nil {
			t.Errorf("Call Test(%v): unexpected error: %v", test.params, err)
		} else if got != test.want {
			t.Errorf("Call Test(%v): got %d, want %d", test.params, got, test.want)
		}
	}

	cli.Close() // also closes the channel

	// Verify that a closed channel reports errors for Send and Recv.
	if err := ch.Send([]byte("whatever")); err == nil {
		t.Error("Send on a closed channel unexpectedly worked")
	}
	if got, err := ch.Recv(); err != io.EOF {
		t.Errorf("Recv = (%#q, %v), want (nil, %v", string(got), err, io.EOF)
	}
}
