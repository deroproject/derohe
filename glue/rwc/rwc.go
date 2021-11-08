//go:build !wasm
// +build !wasm

package rwc

import (
	"github.com/gorilla/websocket"
	"io"
)

type ReadWriteCloser struct {
	WS *websocket.Conn
	r  io.Reader
	w  io.WriteCloser
}

func New(conn *websocket.Conn) *ReadWriteCloser {
	return &ReadWriteCloser{WS: conn}
}

func (rwc *ReadWriteCloser) Read(p []byte) (n int, err error) {
	if rwc.r == nil {
		_, rwc.r, err = rwc.WS.NextReader()
		if err != nil {
			return 0, err
		}
	}
	for n = 0; n < len(p); {
		var m int
		m, err = rwc.r.Read(p[n:])
		n += m
		if err == io.EOF {
			rwc.r = nil
		}
		if err != nil {
			break
		}
	}
	return
}

func (rwc *ReadWriteCloser) Write(p []byte) (n int, err error) {
	if rwc.w == nil {
		rwc.w, err = rwc.WS.NextWriter(websocket.TextMessage)
		if err != nil {
			return 0, err
		}
	}
	for n = 0; n < len(p); {
		var m int
		m, err = rwc.w.Write(p)
		n += m
		if err != nil {
			break
		}
	}
	if err != nil || n == len(p) {
		err = rwc.Close()
	}
	return
}

func (rwc *ReadWriteCloser) Close() (err error) {
	if rwc.w != nil {
		err = rwc.w.Close()
		rwc.w = nil
	}
	return err
}
