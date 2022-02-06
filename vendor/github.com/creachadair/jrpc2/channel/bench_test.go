// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package channel_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/creachadair/jrpc2/channel"
)

var benchMessage = map[string]interface{}{
	"jsonrpc": "2.0",
	"id":      "<XXX>",
	"method":  "echo",
	"params": map[string]interface{}{
		"title":     "A Cask of Amontillado",
		"author":    "Edgar Allan Poe",
		"published": 1847,
		"text": strings.Fields(`

      The thousand injuries of Fortunato I had borne as I best could but when
      he ventured upon insult I vowed revenge you who so well know the nature
      of my soul will not suppose however that gave utterance to a threat at
      length I would be avenged this was a point definitely settled but the
      very definitiveness with which it was resolved precluded the idea of risk
      I must not only punish but punish with impunity a wrong is unredressed
      when retribution overtakes its redresser it is equally unredressed when
      the avenger fails to make himself felt as such to him who has done the
      wrong`),
	},
}

func BenchmarkFramingCost(b *testing.B) {
	framings := []struct {
		name    string
		framing channel.Framing
	}{
		{"Line", channel.Line},
		{"LSP", channel.LSP},
		{"NUL", channel.Split('\x00')},
		{"RawJSON", channel.RawJSON},
	}

	msg, err := json.Marshal(benchMessage)
	if err != nil {
		b.Fatalf("Marshaling benchmark message: %v", err)
	}
	b.Logf("Benchmark baseline message is %d byte", len(msg))

	// Benchmark the round-trip call cycle for a method that does no useful
	// work, as a proxy for overhead for client and server maintenance, and the
	// cost of encoding and decoding request messages.
	for _, bench := range framings {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()

			// Simulate a bidirectional channel with unbuffered in-memory pipes.
			cr, sw := io.Pipe()
			sr, cw := io.Pipe()
			cli := bench.framing(cr, cw)
			srv := bench.framing(sr, sw)
			pkt := bytes.Replace(msg, []byte("<XXX>"), []byte(strconv.Itoa(b.N)), 1)

			var wg sync.WaitGroup
			wg.Add(2)
			b.ResetTimer()

			// The "client" sends a message to the server and waits for it to be
			// returned, then checks that the result matches.
			go func() {
				defer wg.Done()
				for i := 0; i < b.N; i++ {
					if err := cli.Send(pkt); err != nil {
						b.Errorf("cli.Send(%d bytes) failed: %v", len(pkt), err)
						continue
					}
					cmp, err := cli.Recv()
					if err != nil {
						b.Errorf("cli.Recv() failed: %v", err)
						continue
					}
					if !bytes.Equal(cmp, pkt) {
						b.Errorf("Recv does not match Send\nsend: %s\nrecv: %s", string(pkt), string(cmp))
						continue
					}
				}
			}()

			// The "server" receives a messaage from the client and sends it back.
			go func() {
				defer wg.Done()
				for i := 0; i < b.N; i++ {
					pkt, err := srv.Recv()
					if err != nil {
						b.Errorf("srv.Recv() failed: %v", err)
						continue
					}
					if err := srv.Send(pkt); err != nil {
						b.Errorf("srv.Send(%d bytes) failed: %v", len(pkt), err)
						continue
					}
				}
			}()
			wg.Wait()
		})
	}
}
