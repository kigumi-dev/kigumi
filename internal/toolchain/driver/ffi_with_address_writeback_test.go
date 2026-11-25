package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A withAddress pointer aliases the borrowed variable, so a write through it
// must reach that variable. The VM keeps the CValue in a table and used to let
// the callback's writes go with the call; withAddressConst stays read-only.
func TestFFIWithAddressWritesBackOnVM(t *testing.T) {
	t.Parallel()
	const src = `import mem from std/mem
import ffi from std/ffi

type S = {
    pub mut n i32
}

fn main() -> Void! {
    let mut x: u32 = 0
    ffi.withAddress(&mut x, (p: *mut u32) => { unsafe { mem.volatileWrite(p, 7) } })
    print "u32 volatile ${x}"

    let r = ffi.withAddress(&mut x, (p: *mut u32) => {
        unsafe { mem.volatileWrite(p, 11); mem.volatileRead(ffi.toConst(p)) }
    })
    print "returned ${r} after ${x}"

    let c = ffi.withAddressConst(&x, (p: *const u32) => unsafe { mem.volatileRead(p) })
    print "const ${c} after ${x}"

    let mut s = S { n: 5 }
    ffi.withAddress(&mut s.n, (p: *mut i32) => { unsafe { mem.volatileWrite(p, 42) } })
    print "field ${s.n}"

    let mut xs = Array.of[i32](1, 2, 3)
    ffi.withAddress(&mut xs[1], (p: *mut i32) => { unsafe { mem.volatileWrite(p, 42) } })
    print "elem ${xs[1]} ${xs[0]}"
}
`
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"withaddresswriteback\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.Run(m, &out, &errOut, nil)
	if err != nil || code != 0 {
		t.Fatalf("run: %v, exit %d\nstderr:\n%s", err, code, errOut.String())
	}
	want := "u32 volatile 7\nreturned 11 after 11\nconst 11 after 11\nfield 42\nelem 42 1\n"
	if out.String() != want {
		t.Errorf("stdout\n--- got ---\n%s--- want ---\n%s", out.String(), want)
	}
}
