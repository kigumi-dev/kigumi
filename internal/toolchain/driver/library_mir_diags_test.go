package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A library has no entry to reach its functions from, yet `kigumi check` still lowers every body of it and reports their MIR-level diagnostics.
func TestLibraryCheckKeepsMIRDiagnostics(t *testing.T) {
	cases := []struct {
		fixture, allocator, test string
		codes                    []string
	}{
		{"effects_noalloc_entry", "", "", []string{"E1840", "E1841", "E1610", "E907"}},
		{"effects_noalloc_entry_witness", "", "", []string{"E1570"}},
		{"capability_allocator_none_reached_cstring", "none", "", nil},
		{"capability_allocator_none_reached_cstring", "none", probeTest, []string{"E1370"}},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			lib := libraryOf(t, "../../../testdata/sem/"+c.fixture+".txtar")
			check := func(lowerAll bool) string {
				root := t.TempDir()
				os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"lib\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
				os.MkdirAll(filepath.Join(root, "lib"), 0o755)
				os.WriteFile(filepath.Join(root, "lib", "lib.kg"), []byte(lib), 0o644)
				if c.test != "" {
					os.WriteFile(filepath.Join(root, "lib", "lib_test.kg"), []byte(c.test), 0o644)
				}
				target := driver.HostTarget()
				target.Allocator = c.allocator
				m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Target: target, Test: c.test != ""})
				if err != nil {
					t.Fatal(err)
				}
				if lowerAll {
					driver.LowerAll(m)
				}
				res, err := driver.Check(m)
				if err != nil {
					t.Fatal(err)
				}
				return res.Render()
			}
			got, all := check(false), check(true)
			if got != all {
				t.Errorf("check differs from lowering every body\n--- check ---\n%s--- every body ---\n%s", got, all)
			}
			for _, code := range c.codes {
				if !strings.Contains(got, "error["+code+"]") {
					t.Errorf("missing %s:\n%s", code, got)
				}
			}
		})
	}
}

const probeTest = `import {AllocatorHandle} from std/alloc

test "probe" {
    let h: AllocatorHandle? = None
    if h is Some(handle) {
        let r = probe(handle)
    }
}
`

// libraryOf is a fixture's src with its `fn main` renamed, so nothing is an entry, and no entry-only `print`.
func libraryOf(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	_, src, _ = strings.Cut(src, "-- src --\n")
	src, _, _ = strings.Cut(src, "\n-- ")
	if !strings.Contains(src, "\nfn main(") {
		t.Fatalf("%s: no fn main", path)
	}
	src = strings.ReplaceAll(src, "\n    print ", "\n    let _ = ")
	return strings.Replace(src, "\nfn main(", "\nfn demo(", 1)
}
