// Program jcall issues RPC calls to a JSON-RPC server.
//
// Usage:
//    jcall [options] <address> {<method> <params>}...
//
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/creachadair/wschannel"
)

var (
	dialTimeout = flag.Duration("dial", 5*time.Second, "Timeout on dialing the server (0 for no timeout)")
	callTimeout = flag.Duration("timeout", 0, "Timeout on each call (0 for no timeout)")
	doNotify    = flag.Bool("notify", false, "Send a notification")
	withContext = flag.Bool("c", false, "Send context with request")
	chanFraming = flag.String("f", envOrDefault("JCALL_FRAMING", "line"), "Channel framing")
	doBatch     = flag.Bool("batch", false, "Issue calls as a batch rather than sequentially")
	doErrors    = flag.Bool("e", false, "Print error values to stdout")
	doIndent    = flag.Bool("i", false, "Indent JSON output")
	doMulti     = flag.Bool("m", false, "Issue the same call repeatedly with different arguments")
	doTiming    = flag.Bool("T", false, "Print call timing stats")
	doWaitExit  = flag.Bool("W", false, "Wait for interrupt at exit")
	withLogging = flag.Bool("v", false, "Enable verbose logging")
	withMeta    = flag.String("meta", "", "Attach this JSON value as request metadata (implies -c)")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] <address> {<method> <params>}...
       %[1]s [options] -m <address> <method> <params>...

Connect to the specified address and transmit the specified JSON-RPC method
calls in sequence (or as a batch, if -batch is set).  The resulting response
values are printed to stdout.

Supported address formats include:

  host:port              -- TCP connection to the given host and port
  some/path              -- Unix-domain socket at the given path
  http://host:port/path  -- HTTP connection to the given URL
  ws://host:port/path    -- Websocket connection to the given URL

Without -m, each pair of arguments names a method and its parameters to call.
With -m, the first argument names a method to be repeatedly called with each of
the remaining arguments as its parameter.

The -f flag sets the framing discipline to use. The client must agree with the
server in order for communication to work. The options are:

  header:<t> -- header-framed, content-type <t>
  strict:<t> -- strict header-framed, content-type <t>
  line       -- byte-terminated, records end in LF (Unicode 10)
  lsp        -- header-framed, content-type application/vscode-jsonrpc (like LSP)
  raw        -- unframed, each message is a complete JSON value

See also: https://godoc.org/github.com/creachadair/jrpc2/channel.
The default framing is read from the JCALL_FRAMING environment variable, if set.
The -f flag overrides the environment.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// There must be at least one request, and more are permitted.  Each method
	// must have an argument, though it may be empty.
	if *doMulti {
		if flag.NArg() < 3 {
			log.Fatal("Arguments are <address> <method> <params>...")
		}
	} else if flag.NArg() < 3 || flag.NArg()%2 == 0 {
		log.Fatal("Arguments are <address> {<method> <params>}...")
	}

	// Set up the context for the call, including timeouts and any metadata that
	// are specified on the command line. Setting -meta also implicitly sets -c.
	ctx := context.Background()
	if *withMeta == "" {
		*withMeta = os.Getenv("JCALL_META")
	}
	if *withMeta != "" {
		mc, err := jctx.WithMetadata(ctx, json.RawMessage(*withMeta))
		if err != nil {
			log.Fatalf("Invalid request metadata: %v", err)
		}
		ctx = mc
		*withContext = true
	}

	if *callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *callTimeout)
		defer cancel()
	}

	// Establish a client channel. If we are using HTTP we do not need to dial a
	// connection; the HTTP client will handle that.
	start := time.Now()
	var cc channel.Channel
	if isHTTP(flag.Arg(0)) {
		cc = jhttp.NewChannel(flag.Arg(0), nil)
	} else if isWebsocket(flag.Arg(0)) {
		ch, err := wschannel.Dial(flag.Arg(0), nil)
		if err != nil {
			log.Fatalf("Dial %q: %v", flag.Arg(0), err)
		}
		defer ch.Close()
		cc = ch
	} else if nc := newFraming(*chanFraming); nc == nil {
		log.Fatalf("Unknown channel framing %q", *chanFraming)
	} else {
		ntype, _ := jrpc2.Network(flag.Arg(0))
		conn, err := net.DialTimeout(ntype, flag.Arg(0), *dialTimeout)
		if err != nil {
			log.Fatalf("Dial %q: %v", flag.Arg(0), err)
		}
		defer conn.Close()
		cc = nc(conn, conn)
	}
	tdial := time.Now()

	done := make(chan os.Signal, 1)
	if *doWaitExit {
		signal.Notify(done, os.Interrupt)
	} else {
		close(done)
	}
	cli := newClient(cc)
	pdur, err := issueCalls(ctx, cli, flag.Args()[1:])
	// defer failure on error till after we print aggregate timing stats
	tcall := time.Now()
	if e, ok := err.(*jrpc2.Error); ok && *doErrors {
		etxt, _ := json.Marshal(e)
		fmt.Println(string(etxt))
	} else if err != nil {
		log.Printf("Call failed: %v", err)
	}
	cdur := tcall.Sub(tdial) - pdur
	tprintf("%v elapsed: %v dial, %v call, %v print [%s]",
		tcall.Sub(start), tdial.Sub(start), cdur, pdur, callStatus(err))
	if err != nil {
		os.Exit(1)
	}
	if *doWaitExit {
		log.Print("<waiting at exit>")
	}
	<-done
}

func newClient(conn channel.Channel) *jrpc2.Client {
	opts := &jrpc2.ClientOptions{
		OnNotify: func(req *jrpc2.Request) {
			var p json.RawMessage
			req.UnmarshalParams(&p)
			fmt.Printf(`{"method":%q,"params":%s}`+"\n", req.Method(), string(p))
		},
	}
	if *withContext {
		opts.EncodeContext = jctx.Encode
	}
	if *withLogging {
		opts.Logger = jrpc2.StdLogger(nil)
	}
	return jrpc2.NewClient(conn, opts)
}

func printResults(rsps []*jrpc2.Response) (time.Duration, error) {
	var err error
	set := func(e error) {
		if err == nil {
			err = e
		}
	}
	var dur time.Duration
	for i, rsp := range rsps {
		if rerr := rsp.Error(); rerr != nil {
			if *doErrors {
				etxt, _ := json.Marshal(rerr)
				fmt.Println(formatJSON(etxt))
			} else {
				log.Printf("Error (%d): %v", i+1, rerr)
			}
			set(errors.New("batch contained errors"))
			continue
		}
		pstart := time.Now()
		var result json.RawMessage
		if perr := rsp.UnmarshalResult(&result); perr != nil {
			log.Printf("Decoding (%d): %v", i+1, perr)
			set(perr)
			continue
		}
		fmt.Println(formatJSON(result))
		dur += time.Since(pstart)
	}
	return dur, err
}

func issueCalls(ctx context.Context, cli *jrpc2.Client, args []string) (time.Duration, error) {
	specs := newSpecs(args)
	if *doBatch {
		rsps, err := cli.Batch(ctx, specs)
		if err != nil {
			return 0, err
		}
		return printResults(rsps)
	}
	return issueSequential(ctx, cli, specs)
}

func tprintf(msg string, args ...interface{}) {
	if !*doTiming {
		return
	}
	fmt.Fprintf(os.Stderr, msg, args...)
	if !strings.HasSuffix(msg, "\n") {
		fmt.Fprintln(os.Stderr)
	}
}

func issueSequential(ctx context.Context, cli *jrpc2.Client, specs []jrpc2.Spec) (time.Duration, error) {
	var dur time.Duration
	for _, spec := range specs {
		cstart := time.Now()
		if spec.Notify {
			err := cli.Notify(ctx, spec.Method, spec.Params)
			tprintf("[notify %s]: %v call [%s]", spec.Method, time.Since(cstart), callStatus(err))
			if err != nil {
				return dur, err
			}
			continue
		}
		rsp, err := cli.Call(ctx, spec.Method, spec.Params)
		if err != nil {
			return dur, err
		}
		cdur := time.Since(cstart)
		pstart := time.Now()
		var result json.RawMessage
		if perr := rsp.UnmarshalResult(&result); perr != nil {
			return dur, err
		}
		fmt.Println(formatJSON(result))
		pdur := time.Since(pstart)
		dur += pdur
		tprintf("[call %s]: %v call, %v print [%s]\n", spec.Method, cdur, pdur, callStatus(err))
	}
	return dur, nil
}

func newSpecs(args []string) []jrpc2.Spec {
	if *doMulti {
		specs := make([]jrpc2.Spec, 0, len(args)-1)
		method := args[0]
		for _, arg := range args[1:] {
			specs = append(specs, jrpc2.Spec{
				Method: method,
				Params: param(arg),
				Notify: *doNotify,
			})
		}
		return specs
	}
	specs := make([]jrpc2.Spec, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		specs = append(specs, jrpc2.Spec{
			Method: args[i],
			Params: param(args[i+1]),
			Notify: *doNotify,
		})
	}
	return specs
}

func param(s string) interface{} {
	if s == "" {
		return nil
	}
	return json.RawMessage(s)
}

func formatJSON(data []byte) string {
	if *doIndent {
		var buf bytes.Buffer
		json.Indent(&buf, data, "", "  ")
		return buf.String()
	}
	return string(data)
}

func isHTTP(addr string) bool {
	return strings.HasPrefix(addr, "http:") || strings.HasPrefix(addr, "https:")
}

func isWebsocket(addr string) bool {
	return strings.HasPrefix(addr, "ws:") || strings.HasPrefix(addr, "wss:")
}

func callStatus(err error) string {
	switch err.(type) {
	case nil:
		return "OK"
	case *jrpc2.Error:
		return "server error"
	default:
		return "failed"
	}
}

func envOrDefault(env, dflt string) string {
	if s, ok := os.LookupEnv(env); ok {
		return s
	}
	return dflt
}

// newFraming returns a channel.Framing described by the specified name, or nil
// if the name is unknown. The framing types currently understood are:
//
//    header:t -- corresponds to channel.Header(t)
//    strict:t -- corresponds to channel.StrictHeader(t)
//    line     -- corresponds to channel.Line
//    lsp      -- corresponds to channel.LSP
//    raw      -- corresponds to channel.RawJSON
//
func newFraming(name string) channel.Framing {
	if t := strings.TrimPrefix(name, "header:"); t != name {
		return channel.Header(t)
	}
	if t := strings.TrimPrefix(name, "strict:"); t != name {
		return channel.StrictHeader(t)
	}
	switch name {
	case "line":
		return channel.Line
	case "lsp":
		return channel.LSP
	case "raw":
		return channel.RawJSON
	}
	return nil
}
