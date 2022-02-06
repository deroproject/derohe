// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jhttp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/server"
)

// A Getter is a http.Handler that bridges GET requests to a JSON-RPC server.
//
// The JSON-RPC method name and parameters are decoded from the request URL.
// The results from a successful call are encoded as JSON in the response body
// with status 200 (OK). In case of error, the response body is a JSON-RPC
// error object, and the HTTP status is one of the following:
//
//  Condition               HTTP Status
//  ----------------------- -----------------------------------
//  Parsing request         400 (Bad request)
//  Method not found        404 (Not found)
//  (other errors)          500 (Internal server error)
//
// By default, the URL path identifies the JSON-RPC method, and the URL query
// parameters are converted into a JSON object for the parameters. Leading and
// trailing slashes are stripped from the path, and query values are sent as
// JSON strings.
//
// For example, this URL:
//
//    http://site.org:2112/some/method?param1=xyzzy&param2=apple
//
// would produce the method name "some/method" and this parameter object:
//
//    {"param1":"xyzzy", "param2":"apple"}
//
// To override the default behaviour, set a ParseRequest hook in GetterOptions.
// See also the jhttp.ParseQuery function for a more expressive translation.
type Getter struct {
	local    server.Local
	parseReq func(*http.Request) (string, interface{}, error)
}

// NewGetter constructs a new Getter that starts a server on mux and dispatches
// HTTP requests to it. The server will run until the getter is closed.
//
// Note that a getter is not able to push calls or notifications from the
// server back to the remote client even if enabled.
func NewGetter(mux jrpc2.Assigner, opts *GetterOptions) Getter {
	return Getter{
		local: server.NewLocal(mux, &server.LocalOptions{
			Client: opts.clientOptions(),
			Server: opts.serverOptions(),
		}),
		parseReq: opts.parseRequest(),
	}
}

// ServeHTTP implements the required method of http.Handler.
func (g Getter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	method, params, err := g.parseHTTPRequest(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, &jrpc2.Error{
			Code:    code.ParseError,
			Message: err.Error(),
		})
		return
	}

	ctx := context.WithValue(req.Context(), httpReqKey{}, req)
	var result json.RawMessage
	if err := g.local.Client.CallResult(ctx, method, params, &result); err != nil {
		var status int
		switch code.FromError(err) {
		case code.MethodNotFound:
			status = http.StatusNotFound
		default:
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Close closes the channel to the server, waits for the server to exit, and
// reports its exit status.
func (g Getter) Close() error { return g.local.Close() }

func (g Getter) parseHTTPRequest(req *http.Request) (string, interface{}, error) {
	if g.parseReq != nil {
		return g.parseReq(req)
	}
	if err := req.ParseForm(); err != nil {
		return "", nil, err
	}
	method := strings.Trim(req.URL.Path, "/")
	if method == "" {
		return "", nil, errors.New("empty method name")
	}
	params := make(map[string]string)
	for key := range req.Form {
		params[key] = req.Form.Get(key)
	}
	return method, params, nil
}

// GetterOptions are optional settings for a Getter. A nil pointer is ready for
// use and provides default values as described.
type GetterOptions struct {
	// Options for the getter client (default nil).
	Client *jrpc2.ClientOptions

	// Options for the getter server (default nil).
	Server *jrpc2.ServerOptions

	// If set, this function is called to parse a method name and request
	// parameters from an HTTP request. If this is not set, the default handler
	// uses the URL path as the method name and the URL query as the method
	// parameters.
	ParseRequest func(*http.Request) (string, interface{}, error)
}

func (o *GetterOptions) clientOptions() *jrpc2.ClientOptions {
	if o == nil {
		return nil
	}
	return o.Client
}

func (o *GetterOptions) serverOptions() *jrpc2.ServerOptions {
	if o == nil {
		return nil
	}
	return o.Server
}

func (o *GetterOptions) parseRequest() func(*http.Request) (string, interface{}, error) {
	if o == nil {
		return nil
	}
	return o.ParseRequest
}

func writeJSON(w http.ResponseWriter, code int, obj interface{}) {
	bits, err := json.Marshal(obj)
	if err != nil {
		// Fallback in case of marshaling error. This should not happen, but
		// ensures the client gets a loggable reply from a broken server.
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(bits)))
	w.WriteHeader(code)
	w.Write(bits)
}

// ParseQuery parses a request URL and constructs a parameter map from the
// query values encoded in the URL and/or request body.
//
// The method name is the URL path, with leading and trailing slashes trimmed.
// Query values are converted into argument values by these rules:
//
// Double-quoted values are interpreted as JSON string values, with the same
// encoding and escaping rules (UTF-8 with backslash escapes). Examples:
//
//    ""
//    "foo\nbar"
//    "a \"string\" of text"
//
// Values that consist of decimal digits and an optional leading sign are
// treated as either int64 (if there is no decimal point) or float64 values.
// Examples:
//
//    25
//    -16
//    3.259
//
// The unquoted strings "true" and "false" are converted to the corresponding
// Boolean values. The unquoted string "null" is converted to nil.
//
// To express arbitrary bytes, use a singly-quoted string encoded in base64.
// For example:
//
//    'aGVsbG8sIHdvcmxk'   -- represents "hello, world"
//
// All values not matching any of the above are treated as literal strings.
//
// On success, the result has concrete type map[string]interface{} and the
// method name is not empty.
func ParseQuery(req *http.Request) (string, interface{}, error) {
	if err := req.ParseForm(); err != nil {
		return "", nil, err
	}
	method := strings.Trim(req.URL.Path, "/")
	if method == "" {
		return "", nil, errors.New("empty URL path")
	}
	if len(req.Form) == 0 {
		return method, nil, nil
	}

	params := make(map[string]interface{})
	for key := range req.Form {
		val := req.Form.Get(key)
		if v, ok, err := parseJSONString(val); err != nil {
			return "", nil, fmt.Errorf("decoding string %q: %w", key, err)
		} else if ok {
			params[key] = v
		} else if n, ok := parseNumber(val); ok {
			params[key] = n
		} else if b, ok := parseConstant(val); ok {
			params[key] = b
		} else if d, ok, err := parseQuoted64(val); err != nil {
			return "", nil, fmt.Errorf("decoding bytes %q: %w", key, err)
		} else if ok {
			params[key] = d
		} else {
			params[key] = val
		}
	}
	return method, params, nil
}

func parseJSONString(s string) (string, bool, error) {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			var dec string
			err := json.Unmarshal([]byte(s), &dec)
			if err != nil {
				return "", false, err
			}
			return dec, true, nil
		} else if s[0] == '"' || s[len(s)-1] == '"' {
			return "", false, errors.New("missing string quote")
		}
	}
	return "", false, nil
}

func parseNumber(s string) (interface{}, bool) {
	z, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return z, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return v, true
	}
	return nil, false
}

func parseConstant(s string) (interface{}, bool) {
	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	default:
		return nil, false
	}
}

func parseQuoted64(s string) ([]byte, bool, error) {
	if len(s) >= 2 {
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			trim := strings.TrimRight(s[1:len(s)-1], "=") // discard base64 padding
			dec, err := base64.RawStdEncoding.DecodeString(trim)
			return dec, err == nil, err
		} else if s[0] == '\'' || s[len(s)-1] == '\'' {
			return nil, false, errors.New("missing bytes quote")
		}
	}
	return nil, false, nil
}
