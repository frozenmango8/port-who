//go:build windows

package listen

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/frozenmango8/port-who/internal/netstat"
)

func list(port int) ([]Socket, string, error) {
	stdout, stderr, err := execCmd(cmdTimeout, "netstat", "-ano")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, "", errors.New("netstat not found")
		}
		return nil, "", cmdErr("netstat", err, stderr)
	}
	entries, err := netstat.Parse(bytes.NewReader(stdout))
	if err != nil {
		return nil, "", err
	}

	procs, warn := winProcs()
	var out []Socket
	for _, e := range entries {
		if port > 0 && e.Port != port {
			continue
		}
		s := Socket{
			PID:   e.PID,
			Proto: e.Protocol,
			Addr:  e.Addr,
			Port:  e.Port,
		}
		if p, ok := procs[e.PID]; ok {
			s.Name = p.name
			s.Cmdline = p.cmd
			s.User = p.user
		}
		out = append(out, s)
	}
	sortSockets(out)
	return out, warn, nil
}

type winProc struct {
	name string
	cmd  string
	user string
}

func winProcs() (map[int]winProc, string) {
	m := map[int]winProc{}
	stdout, _, err := execCmd(cmdTimeout, "tasklist", "/FO", "CSV", "/NH", "/V")
	if err != nil {
		return m, "tasklist failed; process names are unavailable"
	}
	r := csv.NewReader(bytes.NewReader(stdout))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return m, "could not parse tasklist output"
		}
		if len(rec) < 7 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			continue
		}
		user := strings.TrimSpace(rec[6])
		if strings.EqualFold(user, "N/A") {
			user = ""
		}
		m[pid] = winProc{
			name: strings.TrimSpace(rec[0]),
			user: user,
		}
	}
	for pid, cmd := range winCmdlines() {
		p := m[pid]
		p.cmd = cmd
		if p.name == "" {
			p.name = argv0(cmd)
		}
		m[pid] = p
	}
	return m, ""
}

func winCmdlines() map[int]string {
	// PowerShell 5 writes UTF-16 to pipes unless OutputEncoding is set.
	script := "$OutputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding $false; " +
		"Get-CimInstance Win32_Process | ForEach-Object { if ($_.CommandLine) { \"{0}`t{1}\" -f $_.ProcessId, $_.CommandLine } }"
	stdout, _, err := execCmd(cmdTimeout, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		return nil
	}
	m := map[int]string{}
	sc := bufio.NewScanner(strings.NewReader(decodeOutput(stdout)))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		pidStr, cmd, ok := strings.Cut(strings.TrimRight(sc.Text(), "\r"), "\t")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil {
			continue
		}
		m[pid] = strings.TrimSpace(cmd)
	}
	return m
}

func decodeOutput(b []byte) string {
	if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		b = b[3:]
	}
	if bytes.HasPrefix(b, []byte{0xFF, 0xFE}) {
		b = b[2:]
		return strings.TrimPrefix(decodeUTF16LE(b), "\uFEFF")
	}
	if !bytes.Contains(b, []byte{0}) {
		return strings.TrimPrefix(string(b), "\uFEFF")
	}
	return strings.TrimPrefix(decodeUTF16LE(b), "\uFEFF")
}

func decodeUTF16LE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u))
}
