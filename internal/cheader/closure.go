package cheader

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DepRunner reports the header's full transitive #include closure; tests
// substitute a fake instead of invoking zig.
type DepRunner func(zigPath, target, headerPath string) ([]string, error)

// RunZigDeps is the default DepRunner: translate-c's own frontend (Aro)
// does not report files read, so this asks `zig cc -M` (clang) instead.
func RunZigDeps(zigPath, target, headerPath string) ([]string, error) {
	args := []string{"cc", "-M", "-x", "c-header"}
	if target != "" {
		args = append(args, "-target", target)
	}
	args = append(args, headerPath)
	cmd := exec.Command(zigPath, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zig cc -M %s: %w\n%s", headerPath, err, errOut.String())
	}
	return parseMakeDeps(out.String()), nil
}

// parseMakeDeps reads a `cc -M` Makefile rule, dropping its own target
// (the would-be .o) and unescaping `\ ` and `\\` the way make does.
func parseMakeDeps(out string) []string {
	joined := strings.ReplaceAll(out, "\\\n", " ")
	_, rhs, ok := strings.Cut(joined, ":")
	if !ok {
		return nil
	}
	var deps []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			deps = append(deps, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(rhs); i++ {
		switch c := rhs[i]; {
		case c == '\\' && i+1 < len(rhs) && (rhs[i+1] == ' ' || rhs[i+1] == '\\'):
			cur.WriteByte(rhs[i+1])
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return deps
}
