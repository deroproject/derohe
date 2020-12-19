package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/google/go-cmp/cmp"
)

// Verify that the New function correctly handles the various type signatures
// it's advertised to support, and not others.
func TestNew(t *testing.T) {
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
		{v: func(context.Context, ...string) (bool, error) { return false, nil }},
		{v: func(context.Context, *jrpc2.Request) (byte, error) { return '0', nil }},
		{v: func(context.Context) bool { return true }},
		{v: func(context.Context, int) bool { return true }},

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
		got, err := newHandler(test.v)
		if !test.bad && err != nil {
			t.Errorf("newHandler(%T): unexpected error: %v", test.v, err)
		} else if test.bad && err == nil {
			t.Errorf("newHandler(%T): got %+v, want error", test.v, got)
		}
	}
}

type dummy struct{}

func (dummy) Y1(context.Context) (int, error) { return 0, nil }

func (dummy) N1(string) {}

func (dummy) Y2(_ context.Context, vs ...int) (int, error) { return len(vs), nil }

func (dummy) N2() bool { return false }

func (dummy) Y3(context.Context) error { return errors.New("blah") }

//lint:ignore U1000 verify unexported methods are not assigned
func (dummy) n3(context.Context, []string) error { return nil }

// Verify that the NewService function obtains the correct functions.
func TestNewService(t *testing.T) {
	var stub dummy
	ctx := context.Background()
	m := NewService(stub)
	for _, test := range []string{"Y1", "Y2", "Y3", "N1", "N2", "n3", "foo"} {
		got := m.Assign(ctx, test) != nil
		want := strings.HasPrefix(test, "Y")
		if got != want {
			t.Errorf("Assign %q: got %v, want %v", test, got, want)
		}
	}
}

// Verify that a stub with no usable methods panics.
func TestEmptyService(t *testing.T) {
	type empty struct{}

	defer func() {
		if x := recover(); x != nil {
			t.Logf("Received expected panic: %v", x)
		}
	}()
	m := NewService(empty{})
	t.Fatalf("NewService(empty): got %v, want panic", m)
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
	m := ServiceMap{"Test": NewService(dummy{})}
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
		args Args
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
		{`["foo", 25]`, Args{&tmp.S, &tmp.Z}, stuff{S: "foo", Z: 25}, true},
		{`[25, "foo"]`, Args{&tmp.Z, &tmp.S}, stuff{S: "foo", Z: 25}, true},

		{`[true, 3.5, "blah"]`, Args{&tmp.B, &tmp.F, &tmp.S},
			stuff{S: "blah", B: true, F: 3.5}, true},

		// Skip values with a nil corresponding argument.
		{`[true, 101, "ignored"]`, Args{&tmp.B, &tmp.Z, nil},
			stuff{B: true, Z: 101}, true},
		{`[true, 101, "observed"]`, Args{&tmp.B, nil, &tmp.S},
			stuff{B: true, S: "observed"}, true},

		// Mismatched argument/value count.
		{`["wrong"]`, Args{&tmp.S, &tmp.Z}, stuff{}, false},   // too few values
		{`["really", "wrong"]`, Args{&tmp.S}, stuff{}, false}, // too many values

		// Mismatched argument/value types.
		{`["nope"]`, Args{&tmp.B}, stuff{}, false}, // wrong value type
		{`[{}]`, Args{&tmp.F}, stuff{}, false},     // "
	}
	for _, test := range tests {
		tmp = stuff{} // reset
		if err := json.Unmarshal([]byte(test.json), &test.args); err != nil {
			if test.ok {
				t.Errorf("Unmarshal %#q: unexpected error: %v", test.json, err)
			} else {
				t.Logf("Unmarshal %#q: got expected error: %v", test.json, err)
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
		got, err := json.Marshal(Args(test.input))
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
		obj   Obj
		want  *values
	}{
		{"", nil, nil},                      // error: empty text
		{"true", nil, nil},                  // error: not an object
		{"[]", nil, nil},                    // error: not an object
		{`{"x":true}`, Obj{"x": &v.S}, nil}, // error: wrong type

		// Nothing to unpack, no place to put it.
		{"{}", nil, &values{}},

		// Ignore non-matching keys but keep matching ones.
		{`{"apple":true, "laser":"sauce"}`, Obj{"laser": &v.S}, &values{S: "sauce"}},

		// Assign to matching fields including compound types.
		{`{"x": 25, "q": "snark", "sub": {"foo":"bark"}, "yawp": false, "#":[5,3,2,4,7]}`, Obj{
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

func ExampleArgs_unmarshal() {
	const input = `[25, false, "apple"]`

	var count int
	var item string

	if err := json.Unmarshal([]byte(input), &Args{&count, nil, &item}); err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}
	fmt.Printf("count=%d, item=%q\n", count, item)
	// Output:
	// count=25, item="apple"
}

func ExampleArgs_marshal() {
	bits, err := json.Marshal(Args{1, "foo", false, nil})
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}
	fmt.Println(string(bits))
	// Output:
	// [1,"foo",false,null]
}

func ExampleObj_unmarshal() {
	const input = `{"uid": 501, "name": "P. T. Barnum", "tags": [1, 3]}`

	var uid int
	var name string

	if err := json.Unmarshal([]byte(input), &Obj{
		"uid":  &uid,
		"name": &name,
	}); err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}
	fmt.Printf("uid=%d, name=%q\n", uid, name)
	// Output:
	// uid=501, name="P. T. Barnum"
}
