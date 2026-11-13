package driver_test

import (
	"testing"
	"time"
)

// A CError created by std/ffi inside a std body must keep its concrete type
// through `error.as` on both engines; the VM used to see only an opaque Error.
func TestCErrorDowncastMatchesNative(t *testing.T) {
	const src = `import error from std/error
import ffi from std/ffi
import fs from std/fs

let files = host.files()
match files.open(fs.Path.fromString("/no/such/kigumi-cerror")?) {
    Ok(_) -> print "opened"
    Err(e) -> match error.as[ffi.CError](&e) {
        Some(c) -> print "CError code=${c.code}"
        None -> print "not CError: ${e.message()}"
    }
}
`
	const want = "CError code=2\n"
	if got := runGeneratedVM(t, src, time.Minute); got != want {
		t.Fatalf("vm: got %q, want %q", got, want)
	}
	if got := runGeneratedAOT(t, src, time.Minute); got != want {
		t.Fatalf("native: got %q, want %q", got, want)
	}
}
