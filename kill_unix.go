//go:build unix

package main

import (
	"fmt"
	"syscall"
)

func signal(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to signal pid %d", pid)
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}

func signaled(pid int) string {
	return fmt.Sprintf("sent SIGTERM to %d", pid)
}
