package arch_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestReleaseTargetsCrossCompile builds the release binary for every target in
// the release list except the host's own. This is the gate that lets
// .github/scripts/release/targets.txt gain a line: a target that does not
// cross-compile here must not reach a release matrix.
func TestReleaseTargetsCrossCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-builds are skipped with -short")
	}
	targetsPath := filepath.Join("..", "..", ".github", "scripts", "release", "targets.txt")
	data, err := os.ReadFile(targetsPath)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..")
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		goos, goarch, ok := strings.Cut(line, "/")
		if !ok || goos == "" || goarch == "" {
			t.Fatalf("%s:%d: %q is not <os>/<arch>", targetsPath, i+1, line)
		}
		if goos == runtime.GOOS && goarch == runtime.GOARCH {
			continue
		}
		t.Run(goos+"_"+goarch, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "kigumi")
			if goos == "windows" {
				out += ".exe"
			}
			cmd := exec.Command("go", "build", "-trimpath", "-o", out, ".")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("GOOS=%s GOARCH=%s go build: %v\n%s", goos, goarch, err, output)
			}
		})
	}
}
