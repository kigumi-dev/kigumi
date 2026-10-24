package driver

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStripELFDebugKeepsSymbols(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs an ELF test binary")
	}
	zig, err := exec.LookPath("zig")
	if err != nil {
		t.Skip("needs zig")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.c")
	if err := os.WriteFile(src, []byte("#include <stdio.h>\nint main(void) { puts(\"hi\"); return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "exe")
	cmd := exec.Command(zig, "cc", "-target", "x86_64-linux-musl", "-static", "-O1", src, "-o", path)
	cmd.Env = append(os.Environ(), "ZIG_LOCAL_CACHE_DIR="+filepath.Join(dir, "cache"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	img, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before := openELF(t, path)
	wantSyms, _ := before.Symbols()
	hasDebug := false
	for _, s := range before.Sections {
		hasDebug = hasDebug || strings.HasPrefix(s.Name, ".debug_")
	}
	before.Close()
	if !hasDebug || len(wantSyms) == 0 {
		t.Skip("test binary has no DWARF or no symbols")
	}

	if err := stripELFDebug(path); err != nil {
		t.Fatal(err)
	}
	after := openELF(t, path)
	defer after.Close()
	for _, s := range after.Sections {
		if strings.HasPrefix(s.Name, ".debug_") {
			t.Errorf("%s survived", s.Name)
		}
	}
	gotSyms, err := after.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	if len(gotSyms) != len(wantSyms) || gotSyms[len(gotSyms)-1] != wantSyms[len(wantSyms)-1] {
		t.Errorf("symbols changed: %d -> %d", len(wantSyms), len(gotSyms))
	}
	if info, _ := os.Stat(path); info.Size() >= int64(len(img)) {
		t.Errorf("file did not shrink: %d -> %d", len(img), info.Size())
	}
	if out, err := exec.Command(path).Output(); err != nil || string(out) != "hi\n" {
		t.Errorf("stripped executable: %q, %v", out, err)
	}
	for _, p := range after.Progs {
		if p.Type == elf.PT_LOAD {
			if _, err := p.Open().Read(make([]byte, 1)); err != nil {
				t.Errorf("segment unreadable: %v", err)
			}
		}
	}
}

func openELF(t *testing.T, path string) *elf.File {
	t.Helper()
	f, err := elf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
