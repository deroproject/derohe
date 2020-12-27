package rwc

import (
	"context"
	"io"
	"nhooyr.io/websocket"
)

type ReadWriteCloserNhooyr struct {
	WS *websocket.Conn
	r  io.Reader
	w  io.WriteCloser
}

func NewNhooyr(conn *websocket.Conn) *ReadWriteCloserNhooyr {
	return &ReadWriteCloserNhooyr{WS: conn}
}

func (rwc *ReadWriteCloserNhooyr) Read(p []byte) (n int, err error) {
	if rwc.r == nil {
		rwc.WS.SetReadLimit(2 * 1024 * 1024)
		_, rwc.r, err = rwc.WS.Reader(context.Background())
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

func (rwc *ReadWriteCloserNhooyr) Write(p []byte) (n int, err error) {
	if rwc.w == nil {
		rwc.w, err = rwc.WS.Writer(context.Background(), websocket.MessageText)
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

func (rwc *ReadWriteCloserNhooyr) Close() (err error) {
	if rwc.w != nil {
		err = rwc.w.Close()
		rwc.w = nil
	}
	return err
}
