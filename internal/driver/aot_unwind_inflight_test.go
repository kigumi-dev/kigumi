package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

func inFlightSrc(shape string, k int) string {
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

fn viaBorrow(x: &Array[Res]) -> Int {
    let own = items(%[1]d)
    run((n: Int) => (x.len().toInt() || 0) + (own.len().toInt() || 0) + explode())
}

fn viaCapture() -> Int {
    let xs = items(%[1]d)
    viaBorrow(&xs)
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

func liveAfterFault(t *testing.T, src string) string {
	t.Helper()
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"inflightleak\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_LEAK_CHECK"}})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	var errOut bytes.Buffer
	cmd := exec.Command(exe)
	cmd.Stderr = &errOut
	if exit := exitCodeOf(cmd.Run()); exit != 2 {
		t.Fatalf("exit %d, stderr %q", exit, errOut.String())
	}
	found := liveRe.FindStringSubmatch(errOut.String())
	if found == nil {
		t.Fatalf("no live count in %q", errOut.String())
	}
	return found[1]
}

func TestInFlightCallbackLeavesNothingBehindNative(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, shape := range []string{"viaMap", "viaCapture", "viaFold"} {
		t.Run(shape, func(t *testing.T) {
			t.Parallel()
			var live []string
			for _, k := range []int{1, 3, 6} {
				live = append(live, liveAfterFault(t, inFlightSrc(shape, k)))
			}
			if live[0] != live[1] || live[1] != live[2] {
				t.Errorf("allocations still held at exit for 1, 3 and 6 items: %v, want the same count", live)
			}
		})
	}
}
