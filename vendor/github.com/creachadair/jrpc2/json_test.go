//go:build oldbench
// +build oldbench

package jrpc2

import (
	"encoding/json"
	"strconv"
	"testing"
)

const payloadText = `{
  "alpha": [1, 2, 3, 4, 5],
  "bravo": true,
  "charlie": {
    "foo": ["xyz", "pdq"],
    "bar": false,
    "baz": 1.00003e+19
  },
  "delta": null,
  "echo": 3.352391934e-19,
  "foxtrot": "all your \"base\" are belong to us"
}`

const errMessage = "you did not do the correct thing and you should feel bad"

const methodName = "some long method name whatever"

func BenchmarkEncodeMessage(b *testing.B) {
	msgs := jmessages{
		{ID: []byte("12345"), M: methodName, R: []byte(payloadText)},
		{ID: nil, M: methodName, R: []byte(payloadText)},
		{ID: []byte("12345"), R: []byte(payloadText)},
		{ID: []byte("12345"), E: &Error{Code: 12345, Message: errMessage}},
	}
	for i, msg := range msgs {
		b.Run(strconv.Itoa(i+1)+"-std", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				json.Marshal(msg)
			}
		})
		b.Run(strconv.Itoa(i+1)+"-custom", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				msg.toJSON()
			}
		})
	}

	b.Run("batch-std", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			json.Marshal(msgs)
		}
	})
	b.Run("batch-custom", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			msgs.toJSON()
		}
	})
}
