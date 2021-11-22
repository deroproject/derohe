package jhttp_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jhttp"
)

func Example() {
	// Set up a bridge to demonstrate the API.
	b := jhttp.NewBridge(handler.Map{
		"Test": handler.New(func(ctx context.Context, ss []string) (string, error) {
			return strings.Join(ss, " "), nil
		}),
	}, nil)
	defer b.Close()

	hsrv := httptest.NewServer(b)
	defer hsrv.Close()

	rsp, err := http.Post(hsrv.URL, "application/json", strings.NewReader(`{
  "jsonrpc": "2.0",
  "id": 10235,
  "method": "Test",
  "params": ["full", "plate", "and", "packing", "steel"]
}`))
	if err != nil {
		log.Fatalf("POST request failed: %v", err)
	}
	body, err := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	if err != nil {
		log.Fatalf("Reading response body: %v", err)
	}

	fmt.Println(string(body))
	// Output:
	// {"jsonrpc":"2.0","id":10235,"result":"full plate and packing steel"}
}
