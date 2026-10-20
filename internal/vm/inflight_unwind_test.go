package vm_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

// Sorts before tests_report_test.go, whose abandoned timed-out test keeps freeing into the live counters.
func runTracked(t *testing.T, src string) (stdout string, live, freedUses, strays int64) {
	t.Helper()
	res := checkSource(t, src)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	plans := []mir.Transform{{Name: "plan-rc", Run: mir.PlanRC}, {Name: "specialize", Run: mir.Specialize}, {Name: "elide-rc", Run: mir.ElideRC}, {Name: "plan-direct", Run: mir.PlanDirect}}
	if err := prog.Apply(plans...); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	goroutinesSettle(t)
	vm.TrackLive(true)
	defer vm.TrackLive(false)
	if _, err := vm.RunReleasing(prog, vm.Options{Cwd: t.TempDir()}, &out, &errOut, func() {}); err != nil {
		t.Fatal(err)
	}
	return out.String(), vm.LiveObjects(), vm.LiveFreedUses(), vm.LiveStrays()
}

func TestUnwoundInFlightCallsDropEachOwnerOnce(t *testing.T) {
	quiesce()
	for _, name := range []string{
		"async_task_panic_callback_lent",
		"async_task_panic_callback_capture",
		"async_task_panic_lent_receiver",
		"async_task_panic_fold_owned_items",
		"async_task_panic_temps_callback",
		"async_task_drop_panic_callback_args",
		"async_task_panic_sync_deep",
	} {
		t.Run(name, func(t *testing.T) {
			fx := readFixture(t, "../../testdata/run/"+name+".txtar")
			out, _, freedUses, strays := runTracked(t, fx.src)
			if out != fx.stdout {
				t.Errorf("stdout\n%s\nwant\n%s", out, fx.stdout)
			}
			if freedUses != 0 || strays != 0 {
				t.Errorf("%d releases through a freed object, %d strays", freedUses, strays)
			}
		})
	}
}

func inFlightLeakSrc(shape string, k int) string {
	return fmt.Sprintf(`import task from std/task

type Res = resource {
    pub id Int
}

fn Res.drop(move self) -> Unit {
    let seen = "drop ${self.id}"
}

fn explode() -> Int {
    None.expect("boom")
}

fn items(k: Int) -> Array[Res] {
    let mut out = Array.empty[Res]()
    let mut i = 0
    for i < k {
        out.push(Res { id: i })
        i = i + 1
    }
    out
}

fn run(f: fn(Int) -> Int) -> Int {
    f(1)
}

fn viaMap() -> Int {
    let xs = items(%[1]d)
    let ys = xs.map((x: &Res) => explode())
    0
}

fn viaCapture() -> Int {
    let xs = items(%[1]d)
    run((n: Int) => (xs.len().toInt() || 0) + explode())
}

fn viaFold() -> Int {
    let xs = items(%[1]d)
    xs.fold(0, (acc: Int, x: Res) => explode())
}

async fn boom() -> Int {
    %[2]s()
}

async fn root() -> Int {
    let h = task.spawn(boom())
    await h
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`, k, shape)
}

func TestInFlightCallbackLeavesNothingBehind(t *testing.T) {
	quiesce()
	for _, shape := range []string{"viaMap", "viaCapture", "viaFold"} {
		t.Run(shape, func(t *testing.T) {
			var live []int64
			for _, k := range []int{1, 3, 6} {
				_, n, freedUses, _ := runTracked(t, inFlightLeakSrc(shape, k))
				if freedUses != 0 {
					t.Errorf("k=%d: %d releases through a freed object", k, freedUses)
				}
				live = append(live, n)
			}
			if live[0] != live[1] || live[1] != live[2] {
				t.Errorf("objects live after the fault for 1, 3 and 6 items: %v, want the same count", live)
			}
		})
	}
}

func TestNestedCallbacksUnwindInLinearTime(t *testing.T) {
	src := `import task from std/task

fn deep(n: Int, f: fn(Int) -> Int) -> Int {
    if n == 0 {
        None.expect("deep")
    } else {
        f(n - 1)
    }
}

fn spin(n: Int) -> Int {
    deep(n, (k: Int) => spin(k))
}

async fn boom() -> Int {
    spin(4000)
}

async fn root() -> Int {
    let h = task.spawn(boom())
    await h
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`
	out, _, _, _ := runTracked(t, src)
	if strings.TrimSpace(out) != "" {
		t.Errorf("stdout %q", out)
	}
}
