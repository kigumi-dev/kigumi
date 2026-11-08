package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/native/object"
)

// A --reproducible UEFI archive holds the plain build's objects with only StripCOFFDebug applied.
func TestReproducibleUEFIArchiveIsTheStrippedPlainBuild(t *testing.T) {
	requireZig(t)
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"bare\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-uefi", "")
	if err != nil {
		t.Fatal(err)
	}
	build := func(reproducible bool) []arMember {
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "bare.a")
		var stderr bytes.Buffer
		if ok, err := driver.BuildWith(m, out, &stderr, driver.BuildOptions{Target: target, Reproducible: reproducible}); err != nil || !ok {
			t.Fatalf("build (reproducible=%v): %v\n%s", reproducible, err, stderr.String())
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return arObjects(t, data)
	}
	plain, reproducible := build(false), build(true)
	if len(plain) == 0 || len(plain) != len(reproducible) {
		t.Fatalf("plain archive has %d objects, reproducible %d", len(plain), len(reproducible))
	}
	for i, p := range plain {
		path := filepath.Join(t.TempDir(), p.name)
		if err := os.WriteFile(path, p.data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := object.StripCOFFDebug(path); err != nil {
			t.Fatal(err)
		}
		stripped, _ := os.ReadFile(path)
		if r := reproducible[i]; r.name != p.name || !bytes.Equal(r.data, stripped) {
			t.Errorf("reproducible member %s (%d bytes) is not plain %s stripped (%d bytes)", r.name, len(r.data), p.name, len(stripped))
		}
	}
}

type arMember struct {
	name string
	data []byte
}

// arObjects lists the object members of a GNU ar archive, without its symbol and name tables.
func arObjects(t *testing.T, data []byte) []arMember {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		t.Fatal("not an ar archive")
	}
	var out []arMember
	for off := 8; off+60 <= len(data); {
		hdr := data[off : off+60]
		size, err := strconv.Atoi(strings.TrimSpace(string(hdr[48:58])))
		if err != nil || off+60+size > len(data) {
			t.Fatalf("bad ar member header at %d: %q", off, hdr)
		}
		name := strings.TrimSpace(string(hdr[:16]))
		if name != "/" && name != "//" {
			out = append(out, arMember{strings.TrimSuffix(name, "/"), data[off+60 : off+60+size]})
		}
		off += 60 + size + size%2
	}
	return out
}
