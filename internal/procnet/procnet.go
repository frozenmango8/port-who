// Package procnet parses the kernel socket tables in /proc/net.
package procnet

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// Proto is "tcp" or "udp". IPv6 tables use the same names; the address
// family is in the IP.
type Proto string

const (
	TCP Proto = "tcp"
	UDP Proto = "udp"
)

// tcpListen and tcpClose match include/net/tcp_states.h.
// UDP has no LISTEN state. A bound, unconnected UDP socket is reported
// as TCP_CLOSE with a zero remote port.
const (
	tcpListen = 0x0A
	tcpClose  = 0x07
)

const (
	fldLocal = 1
	fldRem   = 2
	fldState = 3
	fldUID   = 7
	fldInode = 9
)

// Entry is one listening socket from a /proc/net table.
type Entry struct {
	Proto Proto
	IP    net.IP
	Port  int
	UID   uint32
	Inode uint64
}

// Parse reads one /proc/net/{tcp,tcp6,udp,udp6} table.
// The first line is the header and is discarded.
// Only listening TCP sockets and unconnected bound UDP sockets are returned.
func Parse(r io.Reader, proto Proto) ([]Entry, error) {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}

	var out []Entry
	lineNo := 1
	for sc.Scan() {
		lineNo++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		e, ok, err := parseLine(text, proto)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if ok {
			out = append(out, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseLine(line string, proto Proto) (Entry, bool, error) {
	f := strings.Fields(line)
	if len(f) <= fldInode {
		return Entry{}, false, fmt.Errorf("short line")
	}

	ip, port, err := parseAddr(f[fldLocal])
	if err != nil {
		return Entry{}, false, err
	}
	_, remPort, err := parseAddr(f[fldRem])
	if err != nil {
		return Entry{}, false, err
	}
	st, err := strconv.ParseUint(f[fldState], 16, 8)
	if err != nil {
		return Entry{}, false, fmt.Errorf("state: %w", err)
	}
	if !want(proto, st, remPort) {
		return Entry{}, false, nil
	}
	uid, err := strconv.ParseUint(f[fldUID], 10, 32)
	if err != nil {
		return Entry{}, false, fmt.Errorf("uid: %w", err)
	}
	inode, err := strconv.ParseUint(f[fldInode], 10, 64)
	if err != nil {
		return Entry{}, false, fmt.Errorf("inode: %w", err)
	}
	return Entry{
		Proto: proto,
		IP:    ip,
		Port:  port,
		UID:   uint32(uid),
		Inode: inode,
	}, true, nil
}

func want(proto Proto, state uint64, remPort int) bool {
	switch proto {
	case TCP:
		return state == tcpListen
	case UDP:
		return state == tcpClose && remPort == 0
	default:
		return false
	}
}

// parseAddr decodes the hex "ADDR:PORT" field.
//
// The kernel prints each native-endian u32 with %08X (see get_tcp4_sock
// and get_tcp6_sock). On little-endian that reverses each 4-byte group,
// so 127.0.0.1 is the text 0100007F.
func parseAddr(s string) (net.IP, int, error) {
	host, portStr, ok := strings.Cut(s, ":")
	if !ok || (len(host) != 8 && len(host) != 32) {
		return nil, 0, fmt.Errorf("bad address %q", s)
	}
	port, err := strconv.ParseUint(portStr, 16, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("port: %w", err)
	}

	raw := make([]byte, len(host)/2)
	for i := 0; i < len(host); i += 8 {
		word, err := strconv.ParseUint(host[i:i+8], 16, 32)
		if err != nil {
			return nil, 0, fmt.Errorf("address: %w", err)
		}
		off := i / 2
		binary.NativeEndian.PutUint32(raw[off:off+4], uint32(word))
	}
	return net.IP(raw), int(port), nil
}
