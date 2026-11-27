package lsof

import (
	"os"
	"reflect"
	"testing"
)

func TestParseFixture(t *testing.T) {
	f, err := os.Open("testdata/listen.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{PID: 48211, Command: "node", User: "oliver", Protocol: "tcp", Addr: "0.0.0.0:3000", Port: 3000},
		{PID: 48211, Command: "node", User: "oliver", Protocol: "tcp", Addr: "[::1]:3000", Port: 3000},
		{PID: 48211, Command: "node", User: "oliver", Protocol: "udp", Addr: "127.0.0.1:5353", Port: 5353},
		{PID: 912, Command: "sshd", User: "root", Protocol: "tcp", Addr: "0.0.0.0:22", Port: 22},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %#v\nwant %#v", got, want)
	}
}
