package lint_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/lang/diag"
)

func stdRoot(t *testing.T) string {
	t.Helper()
	std, err := filepath.Abs("../../../../std")
	if err != nil {
		t.Fatal(err)
	}
	return std
}

// module writes one file into a fresh module directory and returns the root.
func module(t *testing.T, name, src string) string {
	t.Helper()
	t.Setenv("KIGUMI_PKG_CACHE", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func execute(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return clitest.Execute(t, append([]string{"--std", stdRoot(t)}, args...)...)
}

func read(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "main.kg"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const unusedImport = "import math from std/math\n\nfn main() -> Void {}\n"

// A warning is the ordinary lint result: human text on stderr and exit 0.
func TestLintWarningExitsZero(t *testing.T) {
	dir := module(t, "main.kg", unusedImport)
	code, out, errOut := execute(t, "lint", dir)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr %q", code, errOut)
	}
	if out != "" {
		t.Errorf("human diagnostics belong on stderr, stdout = %q", out)
	}
	if !strings.Contains(errOut, "warning[W100]: unused import `math`") {
		t.Errorf("missing W100: %q", errOut)
	}
}

// --deny-warnings turns a surviving warning into a failure.
func TestLintDenyWarnings(t *testing.T) {
	dir := module(t, "main.kg", unusedImport)
	if code, _, errOut := execute(t, "lint", "--deny-warnings", dir); code != 1 {
		t.Fatalf("exit = %d, want 1; stderr %q", code, errOut)
	}
}

// A compile error always fails lint, and the machine report carries it.
func TestLintErrorExitsOne(t *testing.T) {
	dir := module(t, "main.kg", "fn main() -> Void {\n    let x: String = 1\n}\n")
	code, _, errOut := execute(t, "lint", dir)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr %q", code, errOut)
	}
	if !strings.Contains(errOut, "error[E400]") {
		t.Errorf("missing E400: %q", errOut)
	}
}

// --format json is the shared stable schema: warnings survive into it and the
// run still exits 0.
func TestLintJSON(t *testing.T) {
	dir := module(t, "main.kg", unusedImport)
	code, out, errOut := execute(t, "lint", "--format", "json", dir)
	if code != 0 || errOut != "" {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
	var rep diag.MachineReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stdout is not the machine report: %v\n%s", err, out)
	}
	if len(rep.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v", rep.Diagnostics)
	}
	d := rep.Diagnostics[0]
	if d.Severity != "warning" || d.Code != "W100" || d.File != "main.kg" {
		t.Errorf("diagnostic = %+v", d)
	}
}

// The standard profile's existing checks report through lint: `Option[Bool] ||`
// keeps its W601 code.
func TestLintOptionBoolFallback(t *testing.T) {
	src := "fn flag() -> Bool? {\n    None\n}\n\nfn main() -> Void {\n    let f = flag() || true\n    print \"${f}\"\n}\n"
	dir := module(t, "main.kg", src)
	code, _, errOut := execute(t, "lint", dir)
	if code != 0 || !strings.Contains(errOut, "warning[W601]") {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
}

// A hand-written equals with a compiler-derived hash is an Eq/Hash mismatch:
// the derived half cannot know the author's rule. The reverse pair reports too.
func TestLintEqHashMismatch(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{
			name: "manual equals, derived hash",
			src:  "@impl(Hash)\ntype Key = { n Int }\n\npure fn Key.equals(self, other: &Key) -> Bool {\n    self.n == other.n\n}\n\nfn main() -> Void {}\n",
			want: "`Key` defines `equals` but `hash` is derived",
		},
		{
			name: "manual hash, derived equals",
			src:  "@impl(Eq)\ntype Key = { n Int }\n\npure fn Key.hash(self) -> u64 {\n    1\n}\n\nfn main() -> Void {}\n",
			want: "`Key` defines `hash` but `equals` is derived",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := module(t, "main.kg", tc.src)
			code, _, errOut := execute(t, "lint", dir)
			if code != 0 {
				t.Fatalf("exit = %d, stderr %q", code, errOut)
			}
			if !strings.Contains(errOut, "warning[W602]") || !strings.Contains(errOut, tc.want) {
				t.Errorf("missing W602 %q: %q", tc.want, errOut)
			}
		})
	}
}

// The parser's duplicate-modifier warning carries its stable code.
func TestLintRedundantModifier(t *testing.T) {
	dir := module(t, "main.kg", "pure pure fn f() -> Void {}\n\nfn main() -> Void { f() }\n")
	code, _, errOut := execute(t, "lint", dir)
	if code != 0 || !strings.Contains(errOut, "warning[W201]: duplicate modifier") {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
}

// Repeating the same visibility is redundant, not conflicting; --fix keeps one.
func TestLintDuplicateVisibility(t *testing.T) {
	src := "pub pub fn f() -> Void {}\n\nfn main() -> Void { f() }\n"
	dir := module(t, "main.kg", src)
	code, _, errOut := execute(t, "lint", dir)
	if code != 0 || !strings.Contains(errOut, "warning[W201]: duplicate visibility") {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
	if code, _, errOut := execute(t, "lint", "--fix", dir); code != 0 {
		t.Fatalf("fix exit = %d, stderr %q", code, errOut)
	}
	got := read(t, dir)
	if strings.Contains(got, "pub pub") || !strings.Contains(got, "pub fn f()") {
		t.Errorf("duplicate visibility survived --fix:\n%s", got)
	}
}

// --fix removes an unused import, then reports the clean module and exits 0.
func TestLintFixUnusedImport(t *testing.T) {
	dir := module(t, "main.kg", unusedImport)
	code, _, errOut := execute(t, "lint", "--fix", dir)
	if code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
	if got := read(t, dir); strings.Contains(got, "import") {
		t.Errorf("unused import survived --fix:\n%s", got)
	}
	if errOut != "" {
		t.Errorf("the fixed run must be clean, stderr %q", errOut)
	}
}

// --fix folds a duplicate modifier, keeping one spelling.
func TestLintFixRedundantModifier(t *testing.T) {
	dir := module(t, "main.kg", "pure pure fn f() -> Void {}\n\nfn main() -> Void { f() }\n")
	if code, _, errOut := execute(t, "lint", "--fix", dir); code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
	got := read(t, dir)
	if strings.Contains(got, "pure pure") || !strings.Contains(got, "pure fn f()") {
		t.Errorf("duplicate modifier survived --fix:\n%s", got)
	}
}

// --fix drops the redundant `pub` on an interface requirement.
func TestLintFixRedundantPub(t *testing.T) {
	dir := module(t, "main.kg", "interface I {\n    pub fn f(self) -> Int\n}\n\nfn main() -> Void {}\n")
	if code, _, errOut := execute(t, "lint", "--fix", dir); code != 0 {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
	got := read(t, dir)
	if strings.Contains(got, "pub fn") || !strings.Contains(got, "fn f(self)") {
		t.Errorf("redundant pub survived --fix:\n%s", got)
	}
}

// --fix must not rewrite a tree it could not parse; the diagnostic still
// decides the exit code.
func TestLintFixSkipsUnparsedModule(t *testing.T) {
	src := "pure pure fn f( -> Void {}\n"
	dir := module(t, "main.kg", src)
	code, _, errOut := execute(t, "lint", "--fix", dir)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr %q", code, errOut)
	}
	if got := read(t, dir); got != src {
		t.Errorf("--fix changed an unparsed file:\n%s", got)
	}
	if !strings.Contains(errOut, "not applying fixes") {
		t.Errorf("missing skip notice: %q", errOut)
	}
}
