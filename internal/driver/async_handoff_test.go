package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const handoffSrc = `import task from std/task

type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    let _ = self.id
}

async fn leaf(n: Int) -> Int {
    n
}

async fn pinned(n: Int) -> Int {
    let r = Res { id: n }
    let t = task.spawn(leaf(n))
    r.id + await t
}

async fn mobileBody(n: Int) -> Int {
    let t = task.spawn(leaf(n))
    n + await t
}

async fn root(pool: task.Executor) -> Int {
    let a = pool.handoff(mobileBody(1))
    let b = pool.handoff(PINNED)
    await a + await b
}

let pool = task.threadPoolExecutor(2)
let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(root(pool))}"
`

func handoffModule(t *testing.T, call string) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"handoff\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(strings.Replace(handoffSrc, "PINNED", call, 1)), 0o644)
	return root
}

func handoffDiagnostics(t *testing.T, call string) string {
	t.Helper()
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(handoffModule(t, call), driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	return a.Res.Render()
}

func TestHandoffRejectsExecutorBoundFuture(t *testing.T) {
	t.Parallel()
	got := handoffDiagnostics(t, "pinned(2)")
	if !strings.Contains(got, "E1462") || !strings.Contains(got, "`Res`") {
		t.Fatalf("handing off an executor-bound future should report E1462 naming Res:\n%s", got)
	}
	if strings.Count(got, "E1462") != 1 {
		t.Fatalf("want one diagnostic for the one bound handoff:\n%s", got)
	}
}

func TestHandoffAcceptsMigratableFuture(t *testing.T) {
	t.Parallel()
	if got := handoffDiagnostics(t, "mobileBody(2)"); got != "" {
		t.Fatalf("handing off a migratable future should check clean:\n%s", got)
	}
}
