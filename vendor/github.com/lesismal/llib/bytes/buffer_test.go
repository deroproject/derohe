package bytes

import (
	"testing"
)

func TestBuffer(t *testing.T) {
	str := "hello world"

	buffer := NewBuffer()
	buffer.Write([]byte("hel"))
	buffer.Write([]byte("lo world"))
	b, err := buffer.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != str {
		t.Fatal(string(b))
	}

	buffer.Write([]byte("hel"))
	buffer.Write([]byte("lo "))
	buffer.Write([]byte("wor"))
	buffer.Write([]byte("ld"))
	for i := 0; i < len(str); i++ {
		for j := i; j < len(str); j++ {
			sub, err := buffer.Sub(i, j)
			if err != nil {
				t.Fatal(err)
			}
			if string(sub) != string([]byte(str)[i:j]) {
				t.Fatalf("[%v:%v] %v != %v", i, j, string(sub), string([]byte(str)[i:j]))
			}
		}
	}

	for i := 0; i < len(str); i++ {
		for j := i; j < len(str); j++ {
			buffer.Write([]byte("hel"))
			buffer.Write([]byte("lo "))
			buffer.Write([]byte("wor"))
			buffer.Write([]byte("ld"))

			b, err = buffer.Read(j)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != string([]byte(str)[:j]) {
				t.Fatalf("[%v:%v] %v != %v", i, j, string(b), string([]byte(str)[:j]))
			}

			buffer.Reset()
		}
	}

	for i := 0; i < len(str); i++ {
		for j := i; j < len(str); j++ {
			buffer.Write([]byte("hel"))
			buffer.Write([]byte("lo "))
			buffer.Write([]byte("wor"))
			buffer.Write([]byte("ld"))

			buffer.Read(i)
			b, err = buffer.Read(j - i)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != string([]byte(str)[i:j]) {
				t.Fatalf("[%v:%v] %v != %v", i, j, string(b), string([]byte(str)[i:j]))
			}

			buffer.Reset()
		}
	}

	buffer.Append([]byte("hello"))
	buffer.Append([]byte(" world"))
	if string(buffer.buffers[0]) != "hello world" {
		t.Fatal(string(buffer.buffers[0]))
	}
	b, err = buffer.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello world" {
		t.Fatal(string(b))
	}

	buffer.Reset()

	buffer.Push([]byte("hello "))
	buffer.Push([]byte("world"))
	if string(buffer.buffers[0]) != "hello " {
		t.Fatal(string(buffer.buffers[0]))
	}
	buffer.Pop(1)
	if string(buffer.buffers[0]) != "ello " {
		t.Fatal(string(buffer.buffers[0]))
	}
	buffer.Pop(5)
	if string(buffer.buffers[0]) != "world" {
		t.Fatal(string(buffer.buffers[0]))
	}
	buffer.ReadAll()
	if len(buffer.buffers) != 0 {
		t.Fatal(string(buffer.buffers[0]))
	}
}
