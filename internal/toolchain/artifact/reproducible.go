package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/base/fsx"
)

// Reproduce builds twice, each in a fresh directory with its own compiler cache, and publishes out only when both outputs match byte for byte.
func Reproduce(out string, env []string, build func(out string, env []string) (bool, error)) (bool, error) {
	name := filepath.Base(out)
	var data [2][]byte
	var mode os.FileMode
	var sidecar []byte
	for i := range data {
		dir, err := os.MkdirTemp("", "kigumi-repro")
		if err != nil {
			return false, err
		}
		defer os.RemoveAll(dir)
		buildOut := filepath.Join(dir, name)
		ok, err := build(buildOut, isolatedCacheEnv(dir, env))
		if err != nil || !ok {
			return ok, err
		}
		st, err := os.Stat(buildOut)
		if err != nil {
			return false, err
		}
		if i == 0 {
			mode = st.Mode()
		}
		data[i], err = os.ReadFile(buildOut)
		if err != nil {
			return false, err
		}
		if i == 0 {
			sidecar, _ = os.ReadFile(FingerprintSidecarPath(buildOut))
		}
	}
	if off, equal := firstDiffOffset(data[0], data[1]); !equal {
		return false, fmt.Errorf("build is not reproducible: %s differs at byte offset %d (%d vs %d bytes)", name, off, len(data[0]), len(data[1]))
	}
	if sidecar != nil {
		// The sidecar goes down first: out never exists without the fingerprint that describes it.
		if err := fsx.WriteFileAtomic(FingerprintSidecarPath(out), sidecar, 0o644); err != nil {
			return false, err
		}
	}
	if err := WriteOutput(out, data[0], mode); err != nil {
		return false, err
	}
	if err := os.Chmod(out, mode); err != nil {
		return false, err
	}
	return true, nil
}

// firstDiffOffset is the offset of the first byte at which a and b differ;
// when one is a prefix of the other, that is the shorter one's length.
func firstDiffOffset(a, b []byte) (offset int, equal bool) {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i, false
		}
	}
	if len(a) != len(b) {
		return n, false
	}
	return 0, true
}

// isolatedCacheEnv points zig at dir's caches; base comes from the caller because no toolchain package reads the process environment.
func isolatedCacheEnv(dir string, base []string) []string {
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if strings.HasPrefix(kv, "ZIG_GLOBAL_CACHE_DIR=") || strings.HasPrefix(kv, "ZIG_LOCAL_CACHE_DIR=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"ZIG_GLOBAL_CACHE_DIR="+filepath.Join(dir, "zig-cache-global"),
		"ZIG_LOCAL_CACHE_DIR="+filepath.Join(dir, "zig-cache-local"))
}
