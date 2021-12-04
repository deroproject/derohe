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

func ExamplePositional() {
	fn := func(ctx context.Context, name string, age int, isOld bool) error {
		fmt.Printf("%s is %d (is old: %v)\n", name, age, isOld)
		return nil
	}
	call := handler.NewPos(fn, "name", "age", "isOld")

	req, err := jrpc2.ParseRequests([]byte(`
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "foo",
  "params": {
    "name": "Dennis",
    "age": 37,
    "isOld": false
  }
}`))
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}
	if _, err := call(context.Background(), req[0]); err != nil {
		log.Fatalf("Call: %v", err)
	}
	// Output:
	// Dennis is 37 (is old: false)
}
