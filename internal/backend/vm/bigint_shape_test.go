package vm

import (
	"bytes"
	"io"
	"sort"
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// bigintShapeSource drives every modular kernel with a value that only exists
// at run time, so the interpreter cannot fold it. Each argument has the same
// length, which keeps the argument decoding itself the same shape.
const bigintShapeSource = `import bigint from std/crypto/bigint
import secret from std/secret

fn n1(v: u64) -> secret.SecretNat[1] {
    secret.SecretNat[1].fromLimbs(FixedArray.filled[u64, 1](v))
}

fn shape(v: u64) -> u64 {
    let m = bigint.modulus[1](bigint.Nat.fromU64(97)).expect("m")
    let a = m.mul(n1(v.wrappingAdd(2)), n1(v.wrappingAdd(3)))
    let b = m.pow(a, n1(v.wrappingAdd(4)))
    let c = m.inv(b)
    let d = m.add(c, a)
    let e = m.sub(d, b)
    (unsafe { e.declassify() })[0]
}

fn main() {
    let arg = host.args().get(1) || "1"
    let bytes = arg.toBytes()
    let v = (bytes.get(0) || 48).toU64().wrappingSub(48)
    print "${shape(v)}"
}
`

// TestBigintKernelsHaveFixedShape runs the same modular program over several
// secret operands and requires the interpreter to execute the same number of
// steps each time. A secret-dependent branch, early exit or variable-length
// loop in mul/pow/inv/add/sub would show up as a different count.
func TestBigintKernelsHaveFixedShape(t *testing.T) {
	pkgs := map[string]*sem.Package{}
	for _, p := range semfixture.LoadStd(t, "../../../std") {
		pkgs[p.Path] = p
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(bigintShapeSource)))
	pkgs["main"] = &sem.Package{Path: "main", Files: []*syntax.Tree{tree}, Entry: tree}
	var paths []string
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	mod := &sem.Module{}
	for _, p := range paths {
		mod.Packages = append(mod.Packages, pkgs[p])
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	if prog.HasErrors() {
		t.Fatal("ownership errors")
	}
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}

	// Executed MIR blocks and object-relative indices are a stricter shape than
	// a step count: a secret-dependent branch or index would show up as a
	// different block or index sequence even when the amount of work matches.
	for _, pair := range [][2]int64{{1, 2}, {2, 3}, {1, 5}, {4, 9}} {
		wantBlocks, wantIndices := blockAndIndexShape(t, prog, "shape", pair[0])
		gotBlocks, gotIndices := blockAndIndexShape(t, prog, "shape", pair[1])
		if len(wantBlocks) != len(gotBlocks) {
			t.Fatalf("shape(%d) executed %d blocks, shape(%d) executed %d", pair[0], len(wantBlocks), pair[1], len(gotBlocks))
		}
		for i := range wantBlocks {
			if wantBlocks[i] != gotBlocks[i] {
				t.Fatalf("shape(%d) and shape(%d) differ at block %d: %s#%d vs %s#%d",
					pair[0], pair[1], i, wantBlocks[i].fn, wantBlocks[i].blk, gotBlocks[i].fn, gotBlocks[i].blk)
			}
		}
		if len(wantIndices) != len(gotIndices) {
			t.Fatalf("shape(%d) touched %d indices, shape(%d) touched %d", pair[0], len(wantIndices), pair[1], len(gotIndices))
		}
		for i := range wantIndices {
			if wantIndices[i] != gotIndices[i] {
				t.Fatalf("shape(%d) and shape(%d) differ at index %d: %s %s[%d] vs %s %s[%d]",
					pair[0], pair[1], i, wantIndices[i].op, kindName(wantIndices[i].kind), wantIndices[i].index,
					gotIndices[i].op, kindName(gotIndices[i].kind), gotIndices[i].index)
			}
		}
	}

	var want int64
	var wantOut string
	for i, arg := range []string{"1", "2", "3", "4", "5"} {
		var out, errOut bytes.Buffer
		code, steps, err := RunSandboxedPool(prog, Options{Args: []string{"main", arg}, Cwd: t.TempDir()}, &out, &errOut)
		if err != nil || code != 0 {
			t.Fatalf("arg %s: vm: %v (exit %d)\n%s", arg, err, code, errOut.String())
		}
		if out.Len() == 0 {
			t.Fatalf("arg %s: no output", arg)
		}
		if i == 0 {
			want, wantOut = steps, out.String()
			continue
		}
		if steps != want {
			t.Errorf("arg %s executed %d steps, arg 1 executed %d: a kernel's work depends on its operand", arg, steps, want)
		}
		if out.String() != wantOut && steps == want {
			// Distinct operands may coincidentally agree, but never because the
			// shape changed; the count assertion above is the one that matters.
			t.Logf("arg %s produced %q", arg, out.String())
		}
	}
}

// blockAndIndexShape runs fn on v with the two-run trace enabled and returns
// the executed blocks and object-relative indices.
func blockAndIndexShape(t *testing.T, prog *mir.Program, fn string, v int64) ([]traceBlock, []traceIndex) {
	t.Helper()
	m, f, args, err := runFunction(prog, Options{Trace: true}, fn, []int64{v}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.catch(func() { m.call(f, args) }); err != nil {
		t.Fatal(err)
	}
	if m.trace == nil {
		t.Fatal("trace mode recorded nothing")
	}
	return append([]traceBlock(nil), m.trace.blocks...), append([]traceIndex(nil), m.trace.indices...)
}
