package session_test

import (
	"testing"

	"kigumi/internal/cli/session"
	buildconfig "kigumi/internal/toolchain/config"
)

// TestCompilerOptionsHostIsHostTarget: the CLI's host facts are HostTarget's,
// not a second literal that can drift from it.
func TestCompilerOptionsHostIsHostTarget(t *testing.T) {
	t.Parallel()
	got := session.CompilerOptions().Host
	want := buildconfig.HostTarget()
	if got != want {
		t.Fatalf("CompilerOptions().Host = %+v, want HostTarget() %+v", got, want)
	}
	if got.Triple != "" || got.OS == "" || got.Arch == "" || got.Sys != "posix" {
		t.Fatalf("host quadruple = %+v, want empty triple, platform names and posix", got)
	}
}
