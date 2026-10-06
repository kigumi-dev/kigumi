package fetch

import (
	"fmt"
	"runtime"
)

// zigArch and zigOS translate Go's runtime.GOARCH/GOOS into the names
// ziglang.org's download index keys releases by ("<arch>-<os>").
var zigArch = map[string]string{
	"amd64": "x86_64", "arm64": "aarch64", "386": "x86", "arm": "arm",
	"riscv64": "riscv64", "ppc64le": "powerpc64le", "s390x": "s390x", "loong64": "loongarch64",
}

var zigOS = map[string]string{
	"linux": "linux", "darwin": "macos", "windows": "windows",
	"freebsd": "freebsd", "netbsd": "netbsd", "openbsd": "openbsd",
}

// HostTriple is the index key for the running host, such as "x86_64-linux".
func HostTriple() (string, error) {
	return Triple(runtime.GOOS, runtime.GOARCH)
}

// Triple is the index key for one os/arch pair, in Go's runtime.GOOS/GOARCH spelling.
func Triple(goos, goarch string) (string, error) {
	arch, ok := zigArch[goarch]
	if !ok {
		return "", fmt.Errorf("no known zig release arch for GOARCH %q", goarch)
	}
	os, ok := zigOS[goos]
	if !ok {
		return "", fmt.Errorf("no known zig release os for GOOS %q", goos)
	}
	return arch + "-" + os, nil
}
