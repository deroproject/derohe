package channel

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// newPipe creates a pair of connected in-memory channels using the specified
// framing discipline. Sends to client will be received by server, and vice
// versa. newPipe will panic if framing == nil.
func newPipe(framing Framing) (client, server Channel) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	client = framing(cr, cw)
	server = framing(sr, sw)
	return
}

func testSendRecv(t *testing.T, s Sender, r Receiver, msg string) {
	var wg sync.WaitGroup
	var sendErr, recvErr error
	var data []byte

	wg.Add(2)
	go func() {
		defer wg.Done()
		data, recvErr = r.Recv()
	}()
	go func() {
		defer wg.Done()
		sendErr = s.Send([]byte(msg))
	}()
	wg.Wait()

	if sendErr != nil {
		t.Errorf("Send(%q): unexpected error: %v", msg, sendErr)
	}
	if recvErr != nil {
		t.Errorf("Recv(): unexpected error: %v", recvErr)
	}
	if got := string(data); got != msg {
		t.Errorf("Recv():\ngot  %#q\nwant %#q", got, msg)
	}
}

const message1 = `["Full plate and packing steel"]`
const message2 = `{"slogan":"Jump on your sword, evil!"}`

func TestDirect(t *testing.T) {
	lhs, rhs := Direct()
	defer lhs.Close()
	defer rhs.Close()

	t.Logf("Testing lhs ⇒ rhs :: %s", message1)
	testSendRecv(t, lhs, rhs, message1)
	t.Logf("Testing rhs ⇒ lhs :: %s", message2)
	testSendRecv(t, rhs, lhs, message2)
}

func TestDirectClosed(t *testing.T) {
	lhs, rhs := Direct()
	defer rhs.Close()
	lhs.Close() // immediately

	if err := lhs.Send([]byte("nonsense")); err == nil {
		t.Error("Send on closed channel did not fail")
	} else {
		t.Logf("Send correctly failed: %v", err)
	}
}

var tests = []struct {
	name    string
	framing Framing
}{
	{"Header", Header("binary/octet-stream")},
	{"LSP", LSP},
	{"Line", Line},
	{"NoMIME", Header("")},
	{"RS", Split('\x1e')},
	{"RawJSON", RawJSON},
	{"StrictHeader", StrictHeader("text/plain")},
	{"Varint", Varint},
}

// N.B. the messages in this list must be valid JSON, since the RawJSON framing
// requires that structure. A Channel is not required to check this generally.
var messages = []string{
	message1,
	message2,
	"null",
	"17",
	`"applejack"`,
	"[]",
	"{}",
	"[null]",

	// Include a long message to ensure size-dependent cases get exercised.
	`[` + strings.Repeat(`"ABCDefghIJKLmnopQRSTuvwxYZ!",`, 8000) + `"END"]`,
}

func clip(msg string) string {
	if len(msg) > 80 {
		return msg[:80] + fmt.Sprintf(" ...[%d bytes]", len(msg))
	}
	return msg
}

func TestChannelTypes(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lhs, rhs := newPipe(test.framing)
			defer lhs.Close()
			defer rhs.Close()

			for i, msg := range messages {
				n := strconv.Itoa(i + 1)
				t.Run("LR-"+n, func(t *testing.T) {
					t.Logf("Testing lhs → rhs :: %s", clip(msg))
					testSendRecv(t, lhs, rhs, message1)
				})
				t.Run("RL-"+n, func(t *testing.T) {
					t.Logf("Testing rhs → lhs :: %s", clip(msg))
					testSendRecv(t, rhs, lhs, message2)
				})
			}
		})
	}
}

func TestEmptyMessage(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lhs, rhs := newPipe(test.framing)
			defer lhs.Close()
			defer rhs.Close()

			t.Log(`Testing lhs → rhs :: "" (empty line)`)
			testSendRecv(t, lhs, rhs, "")
		})
		t.Run(test.name, func(t *testing.T) {
			lhs, rhs := Direct()
			defer lhs.Close()
			defer rhs.Close()

			t.Log(`Testing lhs → rhs :: "" (empty line)`)
			testSendRecv(t, lhs, rhs, "")
		})
	}
}

func TestHeaderTypeMismatch(t *testing.T) {
	cli, srv := newPipe(StrictHeader("text/plain"))
	defer cli.Close()
	defer srv.Close()

	noError := func(err error) bool { return err == nil }
	tests := []struct {
		payload string
		ok      func(error) bool
	}{
		// With a content type provided, no error is reported.
		// Order of headers and extra headers should not affect this.
		{"Content-Type: text/plain\r\nContent-Length: 3\r\n\r\nfoo", noError},
		{"Extra: ok\r\nContent-Length: 4\r\nContent-Type: text/plain\r\n\r\nquux", noError},

		// With a content type provided, report an error if it doesn't match.
		{"Content-Length: 2\r\nContent-Type: application/json\r\n\r\nno", func(err error) bool {
			v, ok := err.(*ContentTypeMismatchError)
			return ok && v.Got == "application/json" && v.Want == "text/plain"
		}},

		// With a content type omitted, a sentinel error is reported.
		{"Content-Length: 5\r\n\r\nabcde", func(err error) bool {
			v, ok := err.(*ContentTypeMismatchError)
			return ok && v.Got == "" && v.Want == "text/plain"
		}},

		// Other errors do not use this sentinel.
		{"Nothing: nohow\r\n\r\nfailure\n", func(err error) bool {
			_, isSentinel := err.(*ContentTypeMismatchError)
			return err != nil && !isSentinel
		}},
	}
	h := cli.(*hdr)
	for _, test := range tests {
		go func() {
			if _, err := h.wc.Write([]byte(test.payload)); err != nil {
				t.Errorf("Send %q failed: %v", test.payload, err)
			}
		}()
		msg, err := srv.Recv()
		if !test.ok(err) {
			t.Errorf("Recv failed: %v\n >> %q", err, msg)
		} else {
			t.Logf("Recv OK: %q", msg)
		}
	}
}

func TestWithTrigger(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, w := io.Pipe()
			triggered := false
			ch := WithTrigger(test.framing(r, w), func() {
				triggered = true
			})

			// Send a message to the channel, then close it.
			const message = `["fools", "rush", "in"]`
			done := make(chan struct{})
			go func() {
				defer close(done)
				t.Log("Sending...")
				if err := ch.Send([]byte(message)); err != nil {
					t.Errorf("Send failed: %v", err)
				}
				t.Logf("Close: err=%v", ch.Close())
			}()

			// Read messages from the channel till it closes, then check that
			// the trigger was correctly invoked.
			for {
				msg, err := ch.Recv()
				if err == io.EOF {
					t.Log("Recv: returned io.EOF")
					break
				} else if err != nil {
					t.Errorf("Recv: unexpected error: %v", err)
					break
				}
				t.Logf("Recv: msg=%q", string(msg))
			}

			<-done
			if !triggered {
				t.Error("After channel close: trigger not called")
			}
		})
	}
}
