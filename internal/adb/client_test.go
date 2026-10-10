package adb

import "testing"

func TestAddressFormat(t *testing.T) {
	c := New(5555)
	if c.Address != "127.0.0.1:5555" {
		t.Fatalf("bad address %q", c.Address)
	}
}
