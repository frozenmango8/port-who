package procnet

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseFixtures(t *testing.T) {
	if !littleEndian() {
		t.Skip("fixtures are little-endian /proc dumps")
	}

	tests := []struct {
		file  string
		proto Proto
		want  []Entry
	}{
		{
			file:  "tcp",
			proto: TCP,
			want: []Entry{
				{Proto: TCP, IP: net.ParseIP("127.0.0.1").To4(), Port: 3000, UID: 1000, Inode: 12345},
				{Proto: TCP, IP: net.ParseIP("0.0.0.0").To4(), Port: 80, UID: 0, Inode: 999},
			},
		},
		{
			file:  "tcp6",
			proto: TCP,
			want: []Entry{
				{Proto: TCP, IP: net.ParseIP("::"), Port: 3000, UID: 1000, Inode: 4242},
				{Proto: TCP, IP: net.ParseIP("::1"), Port: 80, UID: 0, Inode: 4243},
			},
		},
		{
			file:  "udp",
			proto: UDP,
			want: []Entry{
				{Proto: UDP, IP: net.ParseIP("0.0.0.0").To4(), Port: 5353, UID: 100, Inode: 7777},
			},
		},
		{
			file:  "udp6",
			proto: UDP,
			want: []Entry{
				{Proto: UDP, IP: net.ParseIP("::"), Port: 5353, UID: 100, Inode: 8888},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			got, err := Parse(f, tt.proto)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestParseHeaderOnly(t *testing.T) {
	const hdr = "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	got, err := Parse(strings.NewReader(hdr), TCP)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d entries, want 0", len(got))
	}
}

func TestParseShortLine(t *testing.T) {
	in := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n   0: broken\n"
	_, err := Parse(strings.NewReader(in), TCP)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func littleEndian() bool {
	var b [2]byte
	binary.NativeEndian.PutUint16(b[:], 0x0102)
	return b[0] == 0x02
}
