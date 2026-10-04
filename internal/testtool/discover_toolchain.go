package testtool

import (
	"os/exec"
	"strings"
)

// Toolchain records which optional build tools this host has on PATH.
type Toolchain struct {
	Zig   bool `json:"zig"`
	Clang bool `json:"clang"`
}

// DiscoverToolchain probes PATH the same way internal/driver picks a C
// compiler (zig cc, falling back to clang), without importing it.
func DiscoverToolchain() Toolchain {
	_, zig := exec.LookPath("zig")
	_, clang := exec.LookPath("clang")
	return Toolchain{Zig: zig == nil, Clang: clang == nil}
}

// CCompiler names and versions whichever of zig or clang DiscoverToolchain found; ok is false with neither on PATH.
func CCompiler() (name, version string, ok bool) {
	switch t := DiscoverToolchain(); {
	case t.Zig:
		return "zig", runVersion("zig", "version"), true
	case t.Clang:
		return "clang", runVersion("clang", "--version"), true
	default:
		return "", "", false
	}
}

func runVersion(name string, arg string) string {
	out, err := exec.Command(name, arg).Output()
	if err != nil {
		return "unknown"
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line)
}
