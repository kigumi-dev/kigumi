package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// TestSecretPrimitivesHaveNoSecretBranches compiles the generated IR with clang or zig cc
// and checks the lowered secret functions for conditional branches and division on three targets.
func TestSecretPrimitivesHaveNoSecretBranches(t *testing.T) {
	t.Parallel()
	cc := ctCompiler()
	if len(cc) == 0 {
		t.Skip("neither clang nor zig is on PATH")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"ct\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(`import secret from std/secret
import aead from std/crypto/chacha20poly1305
import sha256 from std/crypto/sha256

fn secret_safe(a: secret.SecretU32, b: secret.SecretU32) -> secret.SecretU32 {
    let lt = a.ctLt(b)
    lt.selectU32(a, b)
}

let x = secret.SecretU32.classify(1)
let y = secret.SecretU32.classify(2)
let q = aead.secretQuarterRound(x, y, x, y)
let p = aead.secretBlock(x, y, x, y, x, y, x, y, x, y, x, y, x, y, x, y, x, y, x)
let r = sha256.secretSha256Round(x, y, x, y, x, y, x, y, x, y)
let buf = secret.SecretBuffer.fromString("shape")
let eq = secret.secretEqPublic(&buf, &Bytes.fill(1, 5))
let qv = unsafe { q.a.declassify() }
let pv = unsafe { p.h0.declassify() }
let rv = unsafe { r.a.declassify() }
print "${(unsafe { secret_safe(x, y).declassify() })} ${qv} ${pv} ${rv} ${(unsafe { eq.declassify() })}"
`), 0o644)
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
	irFile := filepath.Join(root, "main.ll")
	os.WriteFile(irFile, []byte(ir), 0o644)

	triples := []string{"x86_64-linux-gnu", "aarch64-linux-gnu", "riscv64-linux-gnu"}
	bad := map[string][]string{
		"x86_64-linux-gnu":  {"\tj", "\tdiv", "\tidiv"},
		"aarch64-linux-gnu": {"\tb.", "\tcbz", "\tcbnz", "\ttbz", "\ttbnz", "\tudiv", "\tsdiv"},
		"riscv64-linux-gnu": {"\tbeq", "\tbne", "\tblt", "\tbge", "\tbltu", "\tbgeu", "\tbeqz", "\tbnez", "\tdiv", "\trem"},
	}
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
				funcs := ctSecretFunctions(string(data))
				if len(funcs) < 2 {
					t.Fatalf("found %d secret functions in assembly", len(funcs))
				}
				for _, want := range []string{"secretQuarterRound", "secretBlock", "secretSha256Round", "secretEqPublic"} {
					found := false
					for _, fn := range funcs {
						if strings.Contains(fn.name, want) {
							found = true
						}
					}
					if !found {
						t.Errorf("CT shape %s was not emitted; the test would not check it", want)
					}
				}
				for _, fn := range funcs {
					for _, ins := range bad[triple] {
						if strings.Contains(fn.body, ins) {
							t.Errorf("secret function %s contains %q on %s %s:\n%s", fn.name, ins, triple, level, fn.body)
						}
					}
				}
				if triple == "riscv64-linux-gnu" && level == "-O1" {
					badIR := filepath.Join(root, "bad.ll")
					os.WriteFile(badIR, []byte(`define i1 @secret_bad(i1 %c, i1 %a, i1 %b) {
entry:
  br i1 %c, label %t, label %f
t:
  ret i1 %a
f:
  ret i1 %b
}
`), 0o644)
					badAsm := filepath.Join(root, "bad.s")
					if out, err := ctAssemble(cc, "-O0", triple, badIR, badAsm); err != nil {
						t.Fatalf("negative control: %v\n%s", err, out)
					}
					raw, _ := os.ReadFile(badAsm)
					detected := false
					for _, fn := range ctSecretFunctions(string(raw)) {
						for _, ins := range bad[triple] {
							if strings.Contains(fn.body, ins) {
								detected = true
							}
						}
					}
					if !detected {
						t.Fatalf("negative control branch was not detected:\n%s", raw)
					}
				}
			})
		}
	}
}

type ctAsmFunc struct{ name, body string }

var ctFuncLabel = regexp.MustCompile(`(?m)^"?[A-Za-z0-9_./$.]*secret[A-Za-z0-9_./$.]*"?:\s*(?:(?:#|//).*)?$`)

func ctSecretFunctions(asm string) []ctAsmFunc {
	var out []ctAsmFunc
	lines := strings.SplitAfter(asm, "\n")
	var cur *ctAsmFunc
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\n")
		if ctFuncLabel.MatchString(trimmed) && !strings.Contains(trimmed, ".type.") {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &ctAsmFunc{name: strings.TrimSuffix(strings.TrimSpace(trimmed), ":"), body: line}
			continue
		}
		if cur != nil {
			cur.body += line
			if strings.Contains(line, ".cfi_endproc") {
				out = append(out, *cur)
				cur = nil
			}
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	// A label such as `std/secret.SecretBuffer.fromString` contains the
	// package path but not a secret-named function; only the leaf counts, so
	// allocating helpers in `std/secret` are not mistaken for CT primitives.
	kept := out[:0]
	for _, fn := range out {
		leaf := fn.name
		if i := strings.LastIndexByte(leaf, '.'); i >= 0 {
			leaf = leaf[i+1:]
		}
		if strings.Contains(leaf, "secret") {
			kept = append(kept, fn)
		}
	}
	return kept
}

// ctAssemble lets the .ll extension select LLVM IR: zig cc rejects `-x ir`, which clang alone accepts.
func ctAssemble(cc []string, level, triple, in, out string) ([]byte, error) {
	args := append(append([]string{}, cc[1:]...), "-S", level, "-target", triple, in, "-o", out)
	return exec.Command(cc[0], args...).CombinedOutput()
}

func ctCompiler() []string {
	for _, name := range []string{"clang", "clang-21", "clang-20"} {
		if p, err := exec.LookPath(name); err == nil {
			return []string{p}
		}
	}
	for _, name := range []string{"zig", "zig-0.17.0"} {
		if p, err := exec.LookPath(name); err == nil {
			return []string{p, "cc"}
		}
	}
	return nil
}
