// Package listen finds local TCP listeners and bound UDP sockets.
//
// A non-empty warning means some process details were hidden from us.
package listen

import "sort"

// Socket is one listening TCP socket or one bound UDP socket.
type Socket struct {
	PID     int
	Name    string
	Cmdline string
	User    string
	Proto   string
	Addr    string
	Port    int
}

// All lists every listener visible to the current user.
func All() ([]Socket, string, error) {
	return list(0)
}

// Port lists listeners bound to port.
func Port(port int) ([]Socket, string, error) {
	return list(port)
}

func sortSockets(s []Socket) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Port != s[j].Port {
			return s[i].Port < s[j].Port
		}
		if s[i].Proto != s[j].Proto {
			return s[i].Proto < s[j].Proto
		}
		if s[i].Addr != s[j].Addr {
			return s[i].Addr < s[j].Addr
		}
		return s[i].PID < s[j].PID
	})
}
