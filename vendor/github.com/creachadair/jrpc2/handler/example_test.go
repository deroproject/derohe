// Copyright (C) 2017 Michael J. Fromberger. All Rights Reserved.

package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
)

func ExampleCheck() {
	fi, err := handler.Check(func(_ context.Context, ss []string) int { return len(ss) })
	if err != nil {
		log.Fatalf("Check failed: %v", err)
	}
	fmt.Printf("Argument type: %v\n", fi.Argument)
	fmt.Printf("Result type:   %v\n", fi.Result)
	fmt.Printf("Reports error? %v\n", fi.ReportsError)
	fmt.Printf("Wrapped type:  %T\n", fi.Wrap())
	// Output:
	// Argument type: []string
	// Result type:   int
	// Reports error? false
	// Wrapped type:  handler.Func
}

func ExampleArgs_unmarshal() {
	const input = `[25, false, "apple"]`

	var count int
	var item string

	if err := json.Unmarshal([]byte(input), &handler.Args{&count, nil, &item}); err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}
	fmt.Printf("count=%d, item=%q\n", count, item)
	// Output:
	// count=25, item="apple"
}

func ExampleArgs_marshal() {
	bits, err := json.Marshal(handler.Args{1, "foo", false, nil})
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}
	fmt.Println(string(bits))
	// Output:
	// [1,"foo",false,null]
}

func ExampleObj_unmarshal() {
	const input = `{"uid": 501, "name": "P. T. Barnum", "tags": [1, 3]}`

	var uid int
	var name string

	if err := json.Unmarshal([]byte(input), &handler.Obj{
		"uid":  &uid,
		"name": &name,
	}); err != nil {
		log.Fatalf("Decoding failed: %v", err)
	}
	fmt.Printf("uid=%d, name=%q\n", uid, name)
	// Output:
	// uid=501, name="P. T. Barnum"
}

func describe(_ context.Context, name string, age int, isOld bool) error {
	fmt.Printf("%s is %d (old: %v)\n", name, age, isOld)
	return nil
}

func ExamplePositional_object() {
	call := handler.NewPos(describe, "name", "age", "isOld")

	req := mustParseReq(`{
	  "jsonrpc": "2.0",
	  "id": 1,
	  "method": "foo",
	  "params": {
	    "name":  "Dennis",
	    "age":   37,
	    "isOld": false
	  }
	}`)
	if _, err := call(context.Background(), req); err != nil {
		log.Fatalf("Call: %v", err)
	}
	// Output:
	// Dennis is 37 (old: false)
}

func ExamplePositional_array() {
	call := handler.NewPos(describe, "name", "age", "isOld")

	req := mustParseReq(`{
	  "jsonrpc": "2.0",
	  "id": 1,
	  "method": "foo",
	  "params": ["Marvin", 973000, true]
	}`)
	if _, err := call(context.Background(), req); err != nil {
		log.Fatalf("Call: %v", err)
	}
	// Output:
	// Marvin is 973000 (old: true)
}

func mustParseReq(s string) *jrpc2.Request {
	reqs, err := jrpc2.ParseRequests([]byte(s))
	if err != nil {
		log.Fatalf("ParseRequests: %v", err)
	} else if len(reqs) == 0 {
		log.Fatal("ParseRequests: empty result")
	}
	return reqs[0]
}
