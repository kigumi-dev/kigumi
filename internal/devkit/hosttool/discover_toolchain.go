package hosttool

import (
	"os"
	"os/exec"
	"strings"
)

// Lookup finds an executable by name; the caller supplies the search (internal/cli passes its PATH).
type Lookup func(name string) (string, error)

// Toolchain records which optional build tools this host has on PATH.
type Toolchain struct {
	Zig   bool `json:"zig"`
	Clang bool `json:"clang"`
}

// DiscoverToolchain probes look the same way internal/toolchain/driver picks a C
// compiler (zig cc, falling back to clang), without importing it. KIGUMI_TEST_NO_CC
// is the hermetic CI job's gate: it must see the no-toolchain path even when the
// runner image ships clang.
func DiscoverToolchain(look Lookup) Toolchain {
	if os.Getenv("KIGUMI_TEST_NO_CC") != "" {
		return Toolchain{}
	}
	_, zig := look("zig")
	_, clang := look("clang")
	return Toolchain{Zig: zig == nil, Clang: clang == nil}
}

// CCompiler names and versions whichever of zig or clang DiscoverToolchain found; ok is false with neither on PATH.
func CCompiler(look Lookup) (name, version string, ok bool) {
	switch t := DiscoverToolchain(look); {
	case t.Zig:
		p, _ := look("zig")
		return "zig", runVersion(p, "version"), true
	case t.Clang:
		p, _ := look("clang")
		return "clang", runVersion(p, "--version"), true
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
