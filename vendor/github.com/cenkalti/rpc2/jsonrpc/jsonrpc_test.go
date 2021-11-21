package jsonrpc

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cenkalti/rpc2"
)

const (
	network = "tcp4"
	addr    = "127.0.0.1:5000"
)

func TestJSONRPC(t *testing.T) {
	type Args struct{ A, B int }
	type Reply int

	lis, err := net.Listen(network, addr)
	if err != nil {
		t.Fatal(err)
	}

	srv := rpc2.NewServer()
	srv.Handle("add", func(client *rpc2.Client, args *Args, reply *Reply) error {
		*reply = Reply(args.A + args.B)

		var rep Reply
		err := client.Call("mult", Args{2, 3}, &rep)
		if err != nil {
			t.Fatal(err)
		}

		if rep != 6 {
			t.Fatalf("not expected: %d", rep)
		}

		return nil
	})
	srv.Handle("addPos", func(client *rpc2.Client, args []interface{}, result *float64) error {
		*result = args[0].(float64) + args[1].(float64)
		return nil
	})
	srv.Handle("rawArgs", func(client *rpc2.Client, args []json.RawMessage, reply *[]string) error {
		for _, p := range args {
			var str string
			json.Unmarshal(p, &str)
			*reply = append(*reply, str)
		}
		return nil
	})
	srv.Handle("typedArgs", func(client *rpc2.Client, args []int, reply *[]string) error {
		for _, p := range args {
			*reply = append(*reply, fmt.Sprintf("%d", p))
		}
		return nil
	})
	srv.Handle("nilArgs", func(client *rpc2.Client, args []interface{}, reply *[]string) error {
		for _, v := range args {
			if v == nil {
				*reply = append(*reply, "nil")
			}
		}
		return nil
	})
	number := make(chan int, 1)
	srv.Handle("set", func(client *rpc2.Client, i int, _ *struct{}) error {
		number <- i
		return nil
	})

	go func() {
		conn, err := lis.Accept()
		if err != nil {
			t.Fatal(err)
		}
		srv.ServeCodec(NewJSONCodec(conn))
	}()

	conn, err := net.Dial(network, addr)
	if err != nil {
		t.Fatal(err)
	}

	clt := rpc2.NewClientWithCodec(NewJSONCodec(conn))
	clt.Handle("mult", func(client *rpc2.Client, args *Args, reply *Reply) error {
		*reply = Reply(args.A * args.B)
		return nil
	})
	go clt.Run()

	// Test Call.
	var rep Reply
	err = clt.Call("add", Args{1, 2}, &rep)
	if err != nil {
		t.Fatal(err)
	}
	if rep != 3 {
		t.Fatalf("not expected: %d", rep)
	}

	// Test notification.
	err = clt.Notify("set", 6)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case i := <-number:
		if i != 6 {
			t.Fatalf("unexpected number: %d", i)
		}
	case <-time.After(time.Second):
		t.Fatal("did not get notification")
	}

	// Test undefined method.
	err = clt.Call("foo", 1, &rep)
	if err.Error() != "rpc2: can't find method foo" {
		t.Fatal(err)
	}

	// Test Positional arguments.
	var result float64
	err = clt.Call("addPos", []interface{}{1, 2}, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result != 3 {
		t.Fatalf("not expected: %f", result)
	}

	testArgs := func(expected, reply []string) error {
		if len(reply) != len(expected) {
			return fmt.Errorf("incorrect reply length: %d", len(reply))
		}
		for i := range expected {
			if reply[i] != expected[i] {
				return fmt.Errorf("not expected reply[%d]: %s", i, reply[i])
			}
		}
		return nil
	}

	// Test raw arguments (partial unmarshal)
	var reply []string
	var expected []string = []string{"arg1", "arg2"}
	rawArgs := json.RawMessage(`["arg1", "arg2"]`)
	err = clt.Call("rawArgs", rawArgs, &reply)
	if err != nil {
		t.Fatal(err)
	}

	if err = testArgs(expected, reply); err != nil {
		t.Fatal(err)
	}

	// Test typed arguments
	reply = []string{}
	expected = []string{"1", "2"}
	typedArgs := []int{1, 2}
	err = clt.Call("typedArgs", typedArgs, &reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = testArgs(expected, reply); err != nil {
		t.Fatal(err)
	}

	// Test nil args
	reply = []string{}
	expected = []string{"nil"}
	err = clt.Call("nilArgs", nil, &reply)
	if err != nil {
		t.Fatal(err)
	}
	if err = testArgs(expected, reply); err != nil {
		t.Fatal(err)
	}
}
