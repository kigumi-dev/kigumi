package driver_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

var baseDescriptorRe = regexp.MustCompile(`@"\.type\.[^"]*\.(Ordering|Range|AllocError|Map)" = global`)

func irFor(t *testing.T, target driver.Target, src string) string {
	t.Helper()
	root := t.TempDir()
	writeBaseTypesModule(t, root, src)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	return ir
}

func TestEmitDataOmitsUnreferencedBaseTypes(t *testing.T) {
	t.Parallel()
	const plain = "print \"hi\"\n"
	for name, target := range map[string]driver.Target{
		"hosted":       driver.HostTarget(),
		"freestanding": mustTarget(t, "x86_64-freestanding", ""),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ir := irFor(t, target, plain)
			if found := baseDescriptorRe.FindAllString(ir, -1); len(found) != 0 {
				t.Errorf("a program using none of Ordering, Range, AllocError and Map emits %v", found)
			}
		})
	}
	used := irFor(t, driver.HostTarget(), "let r = 0..3\nprint \"${r.end}\"\nlet o = \"a\".compareTo(&\"b\")\nprint \"${o is Ordering.Less}\"\n")
	if found := baseDescriptorRe.FindAllStringSubmatch(used, -1); len(found) != 2 {
		t.Errorf("a program using Range and Ordering emits %v, want exactly those two descriptors", found)
	}
}

func writeBaseTypesModule(t *testing.T, root, src string) {
	t.Helper()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"basetypes\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
}
