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

// A Channel implements a channel.Channel that dispatches requests via HTTP to
// a user-provided URL. Each message sent to the channel is an HTTP POST
// request with the message as its body.
type Channel struct {
	url string
	cli HTTPClient
	wg  *sync.WaitGroup
	rsp chan response
}

type response struct {
	rsp *http.Response
	err error
}

// HTTPClient is the interface to an HTTP client used by a Channel. It is
// compatible with the standard library http.Client type.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// ChannelOptions gives optional parameters for constructing an HTTP channel.
// A nil *ChannelOptions is ready for use, and provides default options as
// described.
type ChannelOptions struct {
	// The HTTP client to use to send requests. If nil, uses http.DefaultClient.
	Client HTTPClient
}

func (o *ChannelOptions) httpClient() HTTPClient {
	if o == nil || o.Client == nil {
		return http.DefaultClient
	}
	return o.Client
}

// NewChannel constructs a new channel that posts to the specified URL.
func NewChannel(url string, opts *ChannelOptions) *Channel {
	return &Channel{
		url: url,
		cli: opts.httpClient(),
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
	// HTTP server, and c.wg tracks these goroutines. Acknowledgement replies to
	// notifications are handled immediately; (non-empty) responses to calls are
	// delivered to the c.rsp channel. Error handling happens in Recv, since
	// that is where the caller is prepared to receive an error.
	//
	// The goroutine for a Send whose request returns a non-empty result cannot
	// exit until either a Recv occurs, or until the channel is closed (draining
	// any further undelivered responses).  The caller should thus avoid sending
	// too many call requests with no intervening Recv calls.
	req, err := http.NewRequest("POST", c.url, bytes.NewReader(msg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		rsp, err := cli.Do(req)

		// If the server replied with an empty acknowledgement for a
		// notification, exit early so that we don't depend on a Recv.
		if err == nil && rsp.StatusCode == http.StatusNoContent {
			rsp.Body.Close()
			return
		}
		c.rsp <- response{rsp, err}
	}()
	return nil
}

// Recv receives the next available response and reports its body.
func (c *Channel) Recv() ([]byte, error) {
	next, ok := <-c.rsp
	if !ok {
		return nil, io.EOF
	} else if next.err != nil {
		return nil, next.err // HTTP failure, not request failure
	}

	// Ensure the body is fully read and closed before continuing.
	data, err := ioutil.ReadAll(next.rsp.Body)
	next.rsp.Body.Close()

	// N.B. Empty responses (StatusNoContent) is handled by Send, so we will
	// not see those responses here.
	if next.rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %s", next.rsp.Status)
	}
	// ok, we have a message to report
	return data, err
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
