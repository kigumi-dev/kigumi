package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/mir"
)

// The thread profile is what makes refcount updates atomic, so a pool-free program must not link it in.
func TestThreadProfileFollowsReachability(t *testing.T) {
	t.Parallel()
	single := analyzeFixture(t, "async_spawn_order.txtar")
	if mir.UsesThreads(single) {
		t.Fatal("a current-thread executor program asked for the thread runtime")
	}
	pooled := analyzeFixture(t, "async_thread_spawn.txtar")
	if !mir.UsesThreads(pooled) {
		t.Fatal("a thread pool program did not ask for the thread runtime")
	}
}

// indirectPoolSrc names the pool constructor as a value and calls it through
// that value, so no call edge points at the std function.
const indirectPoolSrc = `import task from std/task

async fn leaf(n: Int) -> Int {
    n
}

fn poolCtor() -> fn(usize) -> task.Executor {
    task.threadPoolExecutor
}

let pool = poolCtor()(2)
print "${pool.runToCompletion(leaf(41))}"
`

func indirectPoolModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := "Module {\n    name: \"indirectpool\"\n    kigumi: \"0.1\"\n}\n"
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(indirectPoolSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func indirectPoolAnalysis(t *testing.T, target driver.Target) *driver.Analysis {
	t.Helper()
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(indirectPoolModule(t), driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// Naming the constructor already links pthread via mir.UsesThreads, so the
// target and profile gates must reject a bare mention, not only a call.
func TestIndirectPoolMentionUsesThreads(t *testing.T) {
	t.Parallel()
	a := indirectPoolAnalysis(t, driver.Target{})
	if out := a.Res.Render(); out != "" {
		t.Fatalf("a hosted build of an indirect pool should check clean:\n%s", out)
	}
	if !mir.UsesThreads(a.Prog) {
		t.Fatal("a mention of the pool constructor did not ask for the thread runtime")
	}
}

func TestIndirectPoolRejectedByProfile(t *testing.T) {
	t.Parallel()
	for _, profile := range []string{"isr", "alloc-free-executor", "single-thread-event-loop"} {
		t.Run(profile, func(t *testing.T) {
			t.Parallel()
			target := driver.HostTarget()
			target.BlockingExecutor = profile
			got := indirectPoolAnalysis(t, target).Res.Render()
			if !strings.Contains(got, "E1461") || !strings.Contains(got, profile) {
				t.Fatalf("profile %s should reject an indirect pool with E1461:\n%s", profile, got)
			}
		})
	}
}

func TestIndirectPoolRejectedByTarget(t *testing.T) {
	t.Parallel()
	target, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := indirectPoolAnalysis(t, target).Res.Render(); !strings.Contains(got, "E1460") {
		t.Fatalf("a freestanding target should reject an indirect pool with E1460:\n%s", got)
	}
}
