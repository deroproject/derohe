// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package server_test

import (
	"context"
	"fmt"
	"log"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/server"
)

func ExampleNewLocal() {
	loc := server.NewLocal(handler.Map{
		"Hello": handler.New(func(context.Context) (string, error) {
			return "Hello, world!", nil
		}),
	}, nil)
	defer loc.Close()

	var result string
	if err := loc.Client.CallResult(context.Background(), "Hello", nil, &result); err != nil {
		log.Fatalf("Call failed: %v", err)
	}
	fmt.Println(result)
	// Output:
	// Hello, world!
}

// Service is a trivial service for testing purposes.
type Service struct {
	done chan struct{}
}

func (Service) Assigner() (jrpc2.Assigner, error) {
	fmt.Println("SERVICE STARTED")
	return handler.Map{"Hello": handler.New(func(ctx context.Context) error {
		fmt.Println("Hello human")
		return nil
	})}, nil
}

func (s Service) Finish(_ jrpc2.Assigner, stat jrpc2.ServerStatus) {
	fmt.Printf("SERVICE FINISHED err=%v\n", stat.Err)
	close(s.done)
}

func ExampleRun() {
	done := make(chan struct{})
	cch, sch := channel.Direct()
	go server.Run(sch, Service{done}, nil)

	cli := jrpc2.NewClient(cch, nil)
	if _, err := cli.Call(context.Background(), "Hello", nil); err != nil {
		log.Fatalf("Call failed: %v", err)
	}
	cli.Close()
	<-done
	// Output:
	// SERVICE STARTED
	// Hello human
	// SERVICE FINISHED err=<nil>
}
