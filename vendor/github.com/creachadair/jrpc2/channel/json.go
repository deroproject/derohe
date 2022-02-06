// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package channel

import (
	"encoding/json"
	"io"
)

const bufSize = 4096

// RawJSON is a framing that transmits and receives records on r and wc, in which
// each record is defined by being a complete JSON value. No padding or other
// separation is added.
//
// A RawJSON channel has no out-of-band framing, so the channel cannot usually
// recover after a message that is not syntactically valid JSON.  Applications
// that need a channel to survive invalid JSON should avoid this framing.
func RawJSON(r io.Reader, wc io.WriteCloser) Channel {
	return jsonc{wc: wc, dec: json.NewDecoder(r), buf: make([]byte, bufSize)}
}

// A jsonc implements channel.Channel. Messages sent on a raw channel are not
// explicitly framed, and messages received are framed by JSON syntax.
type jsonc struct {
	wc  io.WriteCloser
	dec *json.Decoder
	buf json.RawMessage
}

// Send implements part of the Channel interface.
func (c jsonc) Send(msg []byte) error {
	if len(msg) == 0 || isNull(msg) {
		_, err := io.WriteString(c.wc, "null\n")
		return err
	}
	_, err := c.wc.Write(msg)
	return err
}

// Recv implements part of the Channel interface. It reports an error if the
// message is not a structurally valid JSON value. It is safe for the caller to
// treat any record returned as a json.RawMessage.
func (c jsonc) Recv() ([]byte, error) {
	c.buf = c.buf[:0] // reset
	if err := c.dec.Decode(&c.buf); err != nil {
		return nil, err
	} else if isNull(c.buf) {
		return nil, nil
	}
	return c.buf, nil
}

// Close implements part of the Channel interface.
func (c jsonc) Close() error { return c.wc.Close() }

func isNull(msg json.RawMessage) bool {
	return len(msg) == 4 && msg[0] == 'n' && msg[1] == 'u' && msg[2] == 'l' && msg[3] == 'l'
}
