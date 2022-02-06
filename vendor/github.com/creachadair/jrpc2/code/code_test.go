// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package code_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/creachadair/jrpc2/code"
)

func TestRegistration(t *testing.T) {
	const message = "fun for the whole family"
	c := code.Register(-100, message)
	if got := c.String(); got != message {
		t.Errorf("Register(-100): got %q, want %q", got, message)
	} else if c != -100 {
		t.Errorf("Register(-100): got %d instead", c)
	}
}

func TestRegistrationError(t *testing.T) {
	defer func() {
		if v := recover(); v != nil {
			t.Logf("Register correctly panicked: %v", v)
		} else {
			t.Fatalf("Register should have panicked on input %d, but did not", code.ParseError)
		}
	}()
	code.Register(int32(code.ParseError), "bogus")
}

type testCoder code.Code

func (t testCoder) ErrCode() code.Code { return code.Code(t) }
func (testCoder) Error() string        { return "bogus" }

func TestFromError(t *testing.T) {
	tests := []struct {
		input error
		want  code.Code
	}{
		{nil, code.NoError},
		{testCoder(code.ParseError), code.ParseError},
		{testCoder(code.InvalidRequest), code.InvalidRequest},
		{fmt.Errorf("wrapped parse error: %w", code.ParseError.Err()), code.ParseError},
		{context.Canceled, code.Cancelled},
		{fmt.Errorf("wrapped cancellation: %w", context.Canceled), code.Cancelled},
		{context.DeadlineExceeded, code.DeadlineExceeded},
		{fmt.Errorf("wrapped deadline: %w", context.DeadlineExceeded), code.DeadlineExceeded},
		{errors.New("other"), code.SystemError},
		{io.EOF, code.SystemError},
	}
	for _, test := range tests {
		if got := code.FromError(test.input); got != test.want {
			t.Errorf("FromError(%v): got %v, want %v", test.input, got, test.want)
		}
	}
}

func TestCodeIs(t *testing.T) {
	tests := []struct {
		code code.Code
		err  error
		want bool
	}{
		{code.NoError, nil, true},
		{0, nil, false},
		{1, code.Code(1).Err(), true},
		{2, code.Code(3).Err(), false},
		{4, fmt.Errorf("blah: %w", code.Code(4).Err()), true},
		{5, fmt.Errorf("nope: %w", code.Code(6).Err()), false},
	}
	for _, test := range tests {
		cerr := test.code.Err()
		got := errors.Is(test.err, cerr)
		if got != test.want {
			t.Errorf("Is(%v, %v): got %v, want %v", test.err, cerr, got, test.want)
		}
	}
}

func TestErr(t *testing.T) {
	eqv := func(e1, e2 error) bool {
		return e1 == e2 || (e1 != nil && e2 != nil && e1.Error() == e2.Error())
	}
	type test struct {
		code code.Code
		want error
	}
	code.Register(1, "look for the bear necessities")
	code.Register(2, "the simple bear necessities")
	tests := []test{
		{code.NoError, nil},
		{0, errors.New("error code 0")},
		{1, errors.New("look for the bear necessities")},
		{-17, errors.New("error code -17")},
	}

	// Make sure all the pre-defined errors get their messages hit.
	for _, v := range []int32{
		// Codes reserved by the JSON-RPC 2.0 spec.
		-32700, -32600, -32601, -32602, -32603,
		// Codes reserved by this implementation.
		-32098, -32097, -32096,
	} {
		c := code.Code(v)
		tests = append(tests, test{
			code: c,
			want: errors.New(c.String()),
		})
	}
	for _, test := range tests {
		got := test.code.Err()
		if !eqv(got, test.want) {
			t.Errorf("Code(%d).Err(): got %#v, want %#v", test.code, got, test.want)
		}
		if c := code.FromError(got); c != test.code {
			t.Errorf("Code(%d).Err(): got code %v, want %v", test.code, c, test.code)
		}
	}
}
