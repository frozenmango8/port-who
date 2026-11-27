//go:build linux

package listen

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestListProc(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "net", "tcp"), ""+
		"  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"+
		"   0: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 12345 1 0000000000000000 100 0 0 10 0\n"+
		"   1: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 999 1 0000000000000000 100 0 0 10 0\n")

	fdDir := filepath.Join(root, "4321", "fd")
	if err := os.MkdirAll(fdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("socket:[12345]", filepath.Join(fdDir, "3")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("pipe:[12]", filepath.Join(fdDir, "1")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "4321", "comm"), "node\n")
	writeFile(t, filepath.Join(root, "4321", "cmdline"), "node\x00server.js\x00")
	writeFile(t, filepath.Join(root, "4321", "status"), "Name:\tnode\nUid:\t0\t0\t0\t0\n")

	hidden := filepath.Join(root, "99", "fd")
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		if err := os.Chmod(hidden, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(hidden, 0o755) })
	}

	got, warn, err := listProc(root, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sockets, want 1: %+v", len(got), got)
	}
	s := got[0]
	if s.PID != 4321 || s.Name != "node" || s.Cmdline != "node server.js" {
		t.Fatalf("process = pid %d name %q cmd %q", s.PID, s.Name, s.Cmdline)
	}
	if s.Proto != "tcp" || s.Addr != "127.0.0.1:3000" || s.Port != 3000 {
		t.Fatalf("socket = %s %s port %d", s.Proto, s.Addr, s.Port)
	}
	if s.User != lookup("0") {
		t.Fatalf("user = %q, want %q", s.User, lookup("0"))
	}
	if warn != "" {
		t.Fatalf("port query warned unexpectedly: %s", warn)
	}

	all, warn, err := listProc(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Port != 80 || all[1].Port != 3000 {
		t.Fatalf("all = %+v", all)
	}
	if all[0].PID != 0 {
		t.Fatalf("unmatched socket got pid %d", all[0].PID)
	}
	if os.Geteuid() != 0 && warn == "" {
		t.Fatal("expected a permission warning for the unread fd directory")
	}
}

func lookup(id string) string {
	u, err := user.LookupId(id)
	if err != nil || u.Username == "" {
		return id
	}
	return u.Username
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSocketInode(t *testing.T) {
	n, ok := socketInode("socket:[12345]")
	if !ok || n != 12345 {
		t.Fatalf("got %d %v", n, ok)
	}
	if _, ok := socketInode("pipe:[12]"); ok {
		t.Fatal("pipe looked like a socket")
	}
	if _, ok := socketInode("socket:[nope]"); ok {
		t.Fatal("bad inode accepted")
	}
}

func TestStatusUID(t *testing.T) {
	uid, ok := statusUID("Name:\tnode\nUid:\t1000\t1000\t1000\t1000\n")
	if !ok || uid != 1000 {
		t.Fatalf("got %d %v", uid, ok)
	}
	if _, ok := statusUID("Name:\tnode\n"); ok {
		t.Fatal("missing Uid parsed")
	}
}
