package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const printParallelSrc = `import task from std/task

async fn job(n: Int) -> Int {
    let word = "task ${n} "
    let mut line = ""
    let mut j = 0
    for j < LEN {
        line = "${line}${word}"
        j = j + 1
    }
    let mut i = 0
    for i < 100 {
        print line
        eprint line
        i = i + 1
    }
    n
}

async fn root() -> Int {
    let mut kids = Array.empty[task.Task[Int]]()
    let mut k = 0
    for k < 8 {
        kids.push(task.spawn(job(k)))
        k = k + 1
    }
    let mut sum = 0
    for h in kids {
        sum = sum + await h
    }
    sum
}

let pool = task.threadPoolExecutor(4)
print "total ${pool.runToCompletion(root())}"
`

func printParallelModule(t *testing.T, lineLen string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"printpar\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(strings.Replace(printParallelSrc, "LEN", lineLen, 1)), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func sortedLines(s string) []string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	slices.Sort(lines)
	return lines
}

// Every line must arrive whole and the multiset of lines must not change between runs or backends; only their order may.
func checkPrintedLines(t *testing.T, name, out, errOut string, want []string) {
	t.Helper()
	if got := sortedLines(out); !slices.Equal(got, want) {
		t.Fatalf("%s stdout lines differ: %d lines, want %d (first mismatch %q)", name, len(got), len(want), firstDifference(got, want))
	}
	if got := sortedLines(errOut); !slices.Equal(got, want[:len(want)-1]) {
		t.Fatalf("%s stderr lines differ: %d lines, want %d (first mismatch %q)", name, len(got), len(want)-1, firstDifference(got, want[:len(want)-1]))
	}
}

func firstDifference(got, want []string) string {
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			return got[i]
		}
	}
	return ""
}

func wantPrintedLines(lineLen int) []string {
	var want []string
	for k := range 8 {
		for range 100 {
			want = append(want, strings.Repeat("task "+string(rune('0'+k))+" ", lineLen))
		}
	}
	want = append(want, "total 28")
	slices.Sort(want)
	return want
}

func TestParallelPrintsStayWhole(t *testing.T) {
	for _, c := range []struct {
		name string
		len  int
	}{{"short", 3}, {"past a pipe buffer", 600}} {
		t.Run(c.name, func(t *testing.T) {
			want := wantPrintedLines(c.len)
			m := printParallelModule(t, strconv.Itoa(c.len))
			for run := range 2 {
				var out, errOut bytes.Buffer
				if code, err := driver.Run(m, &out, &errOut, nil); err != nil || code != 0 {
					t.Fatalf("vm run %d: %d %v\n%s", run, code, err, errOut.String())
				}
				checkPrintedLines(t, "vm", out.String(), errOut.String(), want)
			}
			if testCompiler().CCompiler() == nil {
				return
			}
			exe := filepath.Join(t.TempDir(), "prog")
			var buildErr bytes.Buffer
			if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			for run := range 2 {
				var out, errOut bytes.Buffer
				cmd := exec.Command(exe)
				cmd.Stdout, cmd.Stderr = &out, &errOut
				if code := exitCodeOf(cmd.Run()); code != 0 {
					t.Fatalf("native run %d: exit %d\n%s", run, code, errOut.String())
				}
				checkPrintedLines(t, "native", out.String(), errOut.String(), want)
			}
		})
	}
}
