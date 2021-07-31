// Package jhttp implements a bridge from HTTP to JSON-RPC.  This permits
// requests to be submitted to a JSON-RPC server using HTTP as a transport.
package jhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/server"
)

// A Bridge is a http.Handler that bridges requests to a JSON-RPC server.
//
// The body of the HTTP POST request must contain the complete JSON-RPC request
// message, encoded with Content-Type: application/json. Either a single
// request object or a list of request objects is supported.
//
// If the request completes, whether or not there is an error, the HTTP
// response is 200 (OK) for ordinary requests or 204 (No Response) for
// notifications, and the response body contains the JSON-RPC response.
//
// If the HTTP request method is not "POST", the bridge reports 405 (Method Not
// Allowed). If the Content-Type is not application/json, the bridge reports
// 415 (Unsupported Media Type).
//
// The bridge attaches the inbound HTTP request to the context passed to the
// client, allowing an EncodeContext callback to retrieve state from the HTTP
// headers. Use jhttp.HTTPRequest to retrieve the request from the context.
type Bridge struct {
	local     server.Local
	checkType func(string) bool
}

// ServeHTTP implements the required method of http.Handler.
func (b Bridge) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !b.checkType(req.Header.Get("Content-Type")) {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	if err := b.serveInternal(w, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
	}
}

func (b Bridge) serveInternal(w http.ResponseWriter, req *http.Request) error {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}

	// The HTTP request requires a response, but the server will not reply if
	// all the requests are notifications. Check whether we have any calls
	// needing a response, and choose whether to wait for a reply based on that.
	//
	// Note that we are forgiving about a missing version marker in a request,
	// since we can't tell at this point whether the server is willing to accept
	// messages like that.
	jreq, err := jrpc2.ParseRequests(body)
	if err != nil && err != jrpc2.ErrInvalidVersion {
		return err
	}

	// Because the bridge shares the JSON-RPC client between potentially many
	// HTTP clients, we must virtualize the ID space for requests to preserve
	// the HTTP client's assignment of IDs.
	//
	// To do this, we keep track of the inbound ID for each request so that we
	// can map the responses back. This takes advantage of the fact that the
	// *jrpc2.Client detangles batch order so that responses come back in the
	// same order (modulo notifications) even if the server response did not
	// preserve order.

	// Generate request specifications for the client.
	var inboundID []string                // for requests
	spec := make([]jrpc2.Spec, len(jreq)) // requests & notifications
	for i, req := range jreq {
		spec[i] = jrpc2.Spec{
			Method: req.Method(),
			Notify: req.IsNotification(),
		}
		if req.HasParams() {
			var p json.RawMessage
			req.UnmarshalParams(&p)
			spec[i].Params = p
		}
		if !spec[i].Notify {
			inboundID = append(inboundID, req.ID())
		}
	}

	// Attach the HTTP request to the client context, so the encoder can see it.
	ctx := context.WithValue(req.Context(), httpReqKey{}, req)
	rsps, err := b.local.Client.Batch(ctx, spec)
	if err != nil {
		return err
	}

	// If all the requests were notifications, report success without responses.
	if len(rsps) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	// Otherwise, map the responses back to their original IDs, and marshal the
	// response back into the body.
	for i, rsp := range rsps {
		rsp.SetID(inboundID[i])
	}

	// If the original request was a single message, make sure we encode the
	// response the same way.
	var reply []byte
	if len(rsps) == 1 && !bytes.HasPrefix(bytes.TrimSpace(body), []byte("[")) {
		reply, err = json.Marshal(rsps[0])
	} else {
		reply, err = json.Marshal(rsps)
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(reply)))
	w.Write(reply)
	return nil
}

// Close closes the channel to the server, waits for the server to exit, and
// reports its exit status.
func (b Bridge) Close() error { return b.local.Close() }

// NewBridge constructs a new Bridge that starts a server on mux and dispatches
// HTTP requests to it.  The server will run until the bridge is closed.
//
// Note that a bridge is not able to push calls or notifications from the
// server back to the remote client. The bridge client is shared by multiple
// active HTTP requests, and has no way to know which of the callers the push
// should be forwarded to. You can enable push on the bridge server and set
// hooks on the bridge client as usual, but the remote client will not see push
// messages from the server.
func NewBridge(mux jrpc2.Assigner, opts *BridgeOptions) Bridge {
	return Bridge{
		local: server.NewLocal(mux, &server.LocalOptions{
			Client: opts.clientOptions(),
			Server: opts.serverOptions(),
		}),
		checkType: opts.checkContentType(),
	}
}

// BridgeOptions are optional settings for a Bridge. A nil pointer is ready for
// use and provides default values as described.
type BridgeOptions struct {
	// Options for the bridge client (default nil).
	Client *jrpc2.ClientOptions

	// Options for the bridge server (default nil).
	Server *jrpc2.ServerOptions

	// If non-nil, this function is called to check whether the HTTP request's
	// declared content-type is valid. If this function returns false, the
	// request is rejected. If nil, the default check requires a content type of
	// "application/json".
	CheckContentType func(contentType string) bool
}

func (o *BridgeOptions) clientOptions() *jrpc2.ClientOptions {
	if o == nil {
		return nil
	}
	return o.Client
}

func (o *BridgeOptions) serverOptions() *jrpc2.ServerOptions {
	if o == nil {
		return nil
	}
	return o.Server
}

func (o *BridgeOptions) checkContentType() func(string) bool {
	if o == nil || o.CheckContentType == nil {
		return func(ctype string) bool { return ctype == "application/json" }
	}
	return o.CheckContentType
}

type httpReqKey struct{}

// HTTPRequest returns the HTTP request associated with ctx, or nil. The
// context passed to the JSON-RPC client by the Bridge will contain this value.
func HTTPRequest(ctx context.Context) *http.Request {
	req, ok := ctx.Value(httpReqKey{}).(*http.Request)
	if ok {
		return req
	}
	return nil
}
