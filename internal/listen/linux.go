//go:build linux

package listen

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/frozenmango8/port-who/internal/procnet"
)

func list(port int) ([]Socket, string, error) {
	return listProc("/proc", port)
}

func listProc(root string, port int) ([]Socket, string, error) {
	specs := []struct {
		name  string
		proto procnet.Proto
	}{
		{"tcp", procnet.TCP},
		{"tcp6", procnet.TCP},
		{"udp", procnet.UDP},
		{"udp6", procnet.UDP},
	}

	var entries []procnet.Entry
	missing := 0
	for _, sp := range specs {
		path := filepath.Join(root, "net", sp.name)
		got, err := readTable(path, sp.proto)
		if err != nil {
			if os.IsNotExist(err) {
				missing++
				continue
			}
			return nil, "", err
		}
		for _, e := range got {
			if port != 0 && e.Port != port {
				continue
			}
			entries = append(entries, e)
		}
	}
	if missing == len(specs) {
		return nil, "", fmt.Errorf("%s: no tcp or udp tables", filepath.Join(root, "net"))
	}

	want := make(map[uint64]struct{}, len(entries))
	for _, e := range entries {
		want[e.Inode] = struct{}{}
	}
	owners, denied, err := socketOwners(root, want)
	if err != nil {
		return nil, "", err
	}

	names := map[uint32]string{}
	var out []Socket
	hidden := 0
	for _, e := range entries {
		s := Socket{
			Proto: string(e.Proto),
			Addr:  net.JoinHostPort(e.IP.String(), strconv.Itoa(e.Port)),
			Port:  e.Port,
			User:  userName(names, e.UID),
		}
		if o, ok := owners[e.Inode]; ok {
			s.PID = o.pid
			s.Name = o.name
			s.Cmdline = o.cmdline
			if o.uidSet {
				s.User = userName(names, o.uid)
			}
		} else if denied {
			hidden++
		}
		out = append(out, s)
	}
	sortSockets(out)

	var warn string
	if hidden > 0 {
		warn = "some sockets belong to other users; rerun as root to see the process"
	}
	return out, warn, nil
}

func readTable(path string, proto procnet.Proto) ([]procnet.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ents, err := procnet.Parse(f, proto)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return ents, nil
}

type owner struct {
	pid     int
	name    string
	cmdline string
	uid     uint32
	uidSet  bool
}

func socketOwners(root string, want map[uint64]struct{}) (map[uint64]owner, bool, error) {
	found := make(map[uint64]owner)
	if len(want) == 0 {
		return found, false, nil
	}
	dents, err := os.ReadDir(root)
	if err != nil {
		return nil, false, err
	}

	denied := false
	for _, d := range dents {
		if len(found) == len(want) {
			break
		}
		pid, err := strconv.Atoi(d.Name())
		if err != nil || !d.IsDir() {
			continue
		}
		fdDir := filepath.Join(root, d.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			// Other users' fd directories are mode 0500.
			if os.IsPermission(err) {
				denied = true
			}
			continue
		}

		var info *owner
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			ino, ok := socketInode(target)
			if !ok {
				continue
			}
			if _, needed := want[ino]; !needed {
				continue
			}
			if _, already := found[ino]; already {
				continue
			}
			if info == nil {
				o := readOwner(root, pid)
				info = &o
			}
			found[ino] = *info
		}
	}
	return found, denied, nil
}

func socketInode(link string) (uint64, bool) {
	const pfx = "socket:["
	if !strings.HasPrefix(link, pfx) || !strings.HasSuffix(link, "]") {
		return 0, false
	}
	n, err := strconv.ParseUint(link[len(pfx):len(link)-1], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func readOwner(root string, pid int) owner {
	base := filepath.Join(root, strconv.Itoa(pid))
	o := owner{pid: pid}
	if b, err := os.ReadFile(filepath.Join(base, "comm")); err == nil {
		o.name = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(base, "cmdline")); err == nil {
		o.cmdline = joinCmdline(b)
	}
	if b, err := os.ReadFile(filepath.Join(base, "status")); err == nil {
		if uid, ok := statusUID(string(b)); ok {
			o.uid = uid
			o.uidSet = true
		}
	}
	if o.name == "" {
		o.name = argvBase(o.cmdline)
	}
	return o
}

func joinCmdline(b []byte) string {
	parts := strings.Split(string(b), "\x00")
	var tok []string
	for _, p := range parts {
		if p != "" {
			tok = append(tok, p)
		}
	}
	return strings.Join(tok, " ")
}

func argvBase(cmdline string) string {
	if cmdline == "" {
		return ""
	}
	a, _, _ := strings.Cut(cmdline, " ")
	return filepath.Base(a)
}

func statusUID(s string) (uint32, bool) {
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			return 0, false
		}
		n, err := strconv.ParseUint(f[1], 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(n), true
	}
	return 0, false
}

func userName(cache map[uint32]string, uid uint32) string {
	if s, ok := cache[uid]; ok {
		return s
	}
	id := strconv.FormatUint(uint64(uid), 10)
	s := id
	if u, err := user.LookupId(id); err == nil && u.Username != "" {
		s = u.Username
	}
	cache[uid] = s
	return s
}
