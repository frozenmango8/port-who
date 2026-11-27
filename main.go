package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/frozenmango8/port-who/internal/listen"
)

// version is overridden by goreleaser (-X main.version).
var version = "dev"

const usageText = `usage: port-who [flags] [port]

List listening TCP sockets and bound UDP sockets, or just one port.

  port-who
  port-who 3000
  port-who --json 3000
  port-who --kill 3000

flags:
  --json       print JSON
  --kill       signal the listener (SIGTERM, or taskkill on Windows)
  -y, --yes    do not ask before killing
  --version    print version
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flagArgs, portStr, err := splitArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "port-who: %v\n", err)
		return 2
	}

	fs := flag.NewFlagSet("port-who", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usageText) }
	jsonOut := fs.Bool("json", false, "print JSON")
	doKill := fs.Bool("kill", false, "signal the process listening on the port")
	yes := fs.Bool("y", false, "do not ask for confirmation")
	fs.BoolVar(yes, "yes", false, "do not ask for confirmation")
	showVersion := fs.Bool("version", false, "print version")

	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Printf("port-who %s\n", version)
		return 0
	}

	port, err := parsePort(portStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "port-who: %v\n", err)
		return 2
	}
	if *doKill && port == 0 {
		fmt.Fprintln(os.Stderr, "port-who: --kill needs a port")
		return 2
	}

	var sockets []listen.Socket
	var warn string
	if port == 0 {
		sockets, warn, err = listen.All()
	} else {
		sockets, warn, err = listen.Port(port)
	}
	if warn != "" {
		fmt.Fprintf(os.Stderr, "port-who: %s\n", warn)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "port-who: %v\n", err)
		return 1
	}
	if port != 0 && len(sockets) == 0 {
		if *jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Fprintf(os.Stderr, "port-who: nothing listening on port %d\n", port)
		}
		return 1
	}
	if *doKill {
		return killSockets(sockets, *yes)
	}
	if *jsonOut {
		return printJSON(sockets)
	}
	printTable(sockets)
	return 0
}

func splitArgs(args []string) ([]string, string, error) {
	var flags []string
	var port string
	dash := false
	for _, a := range args {
		if dash {
			if port != "" {
				return nil, "", errors.New("too many arguments")
			}
			port = a
			dash = false
			continue
		}
		if a == "--" {
			dash = true
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			flags = append(flags, a)
			continue
		}
		if port != "" {
			return nil, "", errors.New("too many arguments")
		}
		port = a
	}
	if dash {
		return nil, "", errors.New("too many arguments")
	}
	return flags, port, nil
}

func parsePort(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return n, nil
}

type row struct {
	PID     int    `json:"pid,omitempty"`
	Name    string `json:"name,omitempty"`
	Cmdline string `json:"cmdline,omitempty"`
	User    string `json:"user,omitempty"`
	Proto   string `json:"proto"`
	Addr    string `json:"addr"`
}

func printJSON(sockets []listen.Socket) int {
	rows := make([]row, 0, len(sockets))
	for _, s := range sockets {
		rows = append(rows, row{
			PID:     s.PID,
			Name:    s.Name,
			Cmdline: s.Cmdline,
			User:    s.User,
			Proto:   s.Proto,
			Addr:    s.Addr,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rows); err != nil {
		fmt.Fprintf(os.Stderr, "port-who: %v\n", err)
		return 1
	}
	return 0
}

func printTable(sockets []listen.Socket) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PID\tUSER\tPROTO\tADDRESS\tNAME\tCOMMAND")
	for _, s := range sockets {
		pid := "-"
		if s.PID > 0 {
			pid = strconv.Itoa(s.PID)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			pid, dash(s.User), s.Proto, s.Addr, dash(s.Name), s.Cmdline)
	}
	w.Flush()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func killSockets(sockets []listen.Socket, yes bool) int {
	targets := distinct(sockets)
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "port-who: no visible process for that port; try running as root")
		return 1
	}
	if !yes && !confirm(targets) {
		return 1
	}
	code := 0
	for _, s := range targets {
		if err := signal(s.PID); err != nil {
			fmt.Fprintf(os.Stderr, "port-who: %d: %v\n", s.PID, err)
			code = 1
			continue
		}
		fmt.Println(signaled(s.PID))
	}
	return code
}

func distinct(sockets []listen.Socket) []listen.Socket {
	seen := map[int]bool{}
	var out []listen.Socket
	for _, s := range sockets {
		if s.PID <= 0 || seen[s.PID] {
			continue
		}
		seen[s.PID] = true
		out = append(out, s)
	}
	return out
}

func confirm(targets []listen.Socket) bool {
	if !isTerminal(os.Stdin.Fd()) {
		fmt.Fprintln(os.Stderr, "port-who: not a terminal; pass -y to kill without prompting")
		return false
	}
	if len(targets) == 1 {
		fmt.Fprintf(os.Stderr, "terminate %d (%s)? [y/N] ", targets[0].PID, dash(targets[0].Name))
	} else {
		fmt.Fprintln(os.Stderr, "port is used by:")
		for _, s := range targets {
			fmt.Fprintf(os.Stderr, "  %d  %s\n", s.PID, dash(s.Name))
		}
		fmt.Fprintf(os.Stderr, "terminate these %d processes? [y/N] ", len(targets))
	}
	var ans string
	if _, err := fmt.Scanln(&ans); err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
