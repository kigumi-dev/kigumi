package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

const inlineScalarSrc = `
import math from std/math

fn small(a: i8, b: i8, n: usize) -> i8 {
    -(a + b - a * b / b % b) << n >> n ^ ~a
}

fn word(a: u32, b: u32) -> u32 {
    a.wrappingMul(b).rotateRight(b).wrappingShiftRight(b) + a % b
}

fn wide(a: u64, b: Int) -> Int {
    (a.toInt() || b) - b.wrappingShiftLeft(b)
}

fn conv(x: Float, n: Int) -> Int {
    (x.toInt() || 0) + (n.toU8() || 0).toInt() + n.truncate[i16]().toInt()
}

fn letter(n: Int) -> Int {
    (Char.fromInt(n) || 'a').toInt()
}

fn root(x: Float, y: Float) -> Float {
    math.sqrt(x) + math.floor(y) + math.ceil(x) + math.round(y) + math.pow(x, y)
}

fn big(a: i128, b: i128) -> i128 {
    a * b / b % a
}

fn ubig(a: u128, b: u128) -> u128 {
    a * b / b % a
}

fn main() -> Void! {}
`

var runtimeCall = regexp.MustCompile(`call [^@]*@(rt_[a-z0-9_]+)\(`)

func TestScalarArithmeticLowersToInlineIR(t *testing.T) {
	ir := emitIR(t, inlineScalarSrc)
	for _, fn := range []string{"main.small", "main.word", "main.wide", "main.conv", "main.letter", "main.root", "main.big", "main.ubig"} {
		body := fnBody(t, ir, fn)
		for _, m := range runtimeCall.FindAllStringSubmatch(body, -1) {
			if m[1] != "rt_panic" && m[1] != "rt_unit" && m[1] != "rt_release" {
				t.Errorf("%s calls @%s; its arithmetic, conversions and math must be inline IR:\n%s", fn, m[1], body)
			}
		}
	}
	for _, want := range []string{
		"@llvm.ssub.with.overflow.i8(", "@llvm.smul.with.overflow.i8(", "@llvm.fshr.i32(", "@llvm.sqrt.f64(", "@pow(double",
		"fptosi double", "@kg.smulo128(", "@kg.sdivmod128(", "@kg.umulo128(", "@kg.udivmod128(",
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("the module lacks %s", want)
		}
	}
	if !regexp.MustCompile(`call double @pow\([^)]*\) nobuiltin`).MatchString(fnBody(t, ir, "main.root")) {
		t.Error("main.root must call libm pow as nobuiltin")
	}
	for _, helper := range []string{"@kg.udivmod128(", "@kg.umulo128("} {
		if n := strings.Count(ir, "define internal i128 "+helper) + strings.Count(ir, "define internal { i128, i1 } "+helper); n != 1 {
			t.Errorf("%s is defined %d times, want once", helper, n)
		}
	}
}

func TestIntrinsicsAreDeclaredOnlyWhenCalled(t *testing.T) {
	ir := emitIR(t, "fn add(a: u16, b: u16) -> u16 {\n    a + b\n}\n\nfn main() -> Void! {}\n")
	if !strings.Contains(ir, "declare { i16, i1 } @llvm.uadd.with.overflow.i16(i16, i16)") {
		t.Error("the u16 add's intrinsic is not declared")
	}
	for _, bad := range []string{"@llvm.sadd.with.overflow", "@llvm.fshl", "@llvm.sqrt", "@kg.udivmod128", "@kg.umulo128"} {
		if strings.Contains(ir, bad) {
			t.Errorf("the module names %s, which nothing calls", bad)
		}
	}
}
