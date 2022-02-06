// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

// Package channel defines a basic communications channel.
//
// A Channel encodes/transmits and decodes/receives data records over an
// unstructured stream, using a configurable framing discipline. This package
// provides some basic framing implementations.
//
// Channels
//
// A Channel represents the ability to send and received framed records,
// comprising the methods:
//
//     Send([]byte) error      // send a single complete record
//     Recv() ([]byte, error)  // receive a single complete record
//     Close() error           // close the channel
//
// Each record passed to Send is available for Recv. Record contents are not
// interpreted (except as noted below), and it is up to the implementation to
// decide how records are framed for transport.  A channel must support use by
// one sender and one receiver concurrently, but is not otherwise required to
// be safe for concurrent use.
//
// Framing
//
// A Framing function adapts a pair of io.Reader and io.WriteCloser to a
// Channel by imposing a particular message-framing discipline. This package
// provides several framing implementations, for example:
//
//    ch := channel.LSP(r, wc)
//
// creates a channel that reads from r and writes to wc using the Language
// Server Protocol (LSP) framing defined by
// https://microsoft.github.io/language-server-protocol/specification.
//
package channel

import (
	"errors"
	"io"
	"net"
)

// A Channel represents the ability to transmit and receive data records.  A
// channel does not interpret the contents of a record, but may add and remove
// framing so that records can be embedded in higher-level protocols.
//
// One sender and one receiver may use a Channel concurrently, but the methods
// of a Channel are not otherwise required to be safe for concurrent use.  The
// order of records received must be the same as the order sent.
type Channel interface {
	// Send transmits a record on the channel. Each call to Send transmits one
	// complete record.
	Send([]byte) error

	// Recv returns the next available record from the channel.  If no further
	// messages are available, it returns nil, io.EOF.  Each call to Recv
	// fetches a single complete record.
	Recv() ([]byte, error)

	// Close shuts down the channel, after which no further records may be
	// sent or received.
	Close() error
}

// ErrClosed is a sentinel error that can be returned to indicate an operation
// failed because the channel was closed.
var ErrClosed = errors.New("channel is closed")

// IsErrClosing reports whether err is a channel-closed error.  This is true
// for the internal error returned by a read from a pipe or socket that is
// closed, or an error that wraps ErrClosed. It is false if err == nil.
func IsErrClosing(err error) bool {
	return err != nil && (errors.Is(err, ErrClosed) || errors.Is(err, net.ErrClosed))
}

// A Framing converts a reader and a writer into a Channel with a particular
// message-framing discipline.
type Framing func(io.Reader, io.WriteCloser) Channel

type direct struct {
	send chan<- []byte
	recv <-chan []byte
}

func (d direct) Send(msg []byte) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errors.New("send on closed channel")
		}
	}()
	d.send <- msg
	return nil
}

func (d direct) Recv() ([]byte, error) {
	msg, ok := <-d.recv
	if ok {
		return msg, nil
	}
	return nil, io.EOF
}

func (d direct) Close() error { close(d.send); return nil }

// Direct returns a pair of synchronous connected channels that pass message
// buffers directly in memory without framing or encoding. Sends to client will
// be received by server, and vice versa.
//
// Note that buffers passed to direct channels are not copied. If the caller
// needs to use the buffer after sending it on a direct channel, the caller is
// responsible for making a copy.
func Direct() (client, server Channel) {
	c2s := make(chan []byte)
	s2c := make(chan []byte)
	client = direct{send: c2s, recv: s2c}
	server = direct{send: s2c, recv: c2s}
	return
}
