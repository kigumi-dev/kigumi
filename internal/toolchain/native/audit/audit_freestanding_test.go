package audit

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
)

func freestandingArchive(t *testing.T, src string) (zig, dir string) {
	t.Helper()
	zig, err := exec.LookPath("zig")
	if err != nil {
		t.Skip("needs zig")
	}
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.c"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"cc", "-target", "x86_64-freestanding", "-O2", "-c", "a.c", "-o", "a.o"},
		{"ar", "rcs", "lib.a", "a.o"},
	} {
		cmd := exec.Command(zig, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "ZIG_LOCAL_CACHE_DIR="+filepath.Join(dir, "cache"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("zig %v: %v\n%s", args, err, out)
		}
	}
	return zig, dir
}

func TestArchiveCopiesTheAllowedSymbols(t *testing.T) {
	zig, dir := freestandingArchive(t, "extern int hal_read(void);\nextern int board_init(void);\nint run(void) { return hal_read() + board_init(); }\n")
	o := buildconfig.CompilerOptions{Cwd: dir}
	allowed := map[string]bool{"hal_read": true}
	before := maps.Clone(allowed)
	if err := Archive(o, []string{zig, "ar"}, "lib.a", "out.a", allowed, []string{"board_init"}); err != nil {
		t.Fatalf("an allowed symbol and an embedder symbol must pass: %v", err)
	}
	if !maps.Equal(allowed, before) {
		t.Fatalf("Archive changed the caller's allowed map: %v", allowed)
	}
	err := Archive(o, []string{zig, "ar"}, "lib.a", "out.a", allowed, nil)
	if err == nil || !strings.Contains(err.Error(), `"board_init"`) || strings.Contains(err.Error(), "hal_read") || !strings.Contains(err.Error(), "out.a") {
		t.Fatalf("without the embedder record only board_init may be missing, quoted under out.a, got %v", err)
	}
}
