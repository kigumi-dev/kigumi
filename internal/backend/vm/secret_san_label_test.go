package vm_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

const secretSanLabelSrc = `import secret from std/secret

type P = {
    pub u Void
    pub s u64
}

type Q = {
    pub u Void
    pub b Bool
}

fn store_public_over_secret(s: u64) -> u64 {
    let mut xs = Array.of[u64](s, 1)
    xs[0] = 0
    xs[1]
}

fn store_raw(s: u64) -> u64 {
    let mut t = FixedArray.filled[u64, 64](0)
    t[1] = s
    t[2]
}

fn buffer_set(s: u64) -> u8 {
    let mut b = ByteBuffer.tryNew(4).unwrap()
    b.set(0, s.truncate[u8]())
    b.freeze().get(1) || 0
}

fn push_secret(s: u64) -> u64 {
    let mut xs = Array.empty[u64]()
    xs.push(1)
    xs.push(s)
    xs[0]
}

fn contains_secret(s: u64) -> Bool {
    "${s}".contains("4")
}

fn push_len(s: u64) -> usize {
    let mut xs = Array.empty[u64]()
    xs.push(1)
    xs.push(s)
    xs.len()
}

fn push_public_tag(s: u64) -> Bool {
    let mut xs = Array.empty[u64]()
    xs.push(s)
    xs.get(0) is Some(_)
}

fn interp_len(s: u64) -> usize {
    "${s}".toBytes().len()
}

fn declassify_buffer(s: u64) -> Bytes {
    let buf = secret.SecretBuffer.fromString("k${s}")
    unsafe { buf.declassify() }
}

fn unit_field(s: u64) -> Bool {
    let p = P { u: (), s: s }
    let _u = p.u
    let q = Q { u: (), b: true }
    q.b
}

fn main() {
    _ = store_public_over_secret(1)
    _ = store_raw(1)
    _ = buffer_set(1)
    _ = push_secret(1)
    _ = contains_secret(1)
    _ = push_len(1)
    _ = push_public_tag(1)
    _ = interp_len(1)
    _ = declassify_buffer(1)
    _ = unit_field(1)
}
`

// TestSecretSanLabelJoins pins the label model: a write joins into the content and never clears it, the shape
// (a length, a variant tag) stays public unless a primitive decided it on a secret, and the shared unit stays unlabelled.
func TestSecretSanLabelJoins(t *testing.T) {
	prog := secretSanProgram(t, secretSanLabelSrc)
	tests := []struct {
		fn      string
		tainted bool
	}{
		{"store_public_over_secret", true},
		{"store_raw", true},
		{"buffer_set", true},
		{"push_secret", true},
		{"contains_secret", true},
		{"push_len", false},
		{"push_public_tag", false},
		{"interp_len", true},
		{"declassify_buffer", false},
		{"unit_field", false},
	}
	for _, tc := range tests {
		t.Run(tc.fn, func(t *testing.T) {
			sh, err := vm.SecretSanRun(prog, vm.Options{}, tc.fn, []int64{40}, []bool{true}, nil, nil)
			if err != nil {
				t.Fatalf("SecretSanRun: %v", err)
			}
			if sh.Tainted() != tc.tainted {
				t.Errorf("%s: result shadow %+v, want tainted=%v", tc.fn, sh, tc.tainted)
			}
		})
	}
}

// TestSecretSanUnitStaysShared runs the classify form of the unit case: a later public branch must pass.
func TestSecretSanUnitStaysShared(t *testing.T) {
	prog := secretSanProgram(t, `import secret from std/secret

type P = {
    pub u Void
    pub s secret.SecretU8
}

type Q = {
    pub u Void
    pub b Bool
}

fn main() {
    let p = P { u: (), s: secret.SecretU8.classify(1) }
    let _u = p.u
    let q = Q { u: (), b: true }
    if q.b { print "yes" } else { print "no" }
}
`)
	var out, errOut bytes.Buffer
	code, err := vm.Run(prog, vm.Options{SecretSan: true}, &out, &errOut)
	if err != nil || code != 0 || out.String() != "yes\n" {
		t.Fatalf("exit %d, err %v, stdout %q, stderr %q", code, err, out.String(), errOut.String())
	}
}

// TestSecretSanKeepsWrites checks that labelling a container read through a secret record does not copy it,
// so the program prints the same with the checker on.
func TestSecretSanKeepsWrites(t *testing.T) {
	prog := secretSanProgram(t, `import secret from std/secret

type R = {
    pub mut items Array[u64]
    pub s secret.SecretU64
}

fn reveal(v: u64) -> u64 {
    unsafe { secret.SecretU64.classify(v).declassify() }
}

fn main() {
    let mut r = R { items: Array.of[u64](1, 2, 3), s: secret.SecretU64.classify(5) }
    r.items[0] = 9
    r.items.push(4)
    print "${reveal(r.items[0])} ${reveal(r.items.len().toU64())}"
}
`)
	for _, on := range []bool{false, true} {
		var out, errOut bytes.Buffer
		code, err := vm.Run(prog, vm.Options{SecretSan: on}, &out, &errOut)
		if err != nil || code != 0 || out.String() != "9 4\n" {
			t.Errorf("SecretSan=%v: exit %d, err %v, stdout %q, stderr %q", on, code, err, out.String(), errOut.String())
		}
	}
}

// TestSecretSanStoreLaunders is the reported program: a store into a container must not clear its label.
func TestSecretSanStoreLaunders(t *testing.T) {
	prog := secretSanProgram(t, `import secret from std/secret

fn main() {
    let mut t = FixedArray.filled[u64, 4](0)
    _ = t.set(2, 30)
    let i2 = secret.SecretIndex[4].from(secret.SecretU32.classify(2))
    let x = secret.ctLookupU64(&t, i2)
    let mut cs = "${x}".chars()
    cs[0] = 'z'
    if cs[1] == '0' { print "ends in zero" } else { print "other" }
}
`)
	var out, errOut bytes.Buffer
	code, err := vm.Run(prog, vm.Options{SecretSan: true}, &out, &errOut)
	if err != nil || code == 0 || !strings.Contains(errOut.String(), "SecretSan: secret value reaches a conditional branch") {
		t.Fatalf("exit %d, err %v, stdout %q, stderr %q", code, err, out.String(), errOut.String())
	}
}
