package main

import (
	"bytes"
	"testing"
)

func TestWriteVersion(t *testing.T) {
	previous := version
	version = "0.1.0"
	t.Cleanup(func() { version = previous })

	for _, flag := range []string{"--version", "-v"} {
		var output bytes.Buffer
		if !writeVersion([]string{flag}, &output) {
			t.Fatalf("writeVersion(%q) did not handle version flag", flag)
		}
		if got, want := output.String(), "thisyou 0.1.0\n"; got != want {
			t.Fatalf("writeVersion(%q) = %q, want %q", flag, got, want)
		}
	}
}

func TestWriteVersionIgnoresOtherArguments(t *testing.T) {
	var output bytes.Buffer
	if writeVersion(nil, &output) {
		t.Fatal("writeVersion(nil) handled missing flag")
	}
	if writeVersion([]string{"--help"}, &output) {
		t.Fatal("writeVersion handled unrelated flag")
	}
	if output.Len() != 0 {
		t.Fatalf("writeVersion wrote %q", output.String())
	}
}
