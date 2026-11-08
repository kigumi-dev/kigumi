package object

import (
	"bytes"
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const coffStripSrc = `static int counter = 3;
int table[4] = {1, 2, 3, 4};
static int helper(int x) { return x * counter + table[x & 3]; }
unsigned long long EfiMain(void *image, void *system_table) {
    (void)image; (void)system_table;
    return (unsigned long long)helper(2);
}
`

// A zig-built UEFI object keeps its code, data, relocations and symbols through the strip, drops the
// CodeView that names zig's temp object, and still links to the same image bytes.
func TestStripCOFFDebugKeepsTheLinkableObject(t *testing.T) {
	zig, err := exec.LookPath("zig")
	if err != nil {
		t.Skip("needs zig")
	}
	dir := t.TempDir()
	cache := filepath.Join(dir, "zig-cache")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(zig, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "ZIG_LOCAL_CACHE_DIR="+cache, "ZIG_GLOBAL_CACHE_DIR="+cache)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("zig %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "a.c"), []byte(coffStripSrc), 0o644)
	run("cc", "-target", "x86_64-uefi", "-g", "-ffreestanding", "-fno-stack-protector", "-fno-sanitize=all", "-c", "-o", "a.o", "a.c")
	obj := filepath.Join(dir, "a.o")
	before, err := os.ReadFile(obj)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(before, []byte(cache)) {
		t.Fatal("the object does not name zig's cache; the strip has nothing to prove")
	}
	if err := StripCOFFDebug(obj); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(obj)
	if bytes.Contains(after, []byte(cache)) {
		t.Error("the stripped object still names zig's cache")
	}
	assertSameLinkableParts(t, before, after)

	os.WriteFile(filepath.Join(dir, "orig.o"), before, 0o644)
	for _, name := range []string{"orig", "a"} {
		run("lld-link", "/subsystem:efi_application", "/entry:EfiMain", "/nodefaultlib", "/machine:x64", "/out:"+name+".efi", name+".o")
	}
	for _, sec := range []string{".text", ".data", ".rdata", ".pdata"} {
		if a, b := imageSection(t, filepath.Join(dir, "orig.efi"), sec), imageSection(t, filepath.Join(dir, "a.efi"), sec); !bytes.Equal(a, b) {
			t.Errorf("%s of the linked image differs after the strip", sec)
		}
	}
}

func assertSameLinkableParts(t *testing.T, before, after []byte) {
	t.Helper()
	b, a := openCOFF(t, before), openCOFF(t, after)
	if a.TimeDateStamp != 0 {
		t.Errorf("TimeDateStamp = %d, want 0", a.TimeDateStamp)
	}
	if len(a.Sections) != len(b.Sections) {
		t.Fatalf("%d sections, want %d", len(a.Sections), len(b.Sections))
	}
	debug := 0
	for i, bs := range b.Sections {
		as := a.Sections[i]
		if as.Name != bs.Name {
			t.Fatalf("section %d is %s, want %s", i, as.Name, bs.Name)
		}
		if strings.HasPrefix(bs.Name, ".debug") {
			debug++
			if as.Size != 0 || len(as.Relocs) != 0 {
				t.Errorf("%s keeps %d bytes and %d relocations", as.Name, as.Size, len(as.Relocs))
			}
			continue
		}
		bd, _ := bs.Data()
		ad, _ := as.Data()
		if !bytes.Equal(ad, bd) || !slices.Equal(as.Relocs, bs.Relocs) {
			t.Errorf("%s changed in the strip", bs.Name)
		}
	}
	if debug == 0 {
		t.Fatal("the object has no debug section")
	}
	if !slices.EqualFunc(a.Symbols, b.Symbols, func(x, y *pe.Symbol) bool { return *x == *y }) {
		t.Error("the symbol table changed in the strip")
	}
}

func openCOFF(t *testing.T, data []byte) *pe.File {
	t.Helper()
	f, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func imageSection(t *testing.T, path, name string) []byte {
	t.Helper()
	f, err := pe.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := f.Section(name)
	if s == nil {
		return nil
	}
	data, err := s.Data()
	if err != nil {
		t.Fatal(err)
	}
	return data
}
