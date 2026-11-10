package vm_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

const secretSanDebugSrc = `import secret from std/secret

type P = {
    pub a u64
}

// debug renders a nominal secret with its private fields hidden, and the text it returns keeps their label.
fn nominal(a: u8) -> String {
    debug(&secret.SecretU8.classify(a))
}

// A container's rendering reads everything reachable from it, so the text stays labelled.
fn container(a: u64) -> String {
    debug(&Array.of[u64](1, a))
}

fn record(a: u64) -> String {
    debug(&P { a: a })
}

fn leak(a: u8) {
    print debug(&secret.SecretU8.classify(a))
}

fn main() {
    _ = nominal(1)
    _ = container(2)
    _ = record(3)
    leak(4)
}
`

// TestSecretSanDebugPoisonsHostOutput: redaction hides a secret's value but the rendered text still carries the
// label, so printing it is a host I/O sink on both a nominal secret and a container that holds one.
func TestSecretSanDebugPoisonsHostOutput(t *testing.T) {
	prog := secretSanProgram(t, secretSanDebugSrc)
	for _, fn := range []string{"nominal", "container", "record"} {
		t.Run(fn, func(t *testing.T) {
			sh, err := vm.SecretSanRun(prog, vm.Options{}, fn, []int64{7}, []bool{true}, nil, nil)
			if err != nil {
				t.Fatalf("%s: debug of a secret needs no sink of its own: %v", fn, err)
			}
			if !sh.Tainted() {
				t.Errorf("%s: the rendering lost the secret label", fn)
			}
			if _, err := vm.SecretSanRun(prog, vm.Options{}, fn, []int64{7}, []bool{false}, nil, nil); err != nil {
				t.Errorf("%s with a public input: %v", fn, err)
			}
		})
	}
	if _, err := vm.SecretSanRun(prog, vm.Options{}, "leak", []int64{4}, []bool{true}, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "SecretSan: secret value reaches host I/O") {
		t.Fatalf("printing a secret's rendering gave %v, want the host I/O sink", err)
	}
}

// TestSecretSanDebugRedaction checks the plain rendering the sinks see: a nominal secret's value never appears in
// the text, so the only way out is the poisoned result, not the string itself.
func TestSecretSanDebugRedaction(t *testing.T) {
	prog := secretSanProgram(t, `import secret from std/secret

fn main() {
    print debug(&secret.SecretU32.classify(42))
}
`)
	var out, errOut bytes.Buffer
	code, err := vm.Run(prog, vm.Options{}, &out, &errOut)
	if err != nil || code != 0 {
		t.Fatalf("vm: %v (exit %d)\n%s", err, code, errOut.String())
	}
	if got, want := out.String(), "SecretU32 { v: <private> }\n"; got != want {
		t.Errorf("stdout %q, want %q", got, want)
	}
}
