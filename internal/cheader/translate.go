package cheader

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"kigumi/internal/zigtool"
)

// Runner executes `zig translate-c` and returns its stdout; a test can substitute
// a counting fake to check that a cache hit never invokes it.
type Runner func(zigPath, target, headerPath string) ([]byte, error)

// RunZig is the default Runner: `zig translate-c -lc [-target T] header.h`; freestanding
// and UEFI have no libc sysroot, so headers beyond the compiler's built-in set fail here too.
func RunZig(zigPath, target, headerPath string) ([]byte, error) {
	args := []string{"translate-c", "-lc"}
	if target != "" {
		args = append(args, "-target", target)
	}
	args = append(args, headerPath)
	cmd := exec.Command(zigPath, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zig translate-c %s: %w\n%s", headerPath, err, errOut.String())
	}
	return out.Bytes(), nil
}

// zigVersionString is informational only, unlike zigtool.Fingerprint: never
// part of the cache key, so a lookup never depends on it succeeding.
func zigVersionString(zigPath string) string {
	v, err := zigtool.Version(zigPath)
	if err != nil {
		return ""
	}
	return v
}

// translateBoth runs translate-c on header and, for the baseline Generate
// diffs against, on an empty file with the same flags.
func translateBoth(run Runner, zigPath, target, header string) (full, baseline string, err error) {
	fullOut, err := run(zigPath, target, header)
	if err != nil {
		return "", "", err
	}
	dir, err := os.MkdirTemp("", "kigumi-cheader")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(dir)
	empty := filepath.Join(dir, "empty.h")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		return "", "", err
	}
	baseOut, err := run(zigPath, target, empty)
	if err != nil {
		return "", "", err
	}
	return string(fullOut), string(baseOut), nil
}
