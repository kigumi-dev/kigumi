package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// bigintCTSource exercises every modular kernel. The modulus is two limbs so
// the fixed loops are short, but the same lowering covers any N.
const bigintCTSource = `import bigint from std/crypto/bigint
import secret from std/secret

fn n(l0: u64, l1: u64) -> secret.SecretNat[2] {
    let mut a = FixedArray.filled[u64, 2](0)
    a[0] = l0
    a[1] = l1
    secret.SecretNat[2].fromLimbs(a)
}

fn main() {
    let m = bigint.modulus[2](bigint.Nat.fromLimbs(Array.of[u64](0xffffffffffffffff, 0x7fffffffffffffff))).expect("m")
    let a = n(0x123456789abcdef0, 0x0fedcba987654321)
    let b = n(0xdeadbeefcafebabe, 0x0123456789abcdef)
    let s = m.add(a, b)
    let t = m.sub(s, b)
    let p = m.mul(t, a)
    let q = m.pow(p, b)
    let r = m.inv(q)
    print "${r.limb(0)}"
}
`

var irDefine = regexp.MustCompile(`(?ms)^define [^\n]*?@"([^"]+)"[^\n]*\{\n(.*?)\n\}`)

var irDivision = regexp.MustCompile(`\b(?:u|s)(?:div|rem)\b`)

// TestBigintKernelsHaveNoDivision compiles a module that drives the modular
// kernels, then checks both the generated IR and the machine code for the
// three supported targets at O1 and O2: no division or remainder instruction
// may appear in a bigint or SecretNat function. Montgomery arithmetic and the
// fixed-iteration inverse need only masks and conditional additions.
func TestBigintKernelsHaveNoDivision(t *testing.T) {
	t.Parallel()
	cc := ctCompiler()
	if len(cc) == 0 {
		t.Skip("neither clang nor zig is on PATH")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"bigintct\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(bigintCTSource), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}

	irFuncs := bigintIRFunctions(ir)
	for _, want := range []string{
		"std/crypto/bigint.Modulus.add",
		"std/crypto/bigint.Modulus.sub",
		"std/crypto/bigint.Modulus.mul",
		"std/crypto/bigint.Modulus.pow",
		"std/crypto/bigint.Modulus.inv",
	} {
		if _, ok := irFuncs[want]; !ok {
			t.Fatalf("generated IR does not define %s", want)
		}
	}
	for name, body := range irFuncs {
		if irDivision.MatchString(body) {
			t.Errorf("%s contains a division or remainder in IR:\n%s", name, body)
		}
	}
	if t.Failed() {
		return
	}

	irFile := filepath.Join(root, "main.ll")
	if err := os.WriteFile(irFile, []byte(ir), 0o644); err != nil {
		t.Fatal(err)
	}
	triples := []string{"x86_64-linux-gnu", "aarch64-linux-gnu", "riscv64-linux-gnu"}
	for _, triple := range triples {
		for _, level := range []string{"-O1", "-O2"} {
			t.Run(triple+"/"+level, func(t *testing.T) {
				asm := filepath.Join(root, triple+level+".s")
				if out, err := ctAssemble(cc, level, triple, irFile, asm); err != nil {
					t.Fatalf("%v: %v\n%s", cc, err, out)
				}
				data, err := os.ReadFile(asm)
				if err != nil {
					t.Fatal(err)
				}
				funcs := bigintAsmFunctions(string(data))
				if len(funcs) == 0 {
					t.Fatal("no bigint or SecretNat functions in assembly")
				}
				for _, fn := range funcs {
					if ins := divisionInstruction(fn.body, triple); ins != "" {
						t.Errorf("%s contains %q on %s %s:\n%s", fn.name, ins, triple, level, fn.body)
					}
				}
			})
		}
	}

	// Negative control: a bigint-named function that does divide must be seen.
	badIR := filepath.Join(root, "bad.ll")
	if err := os.WriteFile(badIR, []byte(`define i64 @"std/crypto/bigint.bad"(i64 %a, i64 %b) {
entry:
  %q = sdiv i64 %a, %b
  ret i64 %q
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	badAsm := filepath.Join(root, "bad.s")
	if out, err := ctAssemble(cc, "-O0", "x86_64-linux-gnu", badIR, badAsm); err != nil {
		t.Fatalf("negative control: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(badAsm)
	if err != nil {
		t.Fatal(err)
	}
	detected := false
	for _, fn := range bigintAsmFunctions(string(raw)) {
		if divisionInstruction(fn.body, "x86_64-linux-gnu") != "" {
			detected = true
		}
	}
	if !detected {
		t.Fatalf("negative control division was not detected:\n%s", raw)
	}
}

func bigintIRFunctions(ir string) map[string]string {
	out := map[string]string{}
	for _, m := range irDefine.FindAllStringSubmatch(ir, -1) {
		name := m[1]
		if isBigintFunction(name) {
			out[name] = m[2]
		}
	}
	return out
}

func isBigintFunction(name string) bool {
	return strings.Contains(name, "std/crypto/bigint.") || strings.Contains(name, "std/secret.SecretNat")
}

var asmFunctionStart = regexp.MustCompile(`(?m)^[^\n]*-- Begin function (.+)$`)

// bigintAsmFunctions reads clang's function markers (a `#` comment on x86 and
// a `//` comment elsewhere) and returns the bodies of the bigint and SecretNat
// functions only.
func bigintAsmFunctions(asm string) []ctAsmFunc {
	marks := asmFunctionStart.FindAllStringSubmatchIndex(asm, -1)
	var out []ctAsmFunc
	for i, mark := range marks {
		name := strings.TrimSpace(asm[mark[2]:mark[3]])
		if !isBigintFunction(name) {
			continue
		}
		end := len(asm)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		out = append(out, ctAsmFunc{name: name, body: asm[mark[1]:end]})
	}
	return out
}

func divisionInstruction(body, triple string) string {
	switch triple {
	case "x86_64-linux-gnu":
		for _, ins := range []string{"\tdiv", "\tidiv"} {
			if strings.Contains(body, ins) {
				return ins
			}
		}
	case "aarch64-linux-gnu":
		for _, ins := range []string{"\tsdiv", "\tudiv"} {
			if strings.Contains(body, ins) {
				return ins
			}
		}
	case "riscv64-linux-gnu":
		for _, ins := range []string{"\tdiv", "\trem"} {
			if strings.Contains(body, ins) {
				return ins
			}
		}
	}
	return ""
}
