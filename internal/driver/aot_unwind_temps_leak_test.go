package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const tempsLeakPrelude = `import task from std/task

type Res = resource {
    pub id Int
}

fn Res.drop(move self) {
    let seen = "drop ${self.id}"
}

type Bad = resource {
    pub id Int
}

fn Bad.drop(move self) {
    let _: Void = None.expect("bad drop")
}

fn make(n: Int) -> Res {
    Res { id: n }
}

fn explode() -> Int {
    None.expect("explode")
}

fn consume(rs: Array[Res], n: Int) -> Int {
    n
}

async fn tick() -> Int {
    1
}
`

// The k temporaries are live when a later argument panics, mid-frame and across a suspension.
func asyncTempsSrc(k int) string {
	items := make([]string, k)
	for i := range items {
		items[i] = fmt.Sprintf("make(%d)", i)
	}
	return tempsLeakPrelude + fmt.Sprintf(`
async fn boom() -> Int {
    let v = consume(Array.of(%s), explode())
    v
}

async fn boomAfterAwait() -> Int {
    let v = consume(Array.of(%s), explode() + (await tick()))
    v
}

async fn root() -> Int {
    let a = task.spawn(boom())
    let b = task.spawn(boomAfterAwait())
    let x = await a
    await b + x
}

let ex = task.currentThreadExecutor()
print "total ${ex.runToCompletion(root())}"
`, strings.Join(items, ", "), strings.Join(items, ", "))
}

// A drop that panics outside any task must still release the k locals the frame holds.
func outerDropSrc(k int) string {
	var sb strings.Builder
	sb.WriteString(tempsLeakPrelude)
	sb.WriteString("\nfn scope() {\n")
	for i := range k {
		fmt.Fprintf(&sb, "    let r%d = Res { id: %d }\n    let s%d = \"text %d\"\n", i, i, i, i)
	}
	sb.WriteString("    let bad = Bad { id: 1 }\n}\n\nlet ex = task.currentThreadExecutor()\nprint \"start ${ex.runToCompletion(tick())}\"\nscope()\n")
	return sb.String()
}

// The same frame in a program that never touches a task executor.
func outerDropPlainSrc(k int) string {
	src := strings.Replace(outerDropSrc(k), "import task from std/task\n\n", "", 1)
	src = strings.Replace(src, "async fn tick() -> Int {\n    1\n}\n", "", 1)
	src = strings.Replace(src, "let ex = task.currentThreadExecutor()\nprint \"start ${ex.runToCompletion(tick())}\"\n", "", 1)
	return src
}

func TestUnwoundTemporariesLeaveNothingBehind(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	std, _ := filepath.Abs("../../std")
	for name, gen := range map[string]func(int) string{"async temporaries": asyncTempsSrc, "outside a task": outerDropSrc, "outside a task without an executor": outerDropPlainSrc} {
		var live []string
		for _, k := range []int{1, 3, 6} {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"tempsleak\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte(gen(k)), 0o644)
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
			if err != nil {
				t.Fatal(err)
			}
			exe := filepath.Join(root, "prog")
			var buildErr bytes.Buffer
			ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_LEAK_CHECK"}})
			if err != nil || !ok {
				t.Fatalf("%s %d: build: %v\n%s", name, k, err, buildErr.String())
			}
			var errOut bytes.Buffer
			cmd := exec.Command(exe)
			cmd.Stderr = &errOut
			if exit := exitCodeOf(cmd.Run()); exit != 2 {
				t.Fatalf("%s %d: exit %d, stderr %q", name, k, exit, errOut.String())
			}
			m2 := liveRe.FindStringSubmatch(errOut.String())
			if m2 == nil {
				t.Fatalf("%s %d: no live count in %q", name, k, errOut.String())
			}
			live = append(live, m2[1])
		}
		if live[0] != live[1] || live[1] != live[2] {
			t.Errorf("%s: allocations still held at exit for 1, 3 and 6 temporaries: %v, want the same count", name, live)
		}
	}
}
