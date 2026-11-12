package driver_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

func TestToolCapabilitiesNamesToolchainUse(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("a build toolchain names the C compiler, and this host has none")
	}
	root, out := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"tc3\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "hello.c"), []byte("int main(void) { return 0; }\n"), 0o644)
	program := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n" +
		"    let cc = b.toolchain()\n" +
		"    let src = b.file(\"hello.c\")?\n" +
		"    let o = b.runTool(cc, \"cc\", Array.of(\"$in:0\", \"-o\", \"$out:0\"), Array.of(src), Array.of(\"hello\"), Array.empty[build.Secret]())?\n" +
		"}\n"
	os.WriteFile(filepath.Join(root, "build.kg"), []byte(program), 0o644)
	std, _ := filepath.Abs("../../../std")
	p, _, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	g, err := p.Run(buildconfig.HostTarget(), nil, out)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := driver.ResolveGraph(testCompiler(), g, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := driver.ToolCapabilities(rg.Graph)
	want := []string{"build framework toolchain", "build tool toolchain.cc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if s := (driver.Summary{Capabilities: got}).Render(); !strings.Contains(s, "  capabilities: build framework toolchain, build tool toolchain.cc\n") {
		t.Fatalf("summary does not list the toolchain:\n%s", s)
	}
}
