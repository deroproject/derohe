package x

import (
	"testing"
)

func TestIP2Decimal(t *testing.T) {
	r, err := IP2Decimal0("192.168.1.10")
	if err != nil {
		t.Fatal(r, err)
	}
	t.Log(r)
	r, err = IP2Decimal1("192.168.1.10")
	if err != nil {
		t.Fatal(r, err)
	}
	t.Log(r)
	r, err = IP2Decimal("192.168.1.10")
	if err != nil {
		t.Fatal(r, err)
	}
	t.Log(r)
}

func TestDecimal2IP(t *testing.T) {
	r, err := Decimal2IP0(123423434)
	if err != nil {
		t.Fatal(r, err)
	}
	t.Log(r)
	r = Decimal2IP1(123423434)
	if err != nil {
		t.Fatal(r, err)
	}
	t.Log(r)
	r = Decimal2IP(123423434)
	if err != nil {
		t.Fatal(r, err)
	}
	t.Log(r)
}

func TestCIDR(t *testing.T) {
	r, err := CIDR("192.168.1.10/6")
	if err != nil {
		t.Fatal(r, err)
	}
	t.Log(r)
}
