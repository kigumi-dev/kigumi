package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const scopedCopySrc = `import {AllocatorHandle} from std/alloc

type Nm = {
    pub name String
    pub mut n Int
}

fn bump(b: &mut Nm?) {
    match b {
        Some(x) -> { x.n = 2 }
        None -> {}
    }
}

fn outside(o: Nm?) -> Int {
    let mut b = o
    bump(&mut b)
    if o is Some(_) { 1 } else { 0 }
}

fn inside(h: AllocatorHandle, o: Nm?) -> Int {
    allocator h.scope() {
        let mut b = o
        bump(&mut b)
        if o is Some(_) { 1 } else { 0 }
    }
}

fn nested(h: AllocatorHandle, o: Nm?) -> Int {
    allocator h.scope() {
        let read = () => {
            let mut b = o
            bump(&mut b)
            let r: Int = if o is Some(_) { 1 } else { 0 }
            r
        }
        read()
    }
}

fn absent(o: &Nm?) -> Int {
    if o is Some(_) { 1 } else { 0 }
}

fn main() -> Void! {
    let n = 1
}
`

func TestPlannedCopyInAnAllocatorBlockNeedsNoDefaultAllocator(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"scopedcopy\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(scopedCopySrc), 0o644)
	std, _ := filepath.Abs("../../std")
	target := driver.HostTarget()
	target.Allocator = "none"
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", a.Res.Render())
	}
	for name, want := range map[string]bool{"outside": false, "inside": true, "nested": true, "absent": true} {
		if got := a.Res.Summary(mainFn(t, a.Res, name)).Default; got != want {
			t.Errorf("%s needs no default allocator = %v, want %v", name, got, want)
		}
	}
}
