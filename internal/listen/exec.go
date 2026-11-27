//go:build darwin || windows

package listen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// lsof and powershell both stall on a wedged machine. Bound it.
const cmdTimeout = 15 * time.Second

func execCmd(timeout time.Duration, name string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("%s timed out", name)
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func cmdErr(name string, err error, stderr []byte) error {
	msg := strings.TrimSpace(string(stderr))
	if msg == "" {
		return fmt.Errorf("%s: %w", name, err)
	}
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	return errors.New(msg)
}

func argv0(cmdline string) string {
	if cmdline == "" {
		return ""
	}
	a, _, _ := strings.Cut(cmdline, " ")
	return filepath.Base(strings.TrimSpace(a))
}
