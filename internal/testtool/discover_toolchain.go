package testtool

import "os/exec"

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
