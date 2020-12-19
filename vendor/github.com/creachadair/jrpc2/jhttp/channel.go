package jhttp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
)

// A Channel implements an channel.Channel that dispatches requests via HTTP to
// a user-provided URL. Each message sent to the channel is an HTTP POST
// request with the message as its body.
type Channel struct {
	url string
	cli *http.Client
	wg  *sync.WaitGroup
	rsp chan response
}

type response struct {
	rsp *http.Response
	err error
}

// NewChannel constructs a new channel that posts to the specified URL.
func NewChannel(url string) *Channel {
	return &Channel{
		url: url,
		cli: http.DefaultClient,
		wg:  new(sync.WaitGroup),
		rsp: make(chan response),
	}
}

// Send forwards msg to the server as the body of an HTTP POST request.
func (c *Channel) Send(msg []byte) error {
	cli := c.cli
	if cli == nil {
		return errors.New("channel is closed")
	}

	// Each Send starts a goroutine to wait for the corresponding reply from the
	// HTTP server, and c.wg tracks these goroutines. Responses are delivered to
	// the c.rsp channel without interpretation; error handling happens in Recv.
	//
	// The goroutine for a Send cannot exit until either a Recv occurs, or until
	// the channel is closed (draining any further undelivered responses).  The
	// caller should thus avoid calling Send a large number of times with no
	// intervening Recv calls.
	req, err := http.NewRequest("POST", c.url, bytes.NewReader(msg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		rsp, err := cli.Do(req)
		c.rsp <- response{rsp, err}
	}()
	return nil
}

// Recv receives the next available response and reports its body.
func (c *Channel) Recv() ([]byte, error) {
	for {
		next, ok := <-c.rsp
		if !ok {
			return nil, io.EOF
		} else if next.err != nil {
			return nil, next.err // HTTP failure, not request failure
		}

		// Ensure the body is fully read and closed before continuing.
		data, err := ioutil.ReadAll(next.rsp.Body)
		next.rsp.Body.Close()

		switch next.rsp.StatusCode {
		case http.StatusOK:
			// ok, we have a message to report
			return data, err
		case http.StatusNoContent:
			// ok, but no message to report; wait for another
			continue
		default:
			return nil, fmt.Errorf("unexpected HTTP status %s", next.rsp.Status)
		}
	}
}

// Close shuts down the channel, discarding any pending responses.
func (c *Channel) Close() error {
	c.cli = nil // no further requests may be sent

	// Drain any pending requests.
	go func() { c.wg.Wait(); close(c.rsp) }()
	for range c.rsp {
		// discard
	}
	return nil
}
