// Package lsof parses the field output of `lsof -F`.
package lsof

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// Entry is one listening socket from lsof.
type Entry struct {
	PID      int
	Command  string
	User     string
	Protocol string
	Addr     string
	Port     int
}

type proc struct {
	pid   int
	cmd   string
	login string
	uid   string
}

type file struct {
	proto string
	typ   string
	name  string
	state string
}

// Parse reads `lsof -nP -FpcnPuLT` output.
// TCP sockets must be in LISTEN. UDP sockets must be unconnected.
func Parse(r io.Reader) ([]Entry, error) {
	sc := bufio.NewScanner(r)
	var cur proc
	var f file
	var out []Entry

	emit := func() error {
		e, ok, err := f.entry(cur)
		f = file{}
		if err != nil {
			return err
		}
		if ok {
			out = append(out, e)
		}
		return nil
	}

	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" {
			continue
		}
		id, val := line[0], line[1:]
		switch id {
		case 'p':
			if err := emit(); err != nil {
				return nil, err
			}
			pid, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("pid %q: %w", val, err)
			}
			cur = proc{pid: pid}
		case 'c':
			cur.cmd = val
		case 'L':
			cur.login = val
		case 'u':
			cur.uid = val
		case 'f':
			if err := emit(); err != nil {
				return nil, err
			}
		case 'P':
			f.proto = val
		case 't':
			f.typ = val
		case 'n':
			f.name = val
		case 'T':
			if strings.HasPrefix(val, "ST=") {
				f.state = strings.TrimPrefix(val, "ST=")
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := emit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (f file) entry(p proc) (Entry, bool, error) {
	if f.name == "" || f.proto == "" {
		return Entry{}, false, nil
	}
	proto := strings.ToLower(f.proto)
	switch proto {
	case "tcp":
		if f.state != "LISTEN" {
			return Entry{}, false, nil
		}
	case "udp":
		if strings.Contains(f.name, "->") {
			return Entry{}, false, nil
		}
	default:
		return Entry{}, false, nil
	}

	addr, port, err := normalize(f.name, f.typ)
	if err != nil {
		return Entry{}, false, err
	}
	user := p.login
	if user == "" {
		user = p.uid
	}
	return Entry{
		PID:      p.pid,
		Command:  p.cmd,
		User:     user,
		Protocol: proto,
		Addr:     addr,
		Port:     port,
	}, true, nil
}

// normalize turns lsof's NAME into host:port.
// "*" is the wildcard; the type field says which family.
func normalize(name, typ string) (string, int, error) {
	local, _, _ := strings.Cut(name, "->")
	host, portStr, err := splitHost(local)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("port in %q: %w", name, err)
	}
	if host == "*" {
		if typ == "IPv6" {
			host = "::"
		} else {
			host = "0.0.0.0"
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), port, nil
}

func splitHost(s string) (string, string, error) {
	if strings.HasPrefix(s, "[") {
		h, p, ok := strings.Cut(s, "]:")
		if !ok {
			return "", "", fmt.Errorf("bad address %q", s)
		}
		return strings.TrimPrefix(h, "["), p, nil
	}
	h, p, ok := strings.Cut(s, ":")
	if !ok || strings.Contains(p, ":") {
		return "", "", fmt.Errorf("bad address %q", s)
	}
	return h, p, nil
}
