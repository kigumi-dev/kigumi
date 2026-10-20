package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func buildWithTarget(t *testing.T, target driver.Target, src string, opts driver.BuildOptions) (out string) {
	t.Helper()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"side\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	out = filepath.Join(root, "out")
	opts.Target = target
	var stderr bytes.Buffer
	if ok, err := driver.BuildWith(m, out, &stderr, opts); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, stderr.String())
	}
	return out
}

func requireReason(t *testing.T, out, capability, op string) {
	t.Helper()
	fp, ok, err := driver.ReadFingerprintSidecar(out)
	if err != nil || !ok {
		t.Fatalf("no provenance sidecar next to %s: ok=%v err=%v", out, ok, err)
	}
	for _, r := range fp.CapabilityReasons {
		if r.Capability == capability && r.Op == op && r.RootKind == "entry" && r.Provided && len(r.Path) > 0 {
			return
		}
	}
	t.Fatalf("sidecar reasons %+v lack a provided %s reached through %s from the entry", fp.CapabilityReasons, capability, op)
}

func TestSidecarRecordsCapabilityReasonsForHostedBuild(t *testing.T) {
	out := buildWithTarget(t, driver.HostTarget(), hostArgsSrc, driver.BuildOptions{})
	requireReason(t, out, "host.args", "HOST_ARGS")
}

func TestSidecarRecordsCapabilityReasonsForReproducibleBuild(t *testing.T) {
	out := buildWithTarget(t, driver.HostTarget(), hostArgsSrc, driver.BuildOptions{Reproducible: true})
	requireReason(t, out, "host.args", "HOST_ARGS")
}

func TestSidecarRecordsCapabilityReasonsForBareArchive(t *testing.T) {
	const src = "import ffi from std/ffi\nimport {Arena} from std/alloc\n\n" +
		"let h = Arena.create().unwrap()\nallocator h.scope() {\n    let s = \"hi\"\n    let _ = ffi.CString.new(&s).unwrap()\n}\n"
	out := buildWithTarget(t, mustTarget(t, "x86_64-freestanding", ""), src, driver.BuildOptions{})
	requireReason(t, out, "ffi.cstring", "CSTRING_NEW")
}
