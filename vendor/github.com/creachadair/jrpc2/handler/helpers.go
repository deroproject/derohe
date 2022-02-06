// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Args is a wrapper that decodes an array of positional parameters into
// concrete locations.
//
// Unmarshaling a JSON value into an Args value v succeeds if the JSON encodes
// an array with length len(v), and unmarshaling each subvalue i into the
// corresponding v[i] succeeds.  As a special case, if v[i] == nil the
// corresponding value is discarded.
//
// Marshaling an Args value v into JSON succeeds if each element of the slice
// is JSON marshalable, and yields a JSON array of length len(v) containing the
// JSON values corresponding to the elements of v.
//
// Usage example:
//
//    func Handler(ctx context.Context, req *jrpc2.Request) (interface{}, error) {
//       var x, y int
//       var s string
//
//       if err := req.UnmarshalParams(&handler.Args{&x, &y, &s}); err != nil {
//          return nil, err
//       }
//       // do useful work with x, y, and s
//    }
//
type Args []interface{}

// UnmarshalJSON supports JSON unmarshaling for a.
func (a Args) UnmarshalJSON(data []byte) error {
	var elts []json.RawMessage
	if err := json.Unmarshal(data, &elts); err != nil {
		return filterJSONError("args", "array", err)
	} else if len(elts) != len(a) {
		return fmt.Errorf("wrong number of args (got %d, want %d)", len(elts), len(a))
	}
	for i, elt := range elts {
		if a[i] == nil {
			continue
		} else if err := json.Unmarshal(elt, a[i]); err != nil {
			return fmt.Errorf("decoding argument %d: %w", i+1, err)
		}
	}
	return nil
}

// MarshalJSON supports JSON marshaling for a.
func (a Args) MarshalJSON() ([]byte, error) {
	if len(a) == 0 {
		return []byte(`[]`), nil
	}
	return json.Marshal([]interface{}(a))
}

// Obj is a wrapper that maps object fields into concrete locations.
//
// Unmarshaling a JSON text into an Obj value v succeeds if the JSON encodes an
// object, and unmarshaling the value for each key k of the object into v[k]
// succeeds. If k does not exist in v, it is ignored.
//
// Marshaling an Obj into JSON works as for an ordinary map.
type Obj map[string]interface{}

// UnmarshalJSON supports JSON unmarshaling into o.
func (o Obj) UnmarshalJSON(data []byte) error {
	var base map[string]json.RawMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return filterJSONError("decoding", "object", err)
	}
	for key, arg := range o {
		val, ok := base[key]
		if !ok {
			continue
		} else if err := json.Unmarshal(val, arg); err != nil {
			return fmt.Errorf("decoding %q: %v", key, err)
		}
	}
	return nil
}

func filterJSONError(tag, want string, err error) error {
	if t, ok := err.(*json.UnmarshalTypeError); ok {
		return fmt.Errorf("%s: cannot decode %s as %s", tag, t.Value, want)
	}
	return err
}

// firstByte returns the first non-whitespace byte of data, or 0 if there is none.
func firstByte(data []byte) byte {
	clean := bytes.TrimSpace(data)
	if len(clean) == 0 {
		return 0
	}
	return clean[0]
}
