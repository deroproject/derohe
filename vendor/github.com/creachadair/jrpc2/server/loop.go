// Package server provides support routines for running jrpc2 servers.
package server

import (
	"net"
	"sync"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
)

// Service is the interface used by the Loop and Run functions to start up a
// server. The methods of this interface allow the instance to manage its
// state: The Assigner method is called before the server is started, and can
// be used to initialize the service. The Finish method is called after the
// server exits, and can be used to clean up.
type Service interface {
	// This method is called to create an assigner and initialize the service
	// for use.  If it reports an error, the server is not started.
	Assigner() (jrpc2.Assigner, error)

	// This method is called when the server for this service has exited.
	// The arguments are the assigner returned by the Assigner method and the
	// server exit status.
	Finish(jrpc2.Assigner, jrpc2.ServerStatus)
}

// Static wraps a jrpc2.Assigner to trivially implement the Service interface.
func Static(m jrpc2.Assigner) func() Service { return static{methods: m}.New }

type static struct{ methods jrpc2.Assigner }

func (s static) New() Service                            { return s }
func (s static) Assigner() (jrpc2.Assigner, error)       { return s.methods, nil }
func (static) Finish(jrpc2.Assigner, jrpc2.ServerStatus) {}

// An Accepter obtains client connections from an external source and
// constructs channels from them.
type Accepter interface {
	// Accept accepts a connection and returns a new channel for it.
	Accept() (channel.Channel, error)
}

// NetAccepter adapts a net.Listener to the Accepter interface, using f as the
// channel framing.
func NetAccepter(lst net.Listener, f channel.Framing) Accepter {
	return netAccepter{Listener: lst, newChannel: f}
}

type netAccepter struct {
	net.Listener
	newChannel channel.Framing
}

func (n netAccepter) Accept() (channel.Channel, error) {
	conn, err := n.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return n.newChannel(conn, conn), nil
}

// Loop obtains connections from lst and starts a server for each using a
// service instance returned by newService and the given options. Each server
// runs in a new goroutine.
//
// If the listener reports an error, the loop will terminate and that error
// will be reported to the caller of Loop once any active servers have
// returned.
func Loop(lst Accepter, newService func() Service, opts *LoopOptions) error {
	serverOpts := opts.serverOpts()
	log := func(string, ...interface{}) {}
	if serverOpts != nil && serverOpts.Logger != nil {
		log = serverOpts.Logger.Printf
	}

	var wg sync.WaitGroup
	for {
		ch, err := lst.Accept()
		if err != nil {
			if channel.IsErrClosing(err) {
				err = nil
			} else {
				log("Error accepting new connection: %v", err)
			}
			wg.Wait()
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc := newService()
			assigner, err := svc.Assigner()
			if err != nil {
				log("Service initialization failed: %v", err)
				return
			}
			srv := jrpc2.NewServer(assigner, serverOpts).Start(ch)
			stat := srv.WaitStatus()
			svc.Finish(assigner, stat)
			if stat.Err != nil {
				log("Server exit: %v", stat.Err)
			}
		}()
	}
}

// LoopOptions control the behaviour of the Loop function.  A nil *LoopOptions
// provides default values as described.
type LoopOptions struct {
	// If non-nil, these options are used when constructing the server to
	// handle requests on an inbound connection.
	ServerOptions *jrpc2.ServerOptions
}

func (o *LoopOptions) serverOpts() *jrpc2.ServerOptions {
	if o == nil {
		return nil
	}
	return o.ServerOptions
}
