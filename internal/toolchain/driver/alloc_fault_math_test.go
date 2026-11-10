package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

// The inputs reach every branch of std/math: each special value, every table row, every word of 2/pi.
const mathNoAllocSrc = `import math from std/math

fn special(i: Int) -> Float {
    match i {
        0 -> 0.0
        1 -> -0.0
        2 -> 5e-324
        3 -> 1e-310
        4 -> 1e-20
        5 -> 1e-10
        6 -> 0.2
        7 -> 0.3
        8 -> 0.4
        9 -> 0.6
        10 -> 0.7
        11 -> 0.75
        12 -> 1.0
        13 -> 1.0000001
        14 -> 1.5
        15 -> 1.5707963267948966
        16 -> 2.0
        17 -> 2.3
        18 -> 3.0
        19 -> 4.7
        20 -> 6.2
        21 -> 7.8
        22 -> 10.0
        23 -> 21.0
        24 -> 40.0
        25 -> 100.0
        26 -> 1000000.0
        27 -> 10000000000.0
        28 -> 1e22
        29 -> 1e300
        30 -> 1.7976931348623157e308
        31 -> 700.0
        32 -> 709.9
        33 -> 711.0
        34 -> -745.0
        35 -> -746.0
        36 -> math.inf()
        37 -> math.nan()
        _ -> math.fromBits(0x7FF0000000000001)
    }
}

fn unary(x: Float) -> u64 {
    let a = math.toBits(math.sin(x)) ^ math.toBits(math.cos(x)) ^ math.toBits(math.tan(x)) ^ math.toBits(math.exp(x))
    let b = math.toBits(math.log(x)) ^ math.toBits(math.log2(x)) ^ math.toBits(math.log10(x)) ^ math.toBits(math.sinh(x))
    let c = math.toBits(math.cosh(x)) ^ math.toBits(math.tanh(x)) ^ math.toBits(math.trunc(x)) ^ math.toBits(math.fract(x))
    let d = math.toBits(math.floor(x)) ^ math.toBits(math.ceil(x)) ^ math.toBits(math.round(x)) ^ math.toBits(math.sqrt(x))
    let e = math.toBits(math.abs(x)) ^ math.toBits(math.min(x, 1.0)) ^ math.toBits(math.max(x, 1.0)) ^ math.toBits(math.clamp(x, 0.0, 2.0))
    let f = math.toBits(math.fmod(x, 3.0)) ^ math.toBits(math.fmod(1e300, x)) ^ math.toBits(math.fmod(x, 1e-300)) ^ math.toBits(math.fmod(1.5e-323, x))
    let g = math.toBits(math.pow(x, 2.5)) ^ math.toBits(math.pow(x, -3.0)) ^ math.toBits(math.pow(2.0, x)) ^ math.toBits(math.pow(-x, 3.0))
    let h = math.toBits(math.pow(-x, 0.5)) ^ math.toBits(math.pow(x, 1e-70)) ^ math.toBits(math.pow(x, 1e300)) ^ math.toBits(math.pow(0.5, x))
    let tests: u64 = if math.isNaN(x) || math.isInf(x) || math.isFinite(x) {
        1
    } else {
        2
    }
    a ^ b ^ c ^ d ^ e ^ f ^ g ^ h ^ tests ^ math.toBits(math.toFloat(math.toInt(x) || 0)) ^ unaryNegative(-x)
}

fn unaryNegative(x: Float) -> u64 {
    math.toBits(math.sin(x)) ^ math.toBits(math.cos(x)) ^ math.toBits(math.tan(x)) ^ math.toBits(math.exp(x)) ^ math.toBits(math.sinh(x)) ^ math.toBits(math.tanh(x)) ^ math.toBits(math.log10(x))
}

fn main() {
    let mut h: u64 = 0
    eprint "armed"
    for i in 0..39 {
        h = h ^ unary(special(i))
    }
    for k in 0..200 {
        let x = 0.69 + k.toFloat() * 0.0037
        h = h ^ math.toBits(math.log(x)) ^ math.toBits(math.log2(x)) ^ math.toBits(math.pow(x, 2.5))
    }
    for k in 0..300 {
        h = h ^ math.toBits(math.exp(k.toFloat() * 0.0027))
    }
    for k in 0..160 {
        h = h ^ math.toBits(math.sin(math.fromBits(((k * 6 + 1043).truncate[u64]() << 52) | 0x3456789ABCDEF)))
    }
    for k in 0..40 {
        h = h ^ math.toBits(math.pow(2.0, 1000.0 + k.toFloat() * 2.5)) ^ math.toBits(math.pow(2.0, -1000.0 - k.toFloat() * 2.5))
    }
    if h != 1 {
        print "done"
    }
}
`

// The std/math functions are noalloc: refusing the first allocation after arming must not stop them on either backend.
func TestStdMathAllocatesNothing(t *testing.T) {
	t.Parallel()
	m := loadSweepModule(t, mathNoAllocSrc, "")
	a, err := driver.Analyze(m)
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v\n%s", err, a.Res.Render())
	}
	var out, errOut bytes.Buffer
	code, err := vm.Run(a.Prog, vm.Options{FaultInjectAt: 1, FaultArmOnEprint: true}, &out, &errOut)
	if err != nil || code != 0 || out.String() != "done\n" {
		t.Errorf("vm: exit %d err %v stdout %q stderr %q, want done with no refused allocation", code, err, out.String(), errOut.String())
	}
	if testCompiler().CCompiler() == nil {
		return
	}
	cmd := exec.Command(buildFaultInjected(t, loadSweepModule(t, mathNoAllocSrc, "")))
	cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_ARM=1", "KIGUMI_FAIL_ALLOC_AT=1")
	var nout, nerr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &nout, &nerr
	if code := exitCodeOf(cmd.Run()); code != 0 || nout.String() != "done\n" {
		t.Errorf("native: exit %d stdout %q stderr %q, want done with no refused allocation", code, nout.String(), nerr.String())
	}
}
