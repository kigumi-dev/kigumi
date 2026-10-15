package driver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func planModule(t *testing.T, root string, targets ...string) (string, driver.PlanStats) {
	t.Helper()
	build := "import build from std/build\n\nfn configure(b: build.Builder) -> Unit! {\n"
	for i, tr := range targets {
		build += "    let a" + string(rune('a'+i)) + " = b.executable(\"app" + string(rune('a'+i)) + "\", \"cmd/app\", b.target(\"" + tr + "\"))?\n"
	}
	build += "}\n"
	if err := os.WriteFile(filepath.Join(root, "build.kg"), []byte(build), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load: %v %v", ok, err)
	}
	g, err := p.Run(driver.HostTarget(), nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rg, err := driver.ResolveGraph(driver.CompilerOptions{}, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	return driver.PlanJSONWithStats(rg, p)
}

func planRoot(t *testing.T, app string) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "cmd", "app"), 0o755)
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"plancache\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "cmd", "app", "main.kg"), []byte(app), 0o644)
	return root
}

const planEqApp = `type Point = {
    pub x Int
%s}

fn same(a: Point, b: Point) -> Bool {
    a == b
}

print "${same(Point { x: 1%s }, Point { x: 1%s })}"
`

func eqApp(extraField, extraInit string) string {
	return fmt.Sprintf(planEqApp, extraField, extraInit, extraInit)
}

func TestPlanReadsDeriveCacheOnSecondRun(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := planRoot(t, eqApp("", ""))
	coldPlan, first := planModule(t, root, "x86_64-linux-musl")
	if first.DeriveMisses == 0 || first.DeriveHits != 0 {
		t.Fatalf("first plan = %+v, want derive misses only", first)
	}
	warmPlan, second := planModule(t, root, "x86_64-linux-musl")
	if second.DeriveHits != first.DeriveMisses || second.DeriveMisses != 0 {
		t.Fatalf("second plan = %+v, want every derive from the cache (first %+v)", second, first)
	}
	if coldPlan != warmPlan {
		t.Errorf("cached plan differs from the cold plan:\n%s\n---\n%s", coldPlan, warmPlan)
	}
}

func TestPlanDeriveCacheMissesChangedSchema(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := planRoot(t, eqApp("", ""))
	planModule(t, root, "x86_64-linux-musl")
	os.WriteFile(filepath.Join(root, "cmd", "app", "main.kg"), []byte(eqApp("    pub y Int\n", ", y: 2")), 0o644)
	_, changed := planModule(t, root, "x86_64-linux-musl")
	if changed.DeriveMisses == 0 {
		t.Fatalf("plan after a schema change = %+v, want a derive miss", changed)
	}
	_, again := planModule(t, root, "x86_64-linux-musl")
	if again.DeriveMisses != 0 || again.DeriveHits == 0 {
		t.Fatalf("plan repeated after the change = %+v, want all hits", again)
	}
}

func TestPlanSharesProvisionAcrossPOSIXOSes(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := planRoot(t, "print \"hi\"\n")
	_, stats := planModule(t, root, "x86_64-linux-musl", "x86_64-macos", "x86_64-windows-gnu")
	if stats.Provisions != 2 || stats.ProvisionReused != 1 {
		t.Errorf("plan stats = %+v, want 2 provision tables (posix, windows) and 1 reuse (macos from linux)", stats)
	}
}
