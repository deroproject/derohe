// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

/*
Package jrpc2 implements a server and a client for the JSON-RPC 2.0 protocol
defined by http://www.jsonrpc.org/specification.

Servers

The *Server type implements a JSON-RPC server. A server communicates with a
client over a channel.Channel, and dispatches client requests to user-defined
method handlers.  Handlers satisfy the jrpc2.Handler interface by exporting a
Handle method with this signature:

   Handle(ctx Context.Context, req *jrpc2.Request) (interface{}, error)

A server finds the handler for a request by looking up its method name in a
jrpc2.Assigner provided when the server is set up. A Handler can decode the
request parameters using the UnmarshalParams method on the request:

   func (H) Handle(ctx context.Context, req *jrpc2.Request) (interface{}, error) {
      var args ArgType
      if err := req.UnmarshalParams(&args); err != nil {
         return nil, err
      }
      return usefulStuffWith(args)
   }

The handler package makes it easier to use functions that do not have this
exact type signature as handlers, by using reflection to lift functions into
the Handler interface.  For example, suppose we want to export this Add function:

   // Add returns the sum of a slice of integers.
   func Add(ctx context.Context, values []int) int {
      sum := 0
      for _, v := range values {
         sum += v
      }
      return sum
   }

To convert Add to a jrpc2.Handler, call handler.New, which wraps its argument
into the jrpc2.Handler interface via the handler.Func type:

   h := handler.New(Add)  // h is now a jrpc2.Handler that calls Add

The handler package also provides handler.Map, which implements the Assigner
interface with a Go map. To advertise this function under the name "Add":

   assigner := handler.Map{
      "Add": handler.New(Add),
   }

Equipped with an Assigner we can now construct a Server:

   srv := jrpc2.NewServer(assigner, nil)  // nil for default options

To start the server, we need a channel.Channel. Implementations of the Channel
interface handle the framing, transmission, and receipt of JSON messages.  The
channel package implements some common framing disciplines for byte streams
like pipes and sockets.  For this example, we'll use a channel that
communicates over stdin and stdout, with messages delimited by newlines:

   ch := channel.Line(os.Stdin, os.Stdout)

Now we can start the server:

   srv.Start(ch)

The Start method does not block.  The server runs until the channel closes, or
until it is stopped explicitly by calling srv.Stop(). To wait for the server to
finish, call:

   err := srv.Wait()

This will report the error that led to the server exiting.  The code for this
example is available from tools/examples/adder/adder.go:

    $ go run tools/examples/adder/adder.go

Interact with the server by sending JSON-RPC requests on stdin, such as for
example:

   {"jsonrpc":"2.0", "id":1, "method":"Add", "params":[1, 3, 5, 7]}


Clients

The *Client type implements a JSON-RPC client. A client communicates with a
server over a channel.Channel, and is safe for concurrent use by multiple
goroutines. It supports batched requests and may have arbitrarily many pending
requests in flight simultaneously.

To create a client we need a channel:

   import "net"

   conn, err := net.Dial("tcp", "localhost:8080")
   ...
   ch := channel.Line(conn, conn)
   cli := jrpc2.NewClient(ch, nil)  // nil for default options

To send a single RPC, use the Call method:

   rsp, err := cli.Call(ctx, "Math.Add", []int{1, 3, 5, 7})

Call blocks until the response is received. Errors returned by the server have
concrete type *jrpc2.Error.

To issue a batch of concurrent requests, use the Batch method:

   rsps, err := cli.Batch(ctx, []jrpc2.Spec{
      {Method: "Math.Add", Params: []int{1, 2, 3}},
      {Method: "Math.Mul", Params: []int{4, 5, 6}},
      {Method: "Math.Max", Params: []int{-1, 5, 3, 0, 1}},
   })

Batch blocks until all the responses are received.  An error from the Batch
call reflects an error in sending the request: The caller must check each
response separately for errors from the server. Responses are returned in the
same order as the Spec values, save that notifications are omitted.

To decode the result from a successful response, use its UnmarshalResult method:

   var result int
   if err := rsp.UnmarshalResult(&result); err != nil {
      log.Fatalln("UnmarshalResult:", err)
   }

To close a client and discard all its pending work, call cli.Close().


Notifications

A JSON-RPC notification is a one-way request: The client sends the request to
the server, but the server does not reply. Use the Notify method of a client to
send a notification:

   note := handler.Obj{"message": "A fire is burning!"}
   err := cli.Notify(ctx, "Alert", note)

A notification is complete once it has been sent. Notifications can also be sent
as part of a batch request:

   rsps, err := cli.Batch(ctx, []jrpc2.Spec{
      {Method: "Alert", Params: note, Notify: true},  // this is a notification
      {Method: "Math.Add": Params: []int{1, 2}},      // this is a normal call
      // ...
   })

On the server, notifications are handled just like other requests, except that
the return value is discarded once the handler returns. If a handler does not
want to do anything for a notification, it can query the request:

   if req.IsNotification() {
      return 0, nil  // ignore notifications
   }


Services with Multiple Methods

The example above shows a server with one method.  A handler.Map works for any
number of distinctly-named methods:

   mathService := handler.Map{
      "Add": handler.New(Add),
      "Mul": handler.New(Mul),
   }

Maps may be further combined with the handler.ServiceMap type to allow multiple
services to be exported from the same server:

   func getStatus(context.Context) string { return "all is well" }

   assigner := handler.ServiceMap{
      "Math":   mathService,
      "Status": handler.Map{
        "Get": handler.New(Status),
      },
   }

This assigner dispatches "Math.Add" and "Math.Mul" to the arithmetic functions,
and "Status.Get" to the getStatus function. A ServiceMap splits the method name
on the first period ("."), and you may nest ServiceMaps more deeply if you
require a more complex hierarchy.


Concurrency

A Server issues concurrent requests to handlers in parallel, up to the limit
given by the Concurrency field in ServerOptions.

Two requests (either calls or notifications) are concurrent if they arrive as
part of the same batch. In addition, two calls are concurrent if the time
intervals between the arrival of the request objects and delivery of the
response objects overlap.

The server may issue concurrent requests to their handlers in any order.
Non-concurrent requests are processed in order of arrival. Notifications, in
particular, can only be concurrent with other requests in the same batch.  This
ensures a client that sends a notification can be sure its notification will be
fully processed before any subsequent calls are issued to their handlers.

These rules imply that the client cannot rely on the execution order of calls
that overlap in time: If the caller needs to ensure that call A completes
before call B starts, it must wait for A to return before invoking B.


Built-in Methods

Per the JSON-RPC 2.0 spec, method names beginning with "rpc." are reserved by
the implementation. By default, a server does not dispatch these methods to its
assigner. In this configuration, the server exports a "rpc.serverInfo" method
taking no parameters and returning a jrpc2.ServerInfo value.

Setting the DisableBuiltin server option to true removes special treatment of
"rpc." method names, and disables the rpc.serverInfo handler.  When this option
is true, method names beginning with "rpc." will be dispatched to the assigner
like any other method.


Server Push

The AllowPush server option allows handlers to "push" requests back to the
client.  This is a non-standard extension of JSON-RPC used by some applications
such as the Language Server Protocol (LSP). When this option is enabled, the
server's Notify and Callback methods send requests back to the client.
Otherwise, those methods will report an error:

  if err := s.Notify(ctx, "methodName", params); err == jrpc2.ErrPushUnsupported {
    // server push is not enabled
  }
  if rsp, err := s.Callback(ctx, "methodName", params); err == jrpc2.ErrPushUnsupported {
    // server push is not enabled
  }

A method handler may use jrpc2.ServerFromContext to access the server from its
context, and then invoke these methods on it.  On the client side, the OnNotify
and OnCallback options in jrpc2.ClientOptions provide hooks to which any server
requests are delivered, if they are set.

Since not all clients support server push, handlers should set a timeout when
using the server Callback method; otherwise the callback may block forever for
a client response that will never arrive.
*/
package jrpc2

// Version is the version string for the JSON-RPC protocol understood by this
// implementation, defined at http://www.jsonrpc.org/specification.
const Version = "2.0"
