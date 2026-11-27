// Package netstat parses `netstat -ano` output from Windows.
package netstat

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// Entry is one listening socket from netstat.
type Entry struct {
	Protocol string
	Addr     string
	Port     int
	PID      int
}

// Parse reads `netstat -ano` text.
//
// The state column is localized, so TCP listeners are detected by a
// zero foreign address (0.0.0.0:0 or [::]:0) rather than the word
// LISTENING. UDP sockets with a foreign address of *:* are bound.
func Parse(r io.Reader) ([]Entry, error) {
	sc := bufio.NewScanner(r)
	var out []Entry
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 {
			continue
		}
		kind := strings.ToLower(f[0])
		var proto string
		switch {
		case strings.HasPrefix(kind, "tcp"):
			proto = "tcp"
		case strings.HasPrefix(kind, "udp"):
			proto = "udp"
		default:
			continue
		}

		if proto == "tcp" && !tcpListen(f[2]) {
			continue
		}
		if proto == "udp" && !udpBound(f[2]) {
			continue
		}

		addr, port, err := hostPort(f[1])
		if err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(f[len(f)-1])
		if err != nil {
			return nil, fmt.Errorf("pid %q: %w", f[len(f)-1], err)
		}
		out = append(out, Entry{Protocol: proto, Addr: addr, Port: port, PID: pid})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func tcpListen(foreign string) bool {
	switch foreign {
	case "0.0.0.0:0", "[::]:0", "*:0":
		return true
	default:
		return false
	}
}

func udpBound(foreign string) bool {
	switch foreign {
	case "*:*", "[::]:*", "*:0":
		return true
	default:
		return false
	}
}

func hostPort(s string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, fmt.Errorf("address %q: %w", s, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("port in %q: %w", s, err)
	}
	return net.JoinHostPort(host, portStr), port, nil
}
