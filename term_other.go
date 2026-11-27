//go:build !linux && !darwin && !windows

package main

func isTerminal(uintptr) bool { return false }
