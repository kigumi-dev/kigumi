package driver_test

import (
	"bytes"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// simdLane is one lane type the parity fixture exercises. The fixture prints
// every result as decimal lane bits (integers) or the f64 bits of the lane
// widened from f32, so the reference below can recompute it independently of
// both engines.
type simdLane struct {
	name   string
	lanes  int
	bits   uint
	signed bool
	float  bool
}

var simdIntLanes = []simdLane{
	{name: "i8", lanes: 16, bits: 8, signed: true},
	{name: "u8", lanes: 16, bits: 8},
	{name: "i16", lanes: 8, bits: 16, signed: true},
	{name: "u16", lanes: 8, bits: 16},
	{name: "i32", lanes: 4, bits: 32, signed: true},
	{name: "u32", lanes: 4, bits: 32},
	{name: "i64", lanes: 2, bits: 64, signed: true},
	{name: "u64", lanes: 2, bits: 64},
}

var simdFloatLanes = []simdLane{
	{name: "f32", lanes: 4, bits: 32, float: true},
	{name: "f64", lanes: 2, bits: 64, float: true},
}

// simdOps are the public Vec operations the fixture must exercise on every
// lane type, one output line per lane. roundtrip covers fromArray, toArray and
// lane; setlane covers setLane. A missing (type, op) pair or lane index makes
// the fixture stale and fails the test.
//
// std/simd's whole public surface is Vec plus splat, fromArray, toArray, lane,
// setLane, add, sub and mul: it has no shift, saturating, mask, reduction or
// conversion operation, so this table is exhaustive rather than a sample.
// Arithmetic that would produce a NaN is left out because IEEE-754 leaves a
// NaN payload unspecified; the reference still pins NaN-ness if one appears.
var simdOps = []string{"splat", "roundtrip", "setlane", "add", "sub", "mul"}

// simdComp maps the composition labels to the lane value the fixture must
// print: a Vec through a concrete function, a record field, and an Array
// element (splat/fromArray/add/toArray all appear in those paths too).
var simdComp = map[string]string{
	"call":    "12",
	"wrapper": "200",
	"field":   "9",
	"array":   "10",
}

// TestSimdExhaustiveVmNativeParity runs the exhaustive std/simd table through
// the VM and the native backend and checks every line against a scalar
// two's-complement/IEEE-754 reference. VM and native must agree byte for byte,
// and both must reproduce the fixture's golden stdout.
func TestSimdExhaustiveVmNativeParity(t *testing.T) {
	path := fixturePath(t, "run/simd_parity")
	root, wantOut, wantErr, wantExit := loadRunFixture(t, path)
	if wantExit != 0 || wantErr != "" {
		t.Fatalf("fixture %s: want exit 0 and no stderr, got exit %d, stderr %q", path, wantExit, wantErr)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var vmOut, vmErr bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &vmOut, &vmErr, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("vm run: err %v exit %d\nstderr:\n%s", err, code, vmErr.String())
	}
	if vmOut.String() != wantOut {
		t.Fatalf("vm output differs from the fixture golden\n--- got ---\n%s--- want ---\n%s", vmOut.String(), wantOut)
	}
	checkSimdScalarReference(t, vmOut.String())

	if testCompiler().CCompiler() == nil {
		t.Log("native engine skipped: no C compiler")
		return
	}
	exe := filepath.Join(root, "simd-parity")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("native build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = root
	cmd.Env = os.Environ()
	var nativeOut, nativeErr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &nativeOut, &nativeErr
	if err := cmd.Run(); err != nil {
		t.Fatalf("native run: %v\nstderr:\n%s", err, nativeErr.String())
	}
	if nativeErr.Len() != 0 {
		t.Fatalf("native stderr:\n%s", nativeErr.String())
	}
	if nativeOut.String() != vmOut.String() {
		t.Fatalf("native output differs from the VM\n--- vm ---\n%s--- native ---\n%s", vmOut.String(), nativeOut.String())
	}
	if nativeOut.String() != wantOut {
		t.Fatalf("native output differs from the fixture golden\n--- got ---\n%s--- want ---\n%s", nativeOut.String(), wantOut)
	}
}

// checkSimdScalarReference parses the fixture output and recomputes every
// result with scalar integer wraparound and Go's IEEE-754 arithmetic. The
// operand values are read from the output itself, so the reference never
// duplicates the fixture's tables.
func checkSimdScalarReference(t *testing.T, out string) {
	t.Helper()
	lanes := map[string]simdLane{}
	for _, l := range simdIntLanes {
		lanes[l.name] = l
	}
	for _, l := range simdFloatLanes {
		lanes[l.name] = l
	}
	seen := map[string]map[int]bool{}
	compSeen := map[string]bool{}
	for n, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		fields := strings.Split(line, " ")
		if len(fields) != 6 {
			t.Fatalf("line %d: %q has %d fields, want 6", n+1, line, len(fields))
		}
		name, op, idxText, a, b, got := fields[0], fields[1], fields[2], fields[3], fields[4], fields[5]
		if name == "comp" {
			want, ok := simdComp[op]
			if !ok {
				t.Fatalf("line %d: unknown composition %q", n+1, line)
			}
			if compSeen[op] {
				t.Fatalf("line %d: duplicate composition %q", n+1, op)
			}
			if got != want {
				t.Fatalf("line %d: %q printed %q, want %q", n+1, line, got, want)
			}
			compSeen[op] = true
			continue
		}
		lt, ok := lanes[name]
		if !ok {
			t.Fatalf("line %d: unknown lane type in %q", n+1, line)
		}
		idx, err := strconv.Atoi(idxText)
		if err != nil || idx < 0 || idx >= lt.lanes {
			t.Fatalf("line %d: lane index %q out of range for %s[%d]", n+1, idxText, name, lt.lanes)
		}
		if lt.float {
			checkSimdFloatLine(t, n+1, lt, op, a, b, got)
		} else {
			checkSimdIntLine(t, n+1, lt, op, a, b, got)
		}
		key := name + " " + op
		if seen[key] == nil {
			seen[key] = map[int]bool{}
		}
		// Several operand sets repeat a lane index on purpose; coverage
		// below only requires every index to appear at least once.
		seen[key][idx] = true
	}
	for _, lt := range append(append([]simdLane{}, simdIntLanes...), simdFloatLanes...) {
		for _, op := range simdOps {
			key := lt.name + " " + op
			if len(seen[key]) != lt.lanes {
				t.Errorf("%s: %d of %d lanes checked", key, len(seen[key]), lt.lanes)
				continue
			}
			for i := range lt.lanes {
				if !seen[key][i] {
					t.Errorf("%s: lane %d missing", key, i)
				}
			}
		}
	}
	for label := range simdComp {
		if !compSeen[label] {
			t.Errorf("composition %q missing", label)
		}
	}
}

func simdMask(bits uint) uint64 {
	if bits >= 64 {
		return math.MaxUint64
	}
	return uint64(1)<<bits - 1
}

// simdSignExtend64 sets the high bits of a wrapped lane to its sign bit, the
// same representation ParseInt gives a signed lane's decimal output.
func simdSignExtend64(v uint64, bits uint) uint64 {
	if bits >= 64 {
		return v
	}
	if v&(uint64(1)<<(bits-1)) != 0 {
		return v | ^simdMask(bits)
	}
	return v
}

func simdIntBits(t *testing.T, n int, lt simdLane, s string) uint64 {
	t.Helper()
	if lt.signed {
		v, err := strconv.ParseInt(s, 10, int(lt.bits))
		if err != nil {
			t.Fatalf("line %d: %q is not a %s value: %v", n, s, lt.name, err)
		}
		return uint64(v)
	}
	v, err := strconv.ParseUint(s, 10, int(lt.bits))
	if err != nil {
		t.Fatalf("line %d: %q is not a %s value: %v", n, s, lt.name, err)
	}
	return v
}

func checkSimdIntLine(t *testing.T, n int, lt simdLane, op, a, b, got string) {
	t.Helper()
	abits := simdIntBits(t, n, lt, a)
	rbits := simdIntBits(t, n, lt, got)
	mask := simdMask(lt.bits)
	switch op {
	case "splat":
		if b != "-" {
			t.Fatalf("line %d: splat has a second operand %q", n, b)
		}
		if abits != rbits {
			t.Fatalf("line %d: %s splat lane is %s, want %s", n, lt.name, got, a)
		}
	case "setlane", "roundtrip":
		bbits := simdIntBits(t, n, lt, b)
		if abits != bbits || abits != rbits {
			t.Fatalf("line %d: %s %s carried %s/%s but read back %s", n, lt.name, op, a, b, got)
		}
	case "add", "sub", "mul":
		bbits := simdIntBits(t, n, lt, b)
		var x uint64
		switch op {
		case "add":
			x = (abits + bbits) & mask
		case "sub":
			x = (abits - bbits) & mask
		default:
			x = (abits * bbits) & mask
		}
		want := x
		if lt.signed {
			want = simdSignExtend64(x, lt.bits)
		}
		if rbits != want {
			t.Fatalf("line %d: %s %s %s, %s wrapped to %s, want %d", n, lt.name, op, a, b, got, int64(want))
		}
	default:
		t.Fatalf("line %d: unknown integer op %q", n, op)
	}
}

func simdFloatBits(t *testing.T, n int, lt simdLane, s string) (uint64, float64) {
	t.Helper()
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		t.Fatalf("line %d: %q is not a float bit pattern: %v", n, s, err)
	}
	f := math.Float64frombits(u)
	if lt.bits == 32 && !math.IsNaN(f) && float64(float32(f)) != f {
		t.Fatalf("line %d: %s lane %q is not an exact widening of an f32", n, lt.name, s)
	}
	return u, f
}

func checkSimdFloatLine(t *testing.T, n int, lt simdLane, op, a, b, got string) {
	t.Helper()
	abits, af := simdFloatBits(t, n, lt, a)
	rbits, rf := simdFloatBits(t, n, lt, got)
	switch op {
	case "splat":
		if b != "-" {
			t.Fatalf("line %d: splat has a second operand %q", n, b)
		}
		if abits != rbits {
			t.Fatalf("line %d: %s splat lane is %s, want %s", n, lt.name, got, a)
		}
	case "setlane", "roundtrip":
		bbits, _ := simdFloatBits(t, n, lt, b)
		if abits != bbits || abits != rbits {
			t.Fatalf("line %d: %s %s carried %s/%s but read back %s", n, lt.name, op, a, b, got)
		}
	case "add", "sub", "mul":
		_, bf := simdFloatBits(t, n, lt, b)
		var want float64
		if lt.bits == 32 {
			x, y := float32(af), float32(bf)
			switch op {
			case "add":
				want = float64(x + y)
			case "sub":
				want = float64(x - y)
			default:
				want = float64(x * y)
			}
		}
		if lt.bits == 64 {
			switch op {
			case "add":
				want = af + bf
			case "sub":
				want = af - bf
			default:
				want = af * bf
			}
		}
		if math.IsNaN(want) || math.IsNaN(rf) {
			// IEEE-754 leaves a NaN's payload unspecified, so only NaN-ness is
			// pinned; every operand table here avoids producing one.
			if !math.IsNaN(want) || !math.IsNaN(rf) {
				t.Fatalf("line %d: %s %s %s, %s produced %s, want %d", n, lt.name, op, a, b, got, math.Float64bits(want))
			}
			return
		}
		if math.Float64bits(want) != rbits {
			t.Fatalf("line %d: %s %s %s, %s produced %s, want %d", n, lt.name, op, a, b, got, math.Float64bits(want))
		}
	default:
		t.Fatalf("line %d: unknown float op %q", n, op)
	}
}
