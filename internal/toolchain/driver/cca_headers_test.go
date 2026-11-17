package driver_test

import (
	"fmt"
	"strings"
	"testing"
)

func coseHeaderProgram(t *testing.T) (string, string) {
	t.Helper()
	cases := []struct {
		name, protected, unprotected, want string
	}{
		{"minimal", "a10126", "a0", "ok -7"},
		{"nested", "a2012604a101820203", "a0", "ok -7"},
		{"text-label", "a20126617800", "a0", "ok -7"},
		{"algorithm-duplicate", "a201260126", "a0", "cca: malformed duplicate COSE header label"},
		{"protected-duplicate", "a3012604000401", "a0", "cca: malformed duplicate COSE header label"},
		{"integer-encoding-duplicate", "a301260400180401", "a0", "cca: malformed duplicate COSE header label"},
		{"text-encoding-duplicate", "a3012661780078017801", "a0", "cca: malformed duplicate COSE header label"},
		{"unprotected-duplicate", "a10126", "a204000401", "cca: malformed duplicate COSE header label"},
		{"unprotected-text-duplicate", "a10126", "a261780078017801", "cca: malformed duplicate COSE header label"},
		{"unprotected-critical", "a10126", "a1028101", "cca: malformed COSE critical header must be protected"},
		{"protected-critical", "a20126028101", "a0", "cca: unsupported COSE critical header"},
		{"nested-map-overflow", "a2012604bb8000000000000000", "a0", "cca: CBOR map is truncated"},
		{"nested-array-overflow", "a20126049bffffffffffffffff", "a0", "cca: CBOR array is truncated"},
		{"protected-map-overflow", "bb8000000000000000", "a0", "cca: CBOR map is truncated"},
		{"unprotected-map-overflow", "a10126", "bb8000000000000000", "cca: CBOR map is truncated"},
		{"invalid-text-label", "a2012661ff00", "a0", "cca: malformed COSE text label"},
		{"invalid-label-type", "a20126417800", "a0", "cca: malformed COSE header label"},
		{"large-integer-label", "a201261bffffffffffffffff00", "a0", "cca: malformed COSE integer label"},
		{"deep-array", "a2012604" + strings.Repeat("81", 65) + "00", "a0", "cca: unsupported CBOR nesting deeper than 64"},
	}
	var src, want strings.Builder
	src.WriteString(`import cca from std/cvm/cca
import hex from std/encoding/hex
fn show(name: String, protected: String, unprotected: String) -> Void! {
    let mut out = Array.of[u8](0xd2, 0x84)
    cca.putByteString(&mut out, hex.decode(protected)?)
    cca.appendBytes(&mut out, hex.decode(unprotected)?)
    cca.putByteString(&mut out, b"\x01")
    cca.putByteString(&mut out, Bytes.zeros(64))
    match cca.parseCoseSign1(Bytes.fromArray(&out)) {
        Ok(token) -> print "${name}=ok ${token.algorithm}"
        Err(error) -> print "${name}=${error.message()}"
    }
}
fn main() -> Void! {
`)
	for _, tc := range cases {
		fmt.Fprintf(&src, "    show(%q, %q, %q)?\n", tc.name, tc.protected, tc.unprotected)
		fmt.Fprintf(&want, "%s=%s\n", tc.name, tc.want)
	}
	src.WriteString("}\n")
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main/main.kg": src.String()})
	return root, want.String()
}

func TestCoseHeaderValidation(t *testing.T) {
	root, want := coseHeaderProgram(t)
	for _, engine := range []struct {
		name string
		run  func(*testing.T, string) (string, string, int)
	}{{"vm", runFixtureVM}, {"native", runFixtureNative}} {
		t.Run(engine.name, func(t *testing.T) {
			if engine.name == "native" && testCompiler().CCompiler() == nil {
				t.Skip("no C compiler")
			}
			out, errOut, code := engine.run(t, root)
			if out != want || errOut != "" || code != 0 {
				t.Fatalf("stdout=%q\nwant=%q\nstderr=%q exit=%d", out, want, errOut, code)
			}
		})
	}
}
