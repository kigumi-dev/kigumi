package vm_test

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
	"time"

	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

func buildAndRun(t *testing.T, src string, opts vm.Options) (stdout, stderr string, code int) {
	t.Helper()
	res := checkSource(t, src)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if prog.HasErrors() {
		t.Fatal("ownership errors")
	}
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}
	var out, errOut bytes.Buffer
	c, err := vm.Run(prog, opts, &out, &errOut)
	if err != nil {
		t.Fatalf("vm: %v", err)
	}
	return out.String(), errOut.String(), c
}

func goroutinesSettle(t *testing.T) int {
	t.Helper()
	last := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		runtime.GC()
		n := runtime.NumGoroutine()
		if n == last {
			return n
		}
		last = n
	}
	return last
}

func TestWorkerPanicCancelsSiblingsAndJoinsPool(t *testing.T) {
	before := goroutinesSettle(t)
	src := `import task from std/task

async fn boom() -> Int {
    None.expect("boom")
}

async fn spin(n: Int) -> Int {
    let mut i = 0
    for !task.cancelRequested() {
        await task.yieldNow()
        i = i + 1
    }
    n
}

async fn root() -> Int {
    let a = task.spawn(boom())
    let b = task.spawn(spin(2))
    let c = task.spawn(spin(3))
    let x = await b
    print "b ${x}"
    let y = await c
    print "c ${y}"
    await a
}

let ex = task.threadPoolExecutor(3)
print "total ${ex.runToCompletion(root())}"
`
	out, errOut, code := buildAndRun(t, src, vm.Options{})
	if out != "" {
		t.Fatalf("stdout = %q, want nothing: the unawaited panic cancels the root and the spinning siblings", out)
	}
	if errOut != "panic: boom\n" {
		t.Fatalf("stderr = %q, want the original panic text", errOut)
	}
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if after := goroutinesSettle(t); after > before {
		t.Fatalf("goroutines after the run: %d, before: %d: the pool's workers were not joined", after, before)
	}
}

func TestWorkerPanicDropsOwnedResource(t *testing.T) {
	src := `import task from std/task

type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    print "drop ${self.id}"
}

async fn boom() -> Int {
    let r = Res { id: 1 }
    None.expect("boom")
}

async fn leaf(n: Int) -> Int {
    n * n
}

async fn root() -> Int {
    let a = task.spawn(boom())
    let b = task.spawn(leaf(2))
    await a
}

let ex = task.threadPoolExecutor(3)
print "total ${ex.runToCompletion(root())}"
`
	out, errOut, code := buildAndRun(t, src, vm.Options{})
	if !strings.Contains(out, "drop 1") {
		t.Fatalf("stdout = %q, want the panicking task's resource dropped", out)
	}
	if errOut != "panic: boom\n" {
		t.Fatalf("stderr = %q, want the original panic text", errOut)
	}
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestCancelledRootJoinsLivePool(t *testing.T) {
	before := goroutinesSettle(t)
	src := `import task from std/task

async fn leaf() -> Int {
    1
}

async fn parks() -> Int {
    let t = task.spawn(leaf())
    await t
}

async fn root() -> Int {
    let c = task.spawn(parks())
    c.cancel()
    await c
}

let ex = task.threadPoolExecutor(3)
print "result ${ex.runToCompletion(root())}"
`
	_, errOut, code := buildAndRun(t, src, vm.Options{})
	if !strings.Contains(errOut, "the root task was cancelled") {
		t.Fatalf("stderr = %q, want the usual cancellation diagnostic", errOut)
	}
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if after := goroutinesSettle(t); after > before {
		t.Fatalf("goroutines after the run: %d, before: %d: a cancelled root left a pool's workers running", after, before)
	}
}

func TestFaultedRootJoinsOrphanedPool(t *testing.T) {
	before := goroutinesSettle(t)
	src := `import task from std/task

async fn leaf(n: Int) -> Int {
    n * 3
}

async fn job(n: Int) -> Int {
    let t = task.spawn(leaf(n))
    n + await t
}

fn launch(pool: task.Executor, n: Int) -> task.Task[Int] {
    pool.handoff(job(n))
}

async fn chain(n: Int) -> Int {
    let t = launch(task.threadPoolExecutor(2), n)
    await t
}

async fn boom() -> Int {
    None.expect("boom")
}

async fn quiet() -> Int {
    0
}

async fn root(first: task.Executor) -> Int {
    let orphaned = await first.handoff(chain(1))
    print "orphaned ${orphaned}"
    let c = task.spawn(boom())
    let q = task.spawn(quiet())
    await q
}

let first = task.threadPoolExecutor(2)
let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root(first))}"
`
	out, errOut, code := buildAndRun(t, src, vm.Options{})
	if out != "orphaned 4\n" || errOut != "panic: boom\n" || code != 2 {
		t.Fatalf("stdout = %q, stderr = %q, exit = %d, want the orphaned result then the child's panic", out, errOut, code)
	}
	if after := goroutinesSettle(t); after > before {
		t.Fatalf("goroutines after the run: %d, before: %d: a faulted root left a pool's workers running", after, before)
	}
}

func TestRootPanicStopsPoolWorkers(t *testing.T) {
	before := goroutinesSettle(t)
	src := `import task from std/task

async fn leaf() -> Int {
    1
}

async fn root() -> Int {
    let t = task.spawn(leaf())
    let v = await t
    None.expect("root boom ${v}")
}

let ex = task.threadPoolExecutor(3)
print "total ${ex.runToCompletion(root())}"
`
	_, errOut, code := buildAndRun(t, src, vm.Options{})
	if errOut != "panic: root boom 1\n" || code != 2 {
		t.Fatalf("stderr = %q, exit = %d, want the root's own panic", errOut, code)
	}
	if after := goroutinesSettle(t); after > before {
		t.Fatalf("goroutines after the run: %d, before: %d: the pool's workers stayed parked", after, before)
	}
}

// Thread pools are unreachable under Sandbox, so no Kigumi program can split the step budget.
func TestSandboxRejectsThreadPool(t *testing.T) {
	src := `import task from std/task

fn main() -> Void! {
    let ex = task.threadPoolExecutor(2)
    _ = ex
}
`
	_, errOut, code := buildAndRun(t, src, vm.Options{Sandbox: true, StepLimit: 1000, DepthLimit: 100})
	if !strings.Contains(errOut, "a thread pool executor is not available here") {
		t.Fatalf("stderr = %q, want the sandbox rejection", errOut)
	}
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestSandboxStepBudgetSameDiagnosticWithoutAPool(t *testing.T) {
	src := `fn spin() -> Int {
    let mut i = 0
    for i < 100000000 {
        i = i + 1
    }
    i
}

print "total ${spin()}"
`
	const limit = 2_000
	_, errOut, code := buildAndRun(t, src, vm.Options{Sandbox: true, StepLimit: limit, DepthLimit: 1000})
	if !strings.Contains(errOut, "comptime block exceeded its step limit") {
		t.Fatalf("stderr = %q, want the step-budget diagnostic", errOut)
	}
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestSandboxStepBudgetFromWorker(t *testing.T) {
	before := goroutinesSettle(t)
	src := `import task from std/task

async fn spin() -> Int {
    let mut i = 0
    for i < 100000000 {
        i = i + 1
    }
    i
}

async fn root() -> Int {
    let a = task.spawn(spin())
    let b = task.spawn(spin())
    let c = task.spawn(spin())
    let d = task.spawn(spin())
    await a + await b + await c + await d
}

let ex = task.threadPoolExecutor(3)
print "total ${ex.runToCompletion(root())}"
`
	const limit = 200_000
	res := checkSource(t, src)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}
	var out, errOut bytes.Buffer
	code, steps, err := vm.RunSandboxedPool(prog, vm.Options{Sandbox: true, StepLimit: limit, DepthLimit: 1000}, &out, &errOut)
	if err != nil {
		t.Fatalf("vm: %v", err)
	}
	if !strings.Contains(errOut.String(), "comptime block exceeded its step limit") || code != 2 {
		t.Fatalf("stderr = %q, exit = %d, want the step-budget diagnostic and exit 2", errOut.String(), code)
	}
	if steps != limit {
		t.Fatalf("lanes drew %d steps, want exactly the %d-step limit in total", steps, limit)
	}
	if after := goroutinesSettle(t); after > before {
		t.Fatalf("goroutines after the run: %d, before: %d: the pool's workers were not joined", after, before)
	}
}

// Recovering at every frame would make a deep unwind quadratic.
func TestDeepPanicUnwindsInOnePass(t *testing.T) {
	src := `fn rec(n: Int) -> Int {
    if n == 0 {
        let _: Void = None.expect("deep")
    }
    1 + rec(n - 1)
}

print "${rec(30000)}"
`
	start := time.Now()
	_, stderr, code := buildAndRun(t, src, vm.Options{})
	if code != 2 || !strings.Contains(stderr, "panic: deep") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("unwinding 30000 frames took %v", d)
	}
}

const heldPoolPrelude = `import task from std/task

type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    print "drop ${self.id}"
}

async fn tick() -> Int {
    1
}

async fn spin() -> Int {
    let mut n = 0
    for !task.cancelRequested() {
        n = n + await task.spawn(tick())
    }
    n
}

async fn parks(r: Res) -> Int {
    let t = task.spawn(spin())
    await t
}
`

func TestFaultedRootCancelsSiblingsAndJoinsHeldPool(t *testing.T) {
	before := goroutinesSettle(t)
	src := heldPoolPrelude + `
async fn boom() -> Int {
    None.expect("boom")
}

async fn root() -> Int {
    let s = task.spawn(spin())
    let p = task.spawn(parks(Res { id: 7 }))
    let b = task.spawn(boom())
    await b
}

let ex = task.threadPoolExecutor(3)
print "total ${ex.runToCompletion(root())}"
`
	out, errOut, code := buildAndRun(t, src, vm.Options{})
	if out != "drop 7\n" || errOut != "panic: boom\n" || code != 2 {
		t.Fatalf("stdout = %q, stderr = %q, exit = %d, want the sibling dropped, then the child's panic", out, errOut, code)
	}
	if after := goroutinesSettle(t); after > before {
		t.Fatalf("goroutines after the run: %d, before: %d: a faulted root left a held pool's workers running", after, before)
	}
}

func TestCancelledRootJoinsHeldPoolWithLiveTask(t *testing.T) {
	before := goroutinesSettle(t)
	src := heldPoolPrelude + `
async fn root() -> Int {
    let s = task.spawn(spin())
    let c = task.spawn(parks(Res { id: 3 }))
    c.cancel()
    await c
}

let ex = task.threadPoolExecutor(3)
print "total ${ex.runToCompletion(root())}"
`
	out, errOut, code := buildAndRun(t, src, vm.Options{})
	if out != "drop 3\n" || errOut != "panic: the root task was cancelled\n" || code != 2 {
		t.Fatalf("stdout = %q, stderr = %q, exit = %d, want the cancelled task dropped, then the cancellation", out, errOut, code)
	}
	if after := goroutinesSettle(t); after > before {
		t.Fatalf("goroutines after the run: %d, before: %d: a cancelled root left a held pool's workers running", after, before)
	}
}
