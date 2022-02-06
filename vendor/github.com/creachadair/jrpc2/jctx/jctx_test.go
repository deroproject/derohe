// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package jctx

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

var bicent = time.Date(1976, 7, 4, 1, 2, 3, 4, time.UTC)

func TestEncoding(t *testing.T) {
	tests := []struct {
		desc         string
		deadline     time.Time
		params, want string
	}{
		{"zero-void", time.Time{}, "", `{"jctx":"1"}`},

		{"zero-payload", time.Time{},
			"[1,2,3]", `{"jctx":"1","payload":[1,2,3]}`},

		{"bicentennial-void", bicent.In(time.Local),
			"", `{"jctx":"1","deadline":"1976-07-04T01:02:03.000000004Z"}`,
		},

		{"bicentennial-payload", bicent,
			`{"apple":"pear"}`,
			`{"jctx":"1","deadline":"1976-07-04T01:02:03.000000004Z","payload":{"apple":"pear"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ctx := context.Background()
			if !test.deadline.IsZero() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, test.deadline)
				defer cancel()
			}
			raw, err := Encode(ctx, "dummy", json.RawMessage(test.params))
			if err != nil {
				t.Errorf("Encoding %q failed: %v", test.params, err)
			} else if got := string(raw); got != test.want {
				t.Errorf("Encoding %q: got %#q, want %#q", test.params, got, test.want)
			}
		})
	}
}

func TestDecoding(t *testing.T) {
	tests := []struct {
		desc, input string
		deadline    time.Time
		want        string
	}{
		{"empty context", "", time.Time{}, ""},
		{"empty parameters", `{"jctx":"1"}`, time.Time{}, ""},
		{"non-object input", `[1, 5]`, time.Time{}, `[1, 5]`},
		{"non-context empty object", `{}`, time.Time{}, `{}`},
		{"non-context object", `{"kiss":"me"}`, time.Time{}, `{"kiss":"me"}`},
		{"invalid context", `{"jctx":2, "ok":true}`, time.Time{}, `{"jctx":2, "ok":true}`},

		{"zero-payload", `{"jctx":"1","payload":["a","b","c"]}`, time.Time{}, `["a","b","c"]`},
		{"zero-payload-naked", `["a", "b", "c"]`, time.Time{}, `["a", "b", "c"]`},

		{"bicentennial-void", `{"jctx":"1","deadline":"1976-07-04T01:02:03.000000004Z"}`, bicent, ""},

		{"bicentennial-payload", `{
"jctx":"1",
"deadline":"1976-07-04T01:02:03.000000004Z",
"payload":{"lhs":1,"rhs":2}
}`, bicent, `{"lhs":1,"rhs":2}`},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ctx := context.Background()
			gotctx, params, err := Decode(ctx, "dummy", json.RawMessage(test.input))
			if err != nil {
				t.Fatalf("Decode(_, %q): error: %v", test.input, err)
			}
			if !test.deadline.IsZero() {
				dl, ok := gotctx.Deadline()
				if !ok {
					t.Error("Decode: missing expected deadline")
				} else if !dl.Equal(test.deadline) {
					t.Errorf("Decode deadline: got %v, want %v", dl, test.deadline)
				}
			}
			if got := string(params); got != test.want {
				t.Errorf("Decode params: got %#q, want %#q", got, test.want)
			}
		})
	}
}

func TestMetadata(t *testing.T) {
	type value struct {
		Name    string `json:"name,omitempty"`
		Marbles int    `json:"marbles,omitempty"`
	}
	input := value{Name: "Hieronymus Bosch", Marbles: 3}

	base := context.Background()
	ctx, err := WithMetadata(base, input)
	if err != nil {
		t.Fatalf("WithMetadata(base, %+v) failed: %v", input, err)
	}

	var output value

	// The base value does not contain the value.
	if err := UnmarshalMetadata(base, &output); err != ErrNoMetadata {
		t.Logf("Base metadata decoded value: %+v", output)
		t.Errorf("UnmarshalMetadata(base): got error %v, want %v", err, ErrNoMetadata)
	}

	// The attached context does contain the value (prior to transmission).
	output = value{}
	if err := UnmarshalMetadata(ctx, &output); err != nil {
		t.Errorf("UnmarshalMetadata(ctx): unexpected error: %v", err)
	} else if output != input {
		t.Errorf("UnmarshalMetadata(ctx): got %+v, want %+v", output, input)
	}

	// Simulate transmission -- encode, then decode.
	var dec context.Context
	if enc, err := Encode(ctx, "dummy", nil); err != nil {
		t.Fatalf("Encoding context failed: %v", err)
	} else {
		t.Logf("Encoded context is: %#q", string(enc))
		if dec, _, err = Decode(base, "dummy", enc); err != nil {
			t.Fatalf("Decoding context failed: %v", err)
		}
	}

	// The decoded context does contain the value (after receipt).
	output = value{}
	if err := UnmarshalMetadata(dec, &output); err != nil {
		t.Errorf("Metadata(dec): unexpected error: %v", err)
	} else if output != input {
		t.Errorf("Metadata(dec): got %+v, want %+v", output, input)
	}

	// "Attaching" nil removes the metadata.
	clr, err := WithMetadata(ctx, nil)
	if err != nil {
		t.Fatalf("WithMetadata(ctx, nil): unexpected error: %v", err)
	}
	var bad interface{}
	if err := UnmarshalMetadata(clr, &bad); err != ErrNoMetadata {
		t.Errorf("Metadata(clr): got %+v, %v; want %v", bad, err, ErrNoMetadata)
	}
}
