// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/google/go-cmp/cmp"
)

func y1(context.Context) (int, error) { return 0, nil }

func y2(_ context.Context, vs []int) (int, error) { return len(vs), nil }

func y3(context.Context) error { return errors.New("blah") }

type argStruct struct {
	A string `json:"alpha"`
	B int    `json:"bravo"`
}

// Verify that the CHeck function correctly handles the various type signatures
// it's advertised to support, and not others.
func TestCheck(t *testing.T) {
	tests := []struct {
		v   interface{}
		bad bool
	}{
		{v: nil, bad: true},              // nil value
		{v: "not a function", bad: true}, // not a function

		// All the legal kinds...
		{v: func(context.Context) error { return nil }},
		{v: func(context.Context, *jrpc2.Request) (interface{}, error) { return nil, nil }},
		{v: func(context.Context) (int, error) { return 0, nil }},
		{v: func(context.Context, []int) error { return nil }},
		{v: func(context.Context, []bool) (float64, error) { return 0, nil }},
		{v: func(context.Context, *argStruct) int { return 0 }},
		{v: func(context.Context, *jrpc2.Request) error { return nil }},
		{v: func(context.Context, *jrpc2.Request) float64 { return 0 }},
		{v: func(context.Context, *jrpc2.Request) (byte, error) { return '0', nil }},
		{v: func(context.Context) bool { return true }},
		{v: func(context.Context, int) bool { return true }},
		{v: func(_ context.Context, s [1]string) string { return s[0] }},

		// Things that aren't supposed to work.
		{v: func() error { return nil }, bad: true},                           // wrong # of params
		{v: func(a, b, c int) bool { return false }, bad: true},               // ...
		{v: func(byte) {}, bad: true},                                         // wrong # of results
		{v: func(byte) (int, bool, error) { return 0, true, nil }, bad: true}, // ...
		{v: func(string) error { return nil }, bad: true},                     // missing context
		{v: func(a, b string) error { return nil }, bad: true},                // P1 is not context
		{v: func(context.Context) (int, bool) { return 1, true }, bad: true},  // R2 is not error

		//lint:ignore ST1008 verify permuted error position does not match
		{v: func(context.Context) (error, float64) { return nil, 0 }, bad: true}, // ...
	}
	for _, test := range tests {
		got, err := handler.Check(test.v)
		if !test.bad && err != nil {
			t.Errorf("Check(%T): unexpected error: %v", test.v, err)
		} else if test.bad && err == nil {
			t.Errorf("Check(%T): got %+v, want error", test.v, got)
		}
	}
}

// Verify that the wrappers constructed by FuncInfo.Wrap can properly decode
// their arguments of different types and structure.
func TestFuncInfo_wrapDecode(t *testing.T) {
	tests := []struct {
		fn   handler.Func
		p    string
		want interface{}
	}{
		// A positional handler should decode its argument from an array or an object.
		{handler.NewPos(func(_ context.Context, z int) int { return z }, "arg"),
			`[25]`, 25},
		{handler.NewPos(func(_ context.Context, z int) int { return z }, "arg"),
			`{"arg":109}`, 109},

		// A type with custom marshaling should be properly handled.
		{handler.NewPos(func(_ context.Context, b stringByte) byte { return byte(b) }, "arg"),
			`["00111010"]`, byte(0x3a)},
		{handler.NewPos(func(_ context.Context, b stringByte) byte { return byte(b) }, "arg"),
			`{"arg":"10011100"}`, byte(0x9c)},
		{handler.New(func(_ context.Context, v fauxStruct) int { return int(v) }),
			`{"type":"thing","value":99}`, 99},

		// Plain JSON should get its argument unmodified.
		{handler.New(func(_ context.Context, v json.RawMessage) string { return string(v) }),
			`{"x": true, "y": null}`, `{"x": true, "y": null}`},

		// Npn-positional slice argument.
		{handler.New(func(_ context.Context, ss []string) int { return len(ss) }),
			`["a", "b", "c"]`, 3},
	}
	ctx := context.Background()
	for _, test := range tests {
		req := mustParseRequest(t,
			fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"x","params":%s}`, test.p))
		got, err := test.fn(ctx, req)
		if err != nil {
			t.Errorf("Call %v failed: %v", test.fn, err)
		} else if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("Call %v: wrong result (-want, +got)\n%s", test.fn, diff)
		}
	}
}

// Verify that the Positional function correctly handles its cases.
func TestPositional(t *testing.T) {
	tests := []struct {
		v   interface{}
		n   []string
		bad bool
	}{
		{v: nil, bad: true},              // nil value
		{v: "not a function", bad: true}, // not a function

		// Things that should work.
		{v: func(context.Context) error { return nil }},
		{v: func(context.Context) int { return 1 }},
		{v: func(context.Context, bool) bool { return false },
			n: []string{"isTrue"}},
		{v: func(context.Context, int, int) int { return 0 },
			n: []string{"a", "b"}},
		{v: func(context.Context, string, int, []float64) int { return 0 },
			n: []string{"a", "b", "c"}},

		// Things that should not work.
		{v: func() error { return nil }, bad: true}, // no parameters
		{v: func(int) int { return 0 }, bad: true},  // first argument not context
		{v: func(context.Context, string) error { return nil },
			n: nil, bad: true}, // not enough names
		{v: func(context.Context, string, string, string) error { return nil },
			n: []string{"x", "y"}, bad: true}, // too many names
		{v: func(context.Context, string, ...float64) int { return 0 },
			n: []string{"goHome", "youAreDrunk"}, bad: true}, // variadic

		// N.B. Other cases are covered by TestCheck. The cases here are only
		// those that Positional checks for explicitly.
	}
	for _, test := range tests {
		got, err := handler.Positional(test.v, test.n...)
		if !test.bad && err != nil {
			t.Errorf("Positional(%T, %q): unexpected error: %v", test.v, test.n, err)
		} else if test.bad && err == nil {
			t.Errorf("Positional(%T, %q): got %+v, want error", test.v, test.n, got)
		}
	}
}

func TestNewStrict(t *testing.T) {
	type arg struct {
		A, B string
	}
	fn := handler.NewStrict(func(ctx context.Context, arg *arg) error { return nil })

	req := mustParseRequest(t, `{
   "jsonrpc": "2.0",
   "id":      100,
   "method":  "f",
   "params": {
      "A": "foo",
      "Z": 25
   }}`)
	rsp, err := fn(context.Background(), req)
	if got := code.FromError(err); got != code.InvalidParams {
		t.Errorf("Handler returned (%+v, %v), want InvalidParms", rsp, err)
	}
}

// Verify that a handler with no argument type does not panic attempting to
// enforce strict field checking.
func TestNewStrict_argumentRegression(t *testing.T) {
	defer func() {
		if x := recover(); x != nil {
			t.Fatalf("NewStrict panic: %v", x)
		}
	}()
	handler.NewStrict(func(context.Context) error { return nil })
}

// Verify that the handling of pointer-typed arguments does not incorrectly
// introduce another pointer indirection.
func TestNew_pointerRegression(t *testing.T) {
	var got argStruct
	call := handler.New(func(_ context.Context, arg *argStruct) error {
		got = *arg
		t.Logf("Got argument struct: %+v", got)
		return nil
	})
	req := mustParseRequest(t, `{
   "jsonrpc": "2.0",
   "id":      "foo",
   "method":  "bar",
   "params":{
      "alpha": "xyzzy",
      "bravo": 23
   }}`)
	if _, err := call.Handle(context.Background(), req); err != nil {
		t.Errorf("Handle failed: %v", err)
	}
	want := argStruct{A: "xyzzy", B: 23}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Wrong argStruct value: (-want, +got)\n%s", diff)
	}
}

// Verify that positional arguments are decoded properly.
func TestPositional_decode(t *testing.T) {
	fi, err := handler.Positional(func(ctx context.Context, a, b int) int {
		return a + b
	}, "first", "second")
	if err != nil {
		t.Fatalf("Positional: unexpected error: %v", err)
	}
	call := fi.Wrap()
	tests := []struct {
		input string
		want  int
		bad   bool
	}{
		{`{"jsonrpc":"2.0","id":1,"method":"add","params":{"first":5,"second":3}}`, 8, false},
		{`{"jsonrpc":"2.0","id":2,"method":"add","params":[5,3]}`, 8, false},
		{`{"jsonrpc":"2.0","id":3,"method":"add","params":{"first":5}}`, 5, false},
		{`{"jsonrpc":"2.0","id":4,"method":"add","params":{"second":3}}`, 3, false},
		{`{"jsonrpc":"2.0","id":5,"method":"add","params":{}}`, 0, false},
		{`{"jsonrpc":"2.0","id":6,"method":"add","params":null}`, 0, false},
		{`{"jsonrpc":"2.0","id":7,"method":"add"}`, 0, false},

		{`{"jsonrpc":"2.0","id":10,"method":"add","params":["wrong", "type"]}`, 0, true},
		{`{"jsonrpc":"2.0","id":12,"method":"add","params":[15, "wrong-type"]}`, 0, true},
		{`{"jsonrpc":"2.0","id":13,"method":"add","params":{"unknown":"field"}}`, 0, true},
		{`{"jsonrpc":"2.0","id":14,"method":"add","params":[1]}`, 0, true},     // too few
		{`{"jsonrpc":"2.0","id":15,"method":"add","params":[1,2,3]}`, 0, true}, // too many
	}
	for _, test := range tests {
		req := mustParseRequest(t, test.input)
		got, err := call(context.Background(), req)
		if !test.bad {
			if err != nil {
				t.Errorf("Call %#q: unexpected error: %v", test.input, err)
			} else if z := got.(int); z != test.want {
				t.Errorf("Call %#q: got %d, want %d", test.input, z, test.want)
			}
		} else if test.bad && err == nil {
			t.Errorf("Call %#q: got %v, want error", test.input, got)
		}
	}
}

// Verify that a ServiceMap assigns names correctly.
func TestServiceMap(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"nothing", false}, // not a known service
		{"Test", false},    // no method in the service
		{"Test.", false},   // empty method name in service
		{"Test.Y1", true},  // OK
		{"Test.Y2", true},
		{"Test.Y3", true},
		{"Test.Y4", false},
		{"Test.N1", false},
		{"Test.N2", false},
	}
	ctx := context.Background()
	m := handler.ServiceMap{"Test": handler.Map{
		"Y1": handler.New(y1),
		"Y2": handler.New(y2),
		"Y3": handler.New(y3),
	}}
	for _, test := range tests {
		got := m.Assign(ctx, test.name) != nil
		if got != test.want {
			t.Errorf("Assign(%q): got %v, want %v", test.name, got, test.want)
		}
	}

	got, want := m.Names(), []string{"Test.Y1", "Test.Y2", "Test.Y3"} // sorted
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Wrong method names: (-want, +got)\n%s", diff)
	}
}

// Verify that argument decoding works.
func TestArgs(t *testing.T) {
	type stuff struct {
		S string
		Z int
		F float64
		B bool
	}
	var tmp stuff
	tests := []struct {
		json string
		args handler.Args
		want stuff
		ok   bool
	}{
		{``, nil, stuff{}, false},     // incomplete
		{`{}`, nil, stuff{}, false},   // wrong type (object)
		{`true`, nil, stuff{}, false}, // wrong type (bool)

		{`[]`, nil, stuff{}, true},
		{`[ ]`, nil, stuff{}, true},
		{`null`, nil, stuff{}, true},

		// Respect order of arguments and values.
		{`["foo", 25]`, handler.Args{&tmp.S, &tmp.Z}, stuff{S: "foo", Z: 25}, true},
		{`[25, "foo"]`, handler.Args{&tmp.Z, &tmp.S}, stuff{S: "foo", Z: 25}, true},

		{`[true, 3.5, "blah"]`, handler.Args{&tmp.B, &tmp.F, &tmp.S},
			stuff{S: "blah", B: true, F: 3.5}, true},

		// Skip values with a nil corresponding argument.
		{`[true, 101, "ignored"]`, handler.Args{&tmp.B, &tmp.Z, nil},
			stuff{B: true, Z: 101}, true},
		{`[true, 101, "observed"]`, handler.Args{&tmp.B, nil, &tmp.S},
			stuff{B: true, S: "observed"}, true},

		// Mismatched argument/value count.
		{`["wrong"]`, handler.Args{&tmp.S, &tmp.Z}, stuff{}, false},   // too few values
		{`["really", "wrong"]`, handler.Args{&tmp.S}, stuff{}, false}, // too many values

		// Mismatched argument/value types.
		{`["nope"]`, handler.Args{&tmp.B}, stuff{}, false}, // wrong value type
		{`[{}]`, handler.Args{&tmp.F}, stuff{}, false},     // "
	}
	for _, test := range tests {
		tmp = stuff{} // reset
		if err := json.Unmarshal([]byte(test.json), &test.args); err != nil {
			if test.ok {
				t.Errorf("Unmarshal %#q: unexpected error: %v", test.json, err)
			}
			continue
		}

		if diff := cmp.Diff(test.want, tmp); diff != "" {
			t.Errorf("Unmarshal %#q: (-want, +got)\n%s", test.json, diff)
		}
	}
}

func TestArgsMarshal(t *testing.T) {
	tests := []struct {
		input []interface{}
		want  string
	}{
		{nil, "[]"},
		{[]interface{}{}, "[]"},
		{[]interface{}{12345}, "[12345]"},
		{[]interface{}{"hey you"}, `["hey you"]`},
		{[]interface{}{true, false}, "[true,false]"},
		{[]interface{}{nil, 3.5}, "[null,3.5]"},
		{[]interface{}{[]string{"a", "b"}, 33}, `[["a","b"],33]`},
		{[]interface{}{1, map[string]string{
			"ok": "yes",
		}, 3}, `[1,{"ok":"yes"},3]`},
	}
	for _, test := range tests {
		got, err := json.Marshal(handler.Args(test.input))
		if err != nil {
			t.Errorf("Marshal %+v: unexpected error: %v", test.input, err)
		} else if s := string(got); s != test.want {
			t.Errorf("Marshal %+v: got %#q, want %#q", test.input, s, test.want)
		}
	}
}

func TestObjUnmarshal(t *testing.T) {
	// N.B. Exported field names here to satisfy cmp.Diff.
	type sub struct {
		Foo string `json:"foo"`
	}
	type values struct {
		Z int
		S string
		T sub
		L []int
	}
	var v values

	tests := []struct {
		input string
		obj   handler.Obj
		want  *values
	}{
		{"", nil, nil},     // error: empty text
		{"true", nil, nil}, // error: not an object
		{"[]", nil, nil},   // error: not an object
		{`{"x":true}`, handler.Obj{"x": &v.S}, nil}, // error: wrong type

		// Nothing to unpack, no place to put it.
		{"{}", nil, &values{}},

		// Ignore non-matching keys but keep matching ones.
		{`{"apple":true, "laser":"sauce"}`, handler.Obj{"laser": &v.S}, &values{S: "sauce"}},

		// Assign to matching fields including compound types.
		{`{"x": 25, "q": "snark", "sub": {"foo":"bark"}, "yawp": false, "#":[5,3,2,4,7]}`, handler.Obj{
			"x":   &v.Z,
			"q":   &v.S,
			"sub": &v.T,
			"#":   &v.L,
		}, &values{
			Z: 25,
			S: "snark",
			T: sub{Foo: "bark"},
			L: []int{5, 3, 2, 4, 7},
		}},
	}
	for _, test := range tests {
		v = values{} // reset

		if err := json.Unmarshal([]byte(test.input), &test.obj); err != nil {
			if test.want == nil {
				t.Logf("Unmarshal: got expected error: %v", err)
			} else {
				t.Errorf("Unmarshal %q: %v", test.input, err)
			}
			continue
		}
		if diff := cmp.Diff(*test.want, v); diff != "" {
			t.Errorf("Wrong values: (-want, +got)\n%s", diff)
		}
	}
}

func mustParseRequest(t *testing.T, text string) *jrpc2.Request {
	t.Helper()
	req, err := jrpc2.ParseRequests([]byte(text))
	if err != nil {
		t.Fatalf("ParseRequests: %v", err)
	} else if len(req) != 1 {
		t.Fatalf("Wrong number of requests: got %d, want 1", len(req))
	}
	return req[0]
}

// stringByte is a byte with a custom JSON encoding. It expects a string of
// decimal digits 1 and 0, e.g., "10011000" == 0x98.
type stringByte byte

func (s *stringByte) UnmarshalText(text []byte) error {
	v, err := strconv.ParseUint(string(text), 2, 8)
	if err != nil {
		return err
	}
	*s = stringByte(v)
	return nil
}

// fauxStruct is an integer with a custom JSON encoding. It expects an object:
//
//   {"type":"thing","value":<integer>}
//
type fauxStruct int

func (s *fauxStruct) UnmarshalJSON(data []byte) error {
	var tmp struct {
		T string `json:"type"`
		V *int   `json:"value"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	} else if tmp.T != "thing" {
		return fmt.Errorf("unknown type %q", tmp.T)
	} else if tmp.V == nil {
		return errors.New("missing value")
	}
	*s = fauxStruct(*tmp.V)
	return nil
}
