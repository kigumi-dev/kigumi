package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func TestVMDropsDeepOwnershipChains(t *testing.T) {
	root := t.TempDir()
	src := `import {Box} from std/alloc

type List = Nil | Cons(next: Box[List])
type Link = { pub next Option[Box[Link]] }
type Tree = { pub kids Array[Tree] }

fn boxes(n: Int) -> Int {
    let mut list = Nil
    let mut i = 0
    for i < n {
        list = Cons(Box.new(list))
        i = i + 1
    }
    n
}

fn options(n: Int) -> Int {
    let mut link = Link { next: None }
    let mut i = 0
    for i < n {
        link = Link { next: Some(Box.new(link)) }
        i = i + 1
    }
    n
}

fn arrays(n: Int) -> Int {
    let mut tree = Tree { kids: Array.empty[Tree]() }
    let mut i = 0
    for i < n {
        tree = Tree { kids: Array.of(tree) }
        i = i + 1
    }
    n
}

print "boxes ${boxes(1000000)}"
print "options ${options(300000)}"
print "arrays ${arrays(300000)}"
`
	if err := os.MkdirAll(filepath.Join(root, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	start := time.Now()
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("run: %v exit %d\n%s", err, code, errOut.String())
	}
	if want := "boxes 1000000\noptions 300000\narrays 300000\n"; out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
	if d := time.Since(start); d > 30*time.Second {
		t.Errorf("took %v, want under 30s", d)
	}
}
