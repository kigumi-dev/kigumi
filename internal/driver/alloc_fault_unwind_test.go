package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const unwindSweepSrc = `import task from std/task

type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    let keep = "drop ${self.id}"
}

fn build(n: Int) -> Array[String] {
    let mut out = Array.empty[String]()
    let mut i = 0
    for i < n {
        out.push("item ${i}")
        i = i + 1
    }
    out
}

type Pt = {
    pub name String
    pub tags Array[String]
}

fn mkPt(n: Int) -> Pt {
    Pt { name: "pt ${n}", tags: build(2) }
}

fn consume(p: Pt) -> Int {
    (p.name.len().toInt() || 0) + (p.tags.len().toInt() || 0)
}

fn branchy(n: Int) -> Int {
    let p = mkPt(n)
    let q = mkPt(n + 1)
    if n > 1 { consume(p) } else { consume(q) + (p.name.len().toInt() || 0) }
}

fn loopy(n: Int) -> Int {
    let mut total = 0
    let mut i = 0
    for i < 3 {
        let p = mkPt(i)
        total = total + consume(p)
        i = i + 1
    }
    total + n
}

fn viaClosure(n: Int) -> Int {
    let base = mkPt(n)
    let f = (k: Int) => k + (base.tags.len().toInt() || 0)
    f(n) + f(1)
}

fn leaf(n: Int) -> Int {
    let r = Res { id: n }
    let items = build(3)
    let joined = "${items[0]}-${items[1]}-${n}"
    (joined.len().toInt() || 0) + r.id
}

fn mid(n: Int) -> Int {
    let a = build(2)
    let r = Res { id: n }
    let v = leaf(n) + (a.len().toInt() || 0) + branchy(n) + loopy(n) + viaClosure(n)
    v + r.id
}

fn top(n: Int) -> Int {
    let s = "top ${n}"
    let r = Res { id: n }
    mid(n) + (s.len().toInt() || 0) + r.id
}

async fn worker(n: Int) -> Int {
    let keep = build(2)
    let v = top(n)
    (keep.len().toInt() || 0) + v
}

async fn root() -> Int {
    let a = task.spawn(worker(1))
    let b = task.spawn(worker(2))
    let c = task.spawn(worker(3))
    (await a) + (await b) + (await c)
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`

const unwindSweepRange = 900

func loadUnwindSweepModule(t *testing.T) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"unwindsweep\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(unwindSweepSrc), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func checkUnwindOutcome(t *testing.T, n, exit int, out, errOut string) (faulted bool) {
	t.Helper()
	switch exit {
	case 0:
		if !strings.HasPrefix(out, "total ") {
			t.Errorf("N=%d: exit 0 with stdout %q", n, out)
		}
	case 2:
		if !strings.HasPrefix(errOut, "panic: ") || strings.Contains(errOut, "use after free") {
			t.Errorf("N=%d: exit 2 with stderr %q", n, errOut)
		}
		return true
	default:
		t.Errorf("N=%d: exit %d, stdout=%q stderr=%q", n, exit, out, errOut)
	}
	return false
}

func TestUnwindSweepNative(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	m := loadUnwindSweepModule(t)
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT", "-DKIGUMI_RC_CHECK"}})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	faults := 0
	for n := 1; n <= unwindSweepRange; n++ {
		cmd := exec.Command(exe)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		exit := exitCodeOf(cmd.Run())
		if checkUnwindOutcome(t, n, exit, out.String(), errOut.String()) {
			faults++
		}
	}
	if faults < 20 {
		t.Errorf("only %d of %d allocation failures faulted the program", faults, unwindSweepRange)
	}
}
