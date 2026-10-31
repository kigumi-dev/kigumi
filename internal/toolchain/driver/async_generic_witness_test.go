package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const asyncGenericWitnessSrc = `import task from std/task

interface Show {
    fn show(self) -> String
}

type Tag = {
    pub s String
}

pub fn Tag.show(self) -> String { "<${self.s}>" }

async fn sameLater[T: Eq](a: T, b: T) -> Bool { a == b }

async fn lessLater[T: Ord](a: T, b: T) -> Bool { a < b }

async fn hashLater[T: Hash](a: T, b: T) -> Bool { a.hash() == b.hash() }

async fn showLater[T: Show](v: T, pre: String) -> String { "${pre}${v.show()}" }

async fn driver() -> Int {
    let zero = 0.0 * 1.0
    let nan = zero / zero
    print "${await sameLater(nan, nan)} ${await sameLater(1, 1)} ${await sameLater("a", "b")}"
    print "${await lessLater(1, 2)} ${await lessLater("b", "a")} ${await lessLater(nan, 1.0)}"
    print "${await hashLater("x", "x")} ${await hashLater(1, 2)}"
    print "${await showLater(Tag { s: "t" }, "> ")}"
    let a = task.spawn(sameLater(3, 3))
    let b = task.spawn(showLater(Tag { s: "u" }, "# "))
    let c = task.spawn(lessLater(2.5, 1.5))
    print "${await a} ${await b} ${await c}"
    0
}

let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(driver())}"
`

const asyncGenericWitnessOut = `false true false
true false false
true false
> <t>
true # <u> false
0
`

func asyncWitnessModule(t *testing.T) (*driver.Module, string) {
	t.Helper()
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"asyncwitness\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(asyncGenericWitnessSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m, root
}

// A direct call of a generic async fn passes its witnesses into the future, awaited or spawned, on both engines.
func TestAsyncGenericWitnessDirectCallVM(t *testing.T) {
	t.Parallel()
	m, root := asyncWitnessModule(t)
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &out, &errOut, []string{"main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("run: code %d, %v\n%s", code, err, errOut.String())
	}
	if out.String() != asyncGenericWitnessOut {
		t.Errorf("stdout\n%s\nwant\n%s", out.String(), asyncGenericWitnessOut)
	}
}

func TestAsyncGenericWitnessDirectCallNative(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	m, root := asyncWitnessModule(t)
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	got, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, got)
	}
	if string(got) != asyncGenericWitnessOut {
		t.Errorf("stdout\n%s\nwant\n%s", got, asyncGenericWitnessOut)
	}
}
