package driver_test

import (
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

func TestLowerCheckedRootRejectsPublishedResult(t *testing.T) {
	t.Parallel()
	m := editorModule(t, editorBorrowSrc)
	res, err := driver.CheckOnly(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() || !res.Sealed() {
		t.Fatalf("expected a published semantic result before borrow checking:\n%s", res.Render())
	}
	entities := len(res.Entities)
	a, err := m.LowerCheckedRoot(res, 0)
	if err == nil || !strings.Contains(err.Error(), "published") || a != nil {
		t.Fatalf("published input must be rejected before lowering: analysis=%v err=%v", a != nil, err)
	}
	if len(res.Entities) != entities || res.HasErrors() || res.Snapshot().HasErrors() {
		t.Fatal("lowering changed the published result")
	}
}

func TestBuildProgramRunKeepsPublishedResult(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeAllocatorFixture(t, root)
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load build program: ok=%v err=%v", ok, err)
	}
	if !p.Res.Sealed() {
		t.Fatal("loading did not publish the build program")
	}
	entities, fns := len(p.Res.Entities), len(p.Res.Fns)
	snapshot := p.Res.Snapshot()
	for _, allocator := range []string{"general", "none"} {
		target := buildconfig.HostTarget()
		target.Allocator = allocator
		g, err := p.Run(target, nil, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(g.Artifacts) != 1 || g.Artifacts[0].Target.Allocator != allocator {
			t.Fatalf("configure lost the requested allocator: %+v", g.Artifacts)
		}
		if len(p.Res.Entities) != entities || len(p.Res.Fns) != fns || p.Res.Snapshot() != snapshot {
			t.Fatal("running configure changed the published analysis")
		}
	}
}

func TestCheckLazyRootPreservesDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, src, code string }{
		{"borrow", editorBorrowSrc, "E2198"},
		{"noalloc", deferredBoxSrc, "E1570"},
	} {
		for _, retry := range []bool{false, true} {
			name := tc.name + "/lazy"
			if retry {
				name = tc.name + "/eager-retry"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				m := editorModule(t, tc.src)
				if retry {
					for _, path := range m.Order {
						if p := m.Packages[path]; p.Std {
							p.Files = append(p.Files, syntax.Parse(token.NewFile("retry.kg", []byte("pub interface Retry { pub fn retry(self) -> Int }\n"))))
							break
						}
					}
				}
				res, err := driver.CheckLazyRoot(m)
				if err != nil {
					t.Fatal(err)
				}
				if res.HasErrors() || res.Sealed() {
					t.Fatalf("expected an unpublished semantic result:\n%s", res.Render())
				}
				if retry && m.UnitStats().LazyRetries != 1 {
					t.Fatal("the std warning did not trigger the eager retry")
				}
				a, err := m.LowerCheckedRoot(res, 0)
				if err != nil {
					t.Fatal(err)
				}
				if !a.Snapshot().HasErrors() || !strings.Contains(a.Snapshot().Render(), "error["+tc.code+"]") {
					t.Fatalf("lowering lost the %s diagnostic:\n%s", tc.code, a.Res.Render())
				}
			})
		}
	}
}

func TestBodyErrorKeepsProgramDiagnostics(t *testing.T) {
	t.Parallel()
	m := editorModule(t, editorBorrowSrc+`
type View['a] = { pub value &'a String }
type Sink['a] = { pub values &'a mut Array[View['a]] }
pure fn retain['a](value: &'a String, sink: Sink['a]) -> View['a] {
    sink.values.push(View { value: value })
    View { value: value }
}
fn erase['a](sink: Sink['a]) {
    let f: fn(&String, Sink['a]) -> View['a] = retain
}
`)
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"E2198", "E2297"} {
		if count := strings.Count(res.Render(), "error["+code+"]"); count != 1 {
			t.Errorf("%s reported %d times, want once:\n%s", code, count, res.Render())
		}
	}
}
