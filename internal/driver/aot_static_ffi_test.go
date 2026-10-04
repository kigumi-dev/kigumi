package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

// TestAotExportStaticReadByC: an `export(C)` static is a genuine C ABI
// object; a C shim reads and writes it directly by its linked symbol.
func TestAotExportStaticReadByC(t *testing.T) {
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	src := `export(C) {
    @linkName("plugin_version")
    pub static pluginVersion: u32 = 7

    @linkName("plugin_counter")
    static mut pluginCounter: u32 = 10
}

extern(C) {
    fn read_plugin_version() -> u32
    fn bump_plugin_counter() -> u32
}

let initial = unsafe { read_plugin_version() }
let bump1 = unsafe { bump_plugin_counter() }
let bump2 = unsafe { bump_plugin_counter() }
print "initial=${initial}"
print "bump1=${bump1}"
print "bump2=${bump2}"
`
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	helper := filepath.Join(root, "helper.c")
	cHelper := "" +
		"extern unsigned int plugin_version;\n" +
		"extern unsigned int plugin_counter;\n" +
		"unsigned int read_plugin_version(void) { return plugin_version; }\n" +
		"unsigned int bump_plugin_counter(void) { plugin_counter += 1; return plugin_counter; }\n"
	os.WriteFile(helper, []byte(cHelper), 0o644)
	t.Setenv("KIGUMI_CFLAGS", helper)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	want := "initial=7\nbump1=11\nbump2=12\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

// TestAotExternStaticFromC: an `extern(C)` static imports storage a C file
// defines; Kigumi reads it under `unsafe` without owning or initializing it.
func TestAotExternStaticFromC(t *testing.T) {
	if driver.CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	src := `extern(C) {
    static firmwareVersion: u32

    @linkName("vendor_counter")
    static mut vendorCounter: u32
}

fn bumpVendor() -> u32 {
    unsafe {
        vendorCounter = vendorCounter + 1
        vendorCounter
    }
}

let v = unsafe { firmwareVersion }
print "firmware=${v}"
print "vendor1=${bumpVendor()}"
print "vendor2=${bumpVendor()}"
`
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	helper := filepath.Join(root, "helper.c")
	cHelper := "" +
		"unsigned int firmwareVersion = 99;\n" +
		"unsigned int vendor_counter = 200;\n"
	os.WriteFile(helper, []byte(cHelper), 0o644)
	t.Setenv("KIGUMI_CFLAGS", helper)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	want := "firmware=99\nvendor1=201\nvendor2=202\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}
