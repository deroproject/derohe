package code

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestRegistration(t *testing.T) {
	const message = "fun for the whole family"
	c := Register(-100, message)
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
			t.Fatalf("Register should have panicked on input %d, but did not", ParseError)
		}
	}()
	Register(int32(ParseError), "bogus")
}

type testCoder Code

func (t testCoder) Code() Code  { return Code(t) }
func (testCoder) Error() string { return "bogus" }

func TestFromError(t *testing.T) {
	tests := []struct {
		input error
		want  Code
	}{
		{nil, NoError},
		{testCoder(ParseError), ParseError},
		{testCoder(InvalidRequest), InvalidRequest},
		{fmt.Errorf("wrapped parse error: %w", ParseError.Err()), ParseError},
		{context.Canceled, Cancelled},
		{fmt.Errorf("wrapped cancellation: %w", context.Canceled), Cancelled},
		{context.DeadlineExceeded, DeadlineExceeded},
		{fmt.Errorf("wrapped deadline: %w", context.DeadlineExceeded), DeadlineExceeded},
		{errors.New("other"), SystemError},
		{io.EOF, SystemError},
	}
	for _, test := range tests {
		if got := FromError(test.input); got != test.want {
			t.Errorf("FromError(%v): got %v, want %v", test.input, got, test.want)
		}
	}
}

func TestCodeIs(t *testing.T) {
	tests := []struct {
		code Code
		err  error
		want bool
	}{
		{NoError, nil, true},
		{0, nil, false},
		{1, Code(1).Err(), true},
		{2, Code(3).Err(), false},
		{4, fmt.Errorf("blah: %w", Code(4).Err()), true},
		{5, fmt.Errorf("nope: %w", Code(6).Err()), false},
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
		code Code
		want error
	}
	Register(1, "look for the bear necessities")
	Register(2, "the simple bear necessities")
	tests := []test{
		{NoError, nil},
		{0, errors.New("error code 0")},
		{1, errors.New("look for the bear necessities")},
		{-17, errors.New("error code -17")},
	}

	// Make sure all the pre-defined errors get their messages hit.
	for code, msg := range stdError {
		if code == NoError {
			continue
		}
		tests = append(tests, test{
			code: code,
			want: errors.New(msg),
		})
	}
	for _, test := range tests {
		got := test.code.Err()
		if !eqv(got, test.want) {
			t.Errorf("Code(%d).Err(): got %#v, want %#v", test.code, got, test.want)
		} else {
			t.Logf("Code(%d).Err() ok: %v", test.code, got)
		}
		if c := FromError(got); c != test.code {
			t.Errorf("Code(%d).Err(): got code %v, want %v", test.code, c, test.code)
		}
	}
}
