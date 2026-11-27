package netstat

import (
	"os"
	"reflect"
	"testing"
)

func TestParseFixture(t *testing.T) {
	f, err := os.Open("testdata/ano.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Protocol: "tcp", Addr: "0.0.0.0:135", Port: 135, PID: 1148},
		{Protocol: "tcp", Addr: "0.0.0.0:3000", Port: 3000, PID: 48211},
		{Protocol: "tcp", Addr: "[::]:3000", Port: 3000, PID: 48211},
		{Protocol: "udp", Addr: "0.0.0.0:5353", Port: 5353, PID: 2104},
		{Protocol: "udp", Addr: "[::]:5353", Port: 5353, PID: 2104},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %#v\nwant %#v", got, want)
	}
}
