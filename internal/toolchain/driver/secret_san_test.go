package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

// TestSecretSanThroughDriver runs the SecretSan hooks on a module the driver loaded and analyzed,
// the program the CLI would run.
func TestSecretSanThroughDriver(t *testing.T) {
	const src = `import secret from std/secret

// classify -> ctLt/select -> declassify: the constant-time shape.
fn ct_choose(a: u8, b: u8) -> u8 {
    let x = secret.SecretU8.classify(a)
    let y = secret.SecretU8.classify(b)
    let lt = x.ctLt(y)
    (unsafe { lt.selectU8(x, y).declassify() })
}

// Two runs with the same public input and different secret inputs trace alike.
fn ct_trace(a: u32, b: u32) -> u32 {
    let x = secret.SecretU32.classify(a)
    let y = secret.SecretU32.classify(b)
    let lt = x.ctLt(y)
    (unsafe { lt.selectU32(x, y).declassify() })
}

// A secret-dependent branch must fail the sink check and split the trace.
fn leak_branch(s: u64) -> u64 {
    if s > 3 { 1 } else { 2 }
}

fn leak_trace_branch(a: u64, b: u64) -> u64 {
    if a < b { 1 } else { 2 }
}

fn main() {
    ct_choose(5, 3)
    ct_trace(1, 2)
    leak_branch(1)
    leak_trace_branch(1, 2)
}
`
	m := loadSweepModule(t, src, "")
	a, err := driver.Analyze(m)
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v", err)
	}

	if sh, err := vm.SecretSanRun(a.Prog, vm.Options{}, "ct_choose", []int64{5, 3}, []bool{false, false}, nil, nil); err != nil {
		t.Fatalf("constant-time function failed the sinks: %v", err)
	} else if sh.Tainted() {
		t.Errorf("declassified result is shadowed: %+v", sh)
	}

	if _, err := vm.SecretSanRun(a.Prog, vm.Options{}, "leak_branch", []int64{9}, []bool{true}, nil, nil); err == nil {
		t.Fatal("secret-dependent branch passed the sink check")
	} else if !strings.Contains(err.Error(), "SecretSan:") {
		t.Fatalf("diagnostic %q lacks the SecretSan prefix", err)
	}

	if err := vm.CompareTraces(a.Prog, vm.Options{}, "ct_trace", []int64{1, 2}, []int64{1, 3}, nil, nil); err != nil {
		t.Fatalf("constant-time function traced differently: %v", err)
	}
	if err := vm.CompareTraces(a.Prog, vm.Options{}, "leak_trace_branch", []int64{1, 2}, []int64{2, 1}, nil, nil); err == nil {
		t.Fatal("branch on a secret produced equal traces")
	} else if !strings.Contains(err.Error(), "executed blocks") {
		t.Fatalf("branch mismatch reported %q", err)
	}
}

// TestSecretSanRefusesNativeOnlyPrograms keeps --secret-san from being answered with `kigumi build`: the secret
// labels live only in the VM, so a program it cannot run is unchecked and the check must fail closed instead.
func TestSecretSanRefusesNativeOnlyPrograms(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	mod := "Module {\n    name: \"secretsanasm\"\n    kigumi: \"0.1\"\n}\n"
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte(mod), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(asmNumericLabelSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if _, err := driver.RunWith(m, driver.RunOptions{SecretSan: true}, &out, &errOut, nil); err == nil ||
		!strings.Contains(err.Error(), "--secret-san cannot check this program") ||
		!strings.Contains(err.Error(), "no shadow checker") {
		t.Fatalf("want a fail-closed --secret-san refusal, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("a refused run wrote %q", out.String())
	}

	// Without the flag the native-only guidance is unchanged.
	if _, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, nil); err == nil ||
		!strings.Contains(err.Error(), "native-only") {
		t.Fatalf("plain refusal changed: %v", err)
	}
}
