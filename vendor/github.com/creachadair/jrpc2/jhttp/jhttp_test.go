package jhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jhttp"
)

var testService = handler.Map{
	"Test1": handler.New(func(ctx context.Context, ss []string) int {
		return len(ss)
	}),
	"Test2": handler.New(func(ctx context.Context, req json.RawMessage) int {
		return len(req)
	}),
}

func checkContext(ctx context.Context, _ string, p json.RawMessage) (json.RawMessage, error) {
	if jhttp.HTTPRequest(ctx) == nil {
		return nil, errors.New("no HTTP request in context")
	}
	return p, nil
}

func TestBridge(t *testing.T) {
	// Set up a bridge with the test configuration.
	b := jhttp.NewBridge(testService, &jhttp.BridgeOptions{
		Client: &jrpc2.ClientOptions{EncodeContext: checkContext},
	})
	defer checkClose(t, b)

	// Create an HTTP test server to call into the bridge.
	hsrv := httptest.NewServer(b)
	defer hsrv.Close()

	// Verify that a valid POST request succeeds.
	t.Run("PostOK", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "application/json", strings.NewReader(`{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "Test1",
  "params": ["a", "foolish", "consistency", "is", "the", "hobgoblin"]
}
`))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST response code: got %v, want %v", got, want)
		}
		body, err := io.ReadAll(rsp.Body)
		if err != nil {
			t.Errorf("Reading POST body: %v", err)
		}

		const want = `{"jsonrpc":"2.0","id":1,"result":6}`
		if got := string(body); got != want {
			t.Errorf("POST body: got %#q, want %#q", got, want)
		}
	})

	// Verify that the bridge will accept a batch.
	t.Run("PostBatchOK", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "application/json", strings.NewReader(`[
  {"jsonrpc":"2.0", "id": 3, "method": "Test1", "params": ["first"]},
  {"jsonrpc":"2.0", "id": 7, "method": "Test1", "params": ["among", "equals"]}
]
`))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST response code: got %v, want %v", got, want)
		}
		body, err := io.ReadAll(rsp.Body)
		if err != nil {
			t.Errorf("Reading POST body: %v", err)
		}

		const want = `[{"jsonrpc":"2.0","id":3,"result":1},` +
			`{"jsonrpc":"2.0","id":7,"result":2}]`
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
		body, err := io.ReadAll(rsp.Body)
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
		body, err := io.ReadAll(rsp.Body)
		if err != nil {
			t.Errorf("Reading POST body: %v", err)
		}
		if got := string(body); got != "" {
			t.Errorf("POST body: got %q, want empty", got)
		}
	})
}

// Verify that the content-type check hook works.
func TestBridge_contentTypeCheck(t *testing.T) {
	b := jhttp.NewBridge(testService, &jhttp.BridgeOptions{
		CheckContentType: func(ctype string) bool {
			return ctype == "application/octet-stream"
		},
	})
	defer checkClose(t, b)

	hsrv := httptest.NewServer(b)
	defer hsrv.Close()

	const reqTemplate = `{"jsonrpc":"2.0","id":%q,"method":"Test1","params":["a","b","c"]}`
	t.Run("ContentTypeOK", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "application/octet-stream",
			strings.NewReader(fmt.Sprintf(reqTemplate, "ok")))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST response code: got %v, want %v", got, want)
		}
	})

	t.Run("ContentTypeBad", func(t *testing.T) {
		rsp, err := http.Post(hsrv.URL, "text/plain",
			strings.NewReader(fmt.Sprintf(reqTemplate, "bad")))
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		} else if got, want := rsp.StatusCode, http.StatusUnsupportedMediaType; got != want {
			t.Errorf("POST response code: got %v, want %v", got, want)
		}
	})
}

func TestChannel(t *testing.T) {
	b := jhttp.NewBridge(testService, nil)
	defer checkClose(t, b)
	hsrv := httptest.NewServer(b)
	defer hsrv.Close()

	tests := []struct {
		params interface{}
		want   int
	}{
		{nil, 0},
		{[]string{"foo"}, 7},         // ["foo"]
		{map[string]int{"hi": 3}, 8}, // {"hi":3}
	}

	var callCount int
	ctx := context.Background()
	for _, opts := range []*jhttp.ChannelOptions{nil, {
		Client: counter{&callCount, http.DefaultClient},
	}} {
		ch := jhttp.NewChannel(hsrv.URL, opts)
		cli := jrpc2.NewClient(ch, nil)

		for _, test := range tests {
			var got int
			if err := cli.CallResult(ctx, "Test2", test.params, &got); err != nil {
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
			t.Errorf("Recv = (%#q, %v), want (nil, %v)", string(got), err, io.EOF)
		}
	}

	if callCount != len(tests) {
		t.Errorf("Channel client call count: got %d, want %d", callCount, len(tests))
	}
}

// counter implements the HTTPClient interface via a real HTTP client.  As a
// side effect it counts the number of invocations of its signature method.
type counter struct {
	z *int
	c *http.Client
}

func (c counter) Do(req *http.Request) (*http.Response, error) {
	defer func() { *c.z++ }()
	return c.c.Do(req)
}

func checkClose(t *testing.T, c io.Closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Errorf("Error in Close: %v", err)
	}
}
