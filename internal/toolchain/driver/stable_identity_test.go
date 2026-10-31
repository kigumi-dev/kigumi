package driver_test

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

var stableFiles = map[string]string{
	"mod.kg": "Module {\n    name: \"stable\"\n    kigumi: \"0.1\"\n}\n",
	"main.kg": `import shapes from stable/lib/shapes
import util from stable/lib/util

fn show(x: Int) -> String { "int ${x}" }
static counter: Int = 3

fn apply[T](x: T, f: fn(T) -> T) -> T { f(x) }

let k = 5
print show(1)
print show("a")
print show(true)
print "${apply(k, (v: Int) => v + 1)}"
print "${util.twice(2)}"
print "${util.twice("x")}"
print "${shapes.area(shapes.Square { side: 3 })}"
print "${shapes.area(shapes.Rect { w: 2, h: 5 })}"
print "${counter}"
print "${pair(2)}"
print "${util.bump(1)}"
`,
	"lib/shapes/a.kg": `pub interface Shape {
    fn area(self) -> Int
}

pub type Square = {
    pub side Int
}

pub fn Square.area(self) -> Int { self.side * self.side }

pub fn area[S: Shape](s: S) -> Int { s.area() }
`,
	"lib/shapes/b.kg": `pub type Rect = {
    pub w Int
    pub h Int
}

pub fn Rect.area(self) -> Int { self.w * self.h }
`,
	"helpers.kg": `fn show(x: String) -> String { "str ${x}" }
fn show(x: Bool) -> String { "bool ${x}" }

fn pair(a: Int) -> Int {
    let f = (v: Int) => v + a
    let g = (v: Int) => v * a
    f(g(a))
}
`,
	"lib/util/a.kg": `pub fn twice(x: Int) -> Int { x * 2 }
`,
	"lib/util/b.kg": `pub fn twice(x: String) -> String { "${x}${x}" }

static hidden: Int = 7

pub fn bump(x: Int) -> Int {
    let f = (v: Int) => v + hidden
    f(x)
}
`,
}

func writeStable(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func stableOutputs(t *testing.T, root string, files map[string]string, reverse bool) (ir, mirText string) {
	t.Helper()
	writeStable(t, root, files)
	std, _ := filepath.Abs("../../../std")
	opts := driver.LoadOptions{Compiler: testCompiler(), StdRoot: std}
	if reverse {
		opts.FileOrder = slices.Reverse[[]string]
	}
	m, err := driver.LoadModule(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	return ir, a.Prog.Dump()
}

func firstTextDifference(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] != lb[i] {
			return fmt.Sprintf("line %d:\n  %s\n  %s", i+1, la[i], lb[i])
		}
	}
	return fmt.Sprintf("lengths %d and %d", len(la), len(lb))
}

func requireSameOutputs(t *testing.T, what, ir1, mir1, ir2, mir2 string) {
	t.Helper()
	if ir1 != ir2 {
		t.Errorf("%s: LLVM IR differs at %s", what, firstTextDifference(ir1, ir2))
	}
	if mir1 != mir2 {
		t.Errorf("%s: MIR dump differs at %s", what, firstTextDifference(mir1, mir2))
	}
}

func TestStableIdentityAcrossRootsAndEnumeration(t *testing.T) {
	r1, r2 := t.TempDir(), filepath.Join(t.TempDir(), "other", "root")
	ir1, mir1 := stableOutputs(t, r1, stableFiles, false)
	ir2, mir2 := stableOutputs(t, r2, stableFiles, true)
	requireSameOutputs(t, "another root and reversed enumeration", ir1, mir1, ir2, mir2)
	for _, ordinal := range ordinalSymbols.FindAllString(ir1+mir1, -1) {
		t.Errorf("a process-local number reached the output: %s", ordinal)
	}
}

// ordinalSymbols matches names built from an entity, type or overload index.
var ordinalSymbols = regexp.MustCompile(`kg\.static\.\d|\.type\.prim\d|[A-Za-z]#\d+["(]`)

// Moving a declaration to another file or position must not rename it.
func TestStableIdentityIgnoresDeclarationPlacement(t *testing.T) {
	moved := maps.Clone(stableFiles)
	moved["main.kg"], moved["helpers.kg"] = stableFiles["helpers.kg"], stableFiles["main.kg"]
	moved["lib/util/a.kg"], moved["lib/util/b.kg"] = stableFiles["lib/util/b.kg"], stableFiles["lib/util/a.kg"]
	ir1, mir1 := stableOutputs(t, t.TempDir(), stableFiles, false)
	ir2, _ := stableOutputs(t, t.TempDir(), moved, false)
	if !slices.Equal(definedSymbols(ir1), definedSymbols(ir2)) {
		t.Errorf("symbols change when declarations move between files:\n%v\n%v", definedSymbols(ir1), definedSymbols(ir2))
	}
	if strings.Contains(mir1, "#0") || strings.Contains(ir1, "#0\"") {
		t.Error("an overload is still numbered by position")
	}
}

func definedSymbols(ir string) []string {
	var out []string
	for _, line := range strings.Split(ir, "\n") {
		if strings.HasPrefix(line, "define ") {
			if at := strings.Index(line, "@"); at >= 0 {
				out = append(out, line[at:strings.LastIndex(line, "(")])
			}
		}
	}
	slices.Sort(out)
	return out
}

func TestStableIdentityAcrossProcesses(t *testing.T) {
	if dir := os.Getenv("KIGUMI_STABLE_CHILD"); dir != "" {
		ir, mirText := stableOutputs(t, t.TempDir(), stableFiles, false)
		os.WriteFile(filepath.Join(dir, "ir"), []byte(ir), 0o644)
		os.WriteFile(filepath.Join(dir, "mir"), []byte(mirText), 0o644)
		return
	}
	out := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestStableIdentityAcrossProcesses$")
	cmd.Env = append(os.Environ(), "KIGUMI_STABLE_CHILD="+out)
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child: %v\n%s", err, msg)
	}
	ir1, mir1 := stableOutputs(t, t.TempDir(), stableFiles, false)
	ir2, _ := os.ReadFile(filepath.Join(out, "ir"))
	mir2, _ := os.ReadFile(filepath.Join(out, "mir"))
	requireSameOutputs(t, "another process", ir1, mir1, string(ir2), string(mir2))
}
