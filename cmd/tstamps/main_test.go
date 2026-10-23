package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr.String())
	}
	if got, want := stdout.String(), "tstamps 0.1.0-dev\n"; got != want {
		t.Fatalf("stdout %q, want %q", got, want)
	}
}

func TestUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"none", nil},
		{"unknown command", []string{"unknown"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != 2 {
				t.Errorf("run(%q) exit %d, want 2", tt.args, code)
			}
			if stderr.Len() == 0 {
				t.Errorf("run(%q) wrote no usage to stderr", tt.args)
			}
		})
	}
}

// TestServerCommandsPointElsewhere keeps the split honest: a stale unit file
// that runs `tstamps serve` gets told where serve lives instead of a bare
// "unknown command".
func TestServerCommandsPointElsewhere(t *testing.T) {
	for _, cmd := range []string{"serve", "admin", "openapi"} {
		t.Run(cmd, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{cmd}, &stdout, &stderr); code != 2 {
				t.Fatalf("exit %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), "tstamps-server") {
				t.Fatalf("stderr %q does not point at tstamps-server", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout %q, want nothing", stdout.String())
			}
		})
	}
}
