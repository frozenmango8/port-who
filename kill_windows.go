//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func signal(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to signal pid %d", pid)
	}
	out, err := exec.Command("taskkill", "/PID", strconv.Itoa(pid)).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return errors.New(msg)
	}
	return nil
}

func signaled(pid int) string {
	return fmt.Sprintf("sent taskkill to %d", pid)
}
