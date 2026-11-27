//go:build darwin

package listen

import (
	"bufio"
	"bytes"
	"errors"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/frozenmango8/port-who/internal/lsof"
)

func list(port int) ([]Socket, string, error) {
	args := []string{"-nP", "-FpcnPuLT", "-iTCP", "-sTCP:LISTEN", "-iUDP"}
	if port > 0 {
		p := strconv.Itoa(port)
		args = []string{"-nP", "-FpcnPuLT", "-iTCP:" + p, "-sTCP:LISTEN", "-iUDP:" + p}
	}

	stdout, stderr, err := execCmd(cmdTimeout, "lsof", args...)
	warn := lsofWarn(stderr)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, "", errors.New("lsof not found")
		}
		if len(bytes.TrimSpace(stdout)) == 0 && lsofEmpty(err, stderr) {
			return nil, warn, nil
		}
		if len(bytes.TrimSpace(stdout)) == 0 {
			return nil, "", cmdErr("lsof", err, stderr)
		}
	}

	entries, err := lsof.Parse(bytes.NewReader(stdout))
	if err != nil {
		return nil, "", err
	}
	cmds := psCmds()
	var out []Socket
	for _, e := range entries {
		if port > 0 && e.Port != port {
			continue
		}
		cmd := cmds[e.PID]
		name := e.Command
		if base := argv0(cmd); base != "" {
			name = base
		}
		out = append(out, Socket{
			PID:     e.PID,
			Name:    name,
			Cmdline: cmd,
			User:    resolveUser(e.User),
			Proto:   e.Protocol,
			Addr:    e.Addr,
			Port:    e.Port,
		})
	}
	sortSockets(out)
	return out, warn, nil
}

func lsofWarn(stderr []byte) string {
	if strings.Contains(strings.ToLower(string(stderr)), "permission") {
		return "lsof could not see some processes; rerun as root"
	}
	return ""
}

func lsofEmpty(err error, stderr []byte) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 1 {
		return false
	}
	s := strings.TrimSpace(strings.ToLower(string(stderr)))
	if s == "" {
		return true
	}
	return strings.Contains(s, "warning") || strings.Contains(s, "permission")
}

func psCmds() map[int]string {
	stdout, _, err := execCmd(cmdTimeout, "ps", "-ax", "-o", "pid=,command=")
	if err != nil {
		return nil
	}
	m := map[int]string{}
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		pidStr, cmd, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		m[pid] = strings.TrimSpace(cmd)
	}
	return m
}

func resolveUser(s string) string {
	if s == "" {
		return ""
	}
	if _, err := strconv.ParseUint(s, 10, 32); err != nil {
		return s
	}
	u, err := user.LookupId(s)
	if err != nil || u.Username == "" {
		return s
	}
	return u.Username
}
