package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func unwindLeakSrc(depth int) string {
	var sb strings.Builder
	sb.WriteString(`import task from std/task

type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    let seen = "drop ${self.id}"
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

fn explode() -> Int {
    None.expect("deep")
}

fn f0() {
    let s = "leaf"
    let a = build(2)
    let r = Res { id: 0 }
    let t = Array.of(Res { id: 100 }, Res { id: explode() })
    let seen = "unreachable ${t.len()}"
}
`)
	for i := 1; i < depth; i++ {
		fmt.Fprintf(&sb, "\nfn f%d() {\n    let s = \"frame %d\"\n    let a = build(2)\n    let r = Res { id: %d }\n    f%d()\n}\n", i, i, i, i-1)
	}
	fmt.Fprintf(&sb, `
async fn tick() -> Int {
    1
}

async fn boom() -> Int {
    let keep = build(2)
    let v = await tick()
    f%d()
    v
}

async fn root() -> Int {
    let h = task.spawn(boom())
    await h
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`, depth-1)
	return sb.String()
}

var liveRe = regexp.MustCompile(`live: (\d+)`)

func TestUnwoundSyncFramesLeaveNothingBehind(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	std, _ := filepath.Abs("../../std")
	var live []string
	for _, depth := range []int{1, 3, 6} {
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"unwindleak\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
		os.WriteFile(filepath.Join(root, "main.kg"), []byte(unwindLeakSrc(depth)), 0o644)
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
		if err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(root, "prog")
		var buildErr bytes.Buffer
		ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_LEAK_CHECK"}})
		if err != nil || !ok {
			t.Fatalf("depth %d: build: %v\n%s", depth, err, buildErr.String())
		}
		var errOut bytes.Buffer
		cmd := exec.Command(exe)
		cmd.Stderr = &errOut
		if exit := exitCodeOf(cmd.Run()); exit != 2 {
			t.Fatalf("depth %d: exit %d, stderr %q", depth, exit, errOut.String())
		}
		m2 := liveRe.FindStringSubmatch(errOut.String())
		if m2 == nil {
			t.Fatalf("depth %d: no live count in %q", depth, errOut.String())
		}
		live = append(live, m2[1])
	}
	if live[0] != live[1] || live[1] != live[2] {
		t.Errorf("allocations still held at exit for 1, 3 and 6 unwound frames: %v, want the same count", live)
	}
}
