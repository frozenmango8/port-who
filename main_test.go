package main

import "testing"

func TestParsePort(t *testing.T) {
	n, err := parsePort("")
	if err != nil || n != 0 {
		t.Fatalf("empty: %d %v", n, err)
	}
	n, err = parsePort("3000")
	if err != nil || n != 3000 {
		t.Fatalf("3000: %d %v", n, err)
	}
	n, err = parsePort("65535")
	if err != nil || n != 65535 {
		t.Fatalf("65535: %d %v", n, err)
	}
	for _, bad := range []string{"0", "65536", "-1", "http", "3000 "} {
		if _, err := parsePort(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func TestSplitArgs(t *testing.T) {
	flags, port, err := splitArgs([]string{"3000", "--json", "-y"})
	if err != nil || port != "3000" || len(flags) != 2 || flags[0] != "--json" || flags[1] != "-y" {
		t.Fatalf("flags=%v port=%q err=%v", flags, port, err)
	}
	flags, port, err = splitArgs([]string{"--kill", "--", "80"})
	if err != nil || port != "80" || len(flags) != 1 {
		t.Fatalf("flags=%v port=%q err=%v", flags, port, err)
	}
	if _, _, err := splitArgs([]string{"80", "443"}); err == nil {
		t.Fatal("two ports accepted")
	}
	if _, _, err := splitArgs([]string{"--"}); err == nil {
		t.Fatal("dangling -- accepted")
	}
}
