package driver_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A ClientHello extension block can hold ~16,300 four-byte extensions, so
// duplicate detection must stay linear; the nested scan this replaces took
// ~30s on the VM for the block below.
const tlsExtensionParseSrc = `import binary from std/encoding/binary
import time from std/time
import tls from std/net/tls

fn extensions(count: usize) -> Bytes {
    let mut out = Array.empty[u8]()
    let mut i: usize = 0
    for i < count {
        binary.putU16Be(&mut out, (i % 65536).toU16() || 0)
        binary.putU16Be(&mut out, 0)
        i = i + 1
    }
    Bytes.fromArray(&out)
}

fn duplicateBlock() -> Bytes {
    let mut out = Array.empty[u8]()
    binary.putU16Be(&mut out, 42)
    binary.putU16Be(&mut out, 0)
    binary.putU16Be(&mut out, 43)
    binary.putU16Be(&mut out, 0)
    binary.putU16Be(&mut out, 42)
    binary.putU16Be(&mut out, 0)
    Bytes.fromArray(&out)
}

fn main() -> Void {
    let start = time.monotonic()
    match tls.parseExtensions(extensions(16300)) {
        Ok(exts) -> print "parsed=${exts.len()} first=${exts[0].typ} last=${exts[exts.len() - 1].typ} ms=${start.elapsedMillis()}"
        Err(e) -> print "error=${e.message()}"
    }
    match tls.parseExtensions(duplicateBlock()) {
        Ok(_) -> print "duplicate=accepted"
        Err(_) -> print "duplicate=rejected"
    }
}
`

func TestTLSParseExtensionsStaysLinear(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestModule(t, dir, tlsExtensionParseSrc)
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
	if err != nil {
		t.Fatalf("run: %v\n%s", err, errOut.String())
	}
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two result lines, got %q", out.String())
	}
	fields := strings.Fields(lines[0])
	if len(fields) != 4 || fields[0] != "parsed=16300" || fields[1] != "first=0" || fields[2] != "last=16299" {
		t.Fatalf("unexpected parse result %q", lines[0])
	}
	ms, err := strconv.ParseInt(strings.TrimPrefix(fields[3], "ms="), 10, 64)
	if err != nil {
		t.Fatalf("unexpected elapsed field %q: %v", fields[3], err)
	}
	// The nested scan took ~30s here; linear parsing stays well under a
	// second, and 5s leaves room for a loaded test runner.
	if ms > 5000 {
		t.Fatalf("parsing 16300 extensions took %dms; duplicate detection is not linear", ms)
	}
	if lines[1] != "duplicate=rejected" {
		t.Fatalf("duplicate extension was not rejected: %q", lines[1])
	}
}
