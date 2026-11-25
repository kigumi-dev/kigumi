package driver_test

import (
	"strings"
	"testing"
)

// A cancelled and dropped awaiter must leave no waiter link behind: the task it
// gave up waiting on keeps running, so its panic has to reach the parent and
// cancel the siblings instead of waking a task that is already gone.
const waiterCancelSrc = `import task from std/task

fn dropTask[T](t: task.Task[T]) {
    let _ = t
}

async fn boom() -> Int {
    await task.yieldNow()
    None.expect("boom")
}

async fn join(t: task.Task[Int]) -> Int {
    await t
}

async fn spin() -> Int {
    let mut i = 0
    for i < 20 {
        await task.yieldNow()
        i = i + 1
    }
    print "sibling finished"
    i
}

async fn root() -> Int {
    let b = task.spawn(boom())
    let a = task.spawn(join(b))
    let s = task.spawn(spin())
    await task.yieldNow()
    a.cancel()
    dropTask(a)
    let last = await s
    last
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`

func TestCancelledAwaiterLeavesNoWaiterLinkVmNative(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main/main.kg": waiterCancelSrc})
	vmOut, vmErr, vmCode := runFixtureVM(t, root)
	if vmCode != 2 || strings.Contains(vmOut, "sibling finished") {
		t.Fatalf("VM: exit=%d stdout=%q stderr=%q, want the unawaited panic to cancel the running sibling", vmCode, vmOut, vmErr)
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	nativeOut, nativeErr, nativeCode := runFixtureNative(t, root)
	if nativeCode != 2 || strings.Contains(nativeOut, "sibling finished") {
		t.Fatalf("native: exit=%d stdout=%q stderr=%q, want the unawaited panic to cancel the running sibling", nativeCode, nativeOut, nativeErr)
	}
}
