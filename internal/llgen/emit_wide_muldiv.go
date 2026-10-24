package llgen

import (
	"fmt"
	"strings"
)

// The 128-bit helpers use 64-bit pieces so no target needs compiler-rt's __muloti4 or __divti3 family.
const wideMin = "-170141183460469231731687303715884105728"

func (e *emitter) wideHelper(name, body string) string {
	sym := "@kg." + name
	if !e.cio[sym] {
		e.cio[sym] = true
		e.helpers.WriteString(body)
	}
	return sym
}

func (e *emitter) wideMul(sb *strings.Builder, signed bool, l, r Value) Value {
	e.intrinsic("umul.with.overflow.i64", "{ i64, i1 }", "i64", "i64")
	e.intrinsic("uadd.with.overflow.i64", "{ i64, i1 }", "i64", "i64")
	fn := e.wideHelper("umulo128", umulo128IR)
	if signed {
		fn = e.wideHelper("smulo128", smulo128IR)
	}
	pair, v, o := e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = call { i128, i1 } %s(i128 %s, i128 %s)\n", pair, fn, l.Text, r.Text)
	fmt.Fprintf(sb, "  %s = extractvalue { i128, i1 } %s, 0\n  %s = extractvalue { i128, i1 } %s, 1\n", v, pair, o, pair)
	e.overflowTrap(sb, o, "integer overflow")
	return Value{TI128, v}
}

func (e *emitter) wideDiv(sb *strings.Builder, signed, rem bool, l, r Value) Value {
	e.overflowTrap(sb, e.icmp(sb, "eq", TI128, r, Value{TI128, "0"}), "division by zero")
	fn := e.wideHelper("udivmod128", udivmod128IR)
	if signed {
		m1 := e.icmp(sb, "eq", TI128, r, Value{TI128, "-1"})
		lo := e.icmp(sb, "eq", TI128, l, Value{TI128, wideMin})
		e.overflowTrap(sb, e.binOpT(sb, "and", TI1, Value{TI1, m1}, Value{TI1, lo}), "integer overflow")
		fn = e.wideHelper("sdivmod128", sdivmod128IR)
	}
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call i128 %s(i128 %s, i128 %s, i1 %t)\n", t, fn, l.Text, r.Text, rem)
	return Value{TI128, t}
}

const umulo128IR = `
define internal { i128, i1 } @kg.umulo128(i128 %a, i128 %b) {
entry:
  %alo = trunc i128 %a to i64
  %as = lshr i128 %a, 64
  %ahi = trunc i128 %as to i64
  %blo = trunc i128 %b to i64
  %bs = lshr i128 %b, 64
  %bhi = trunc i128 %bs to i64
  %za = zext i64 %alo to i128
  %zb = zext i64 %blo to i128
  %p = mul i128 %za, %zb
  %plo = trunc i128 %p to i64
  %ps = lshr i128 %p, 64
  %phi = trunc i128 %ps to i64
  %m1 = call { i64, i1 } @llvm.umul.with.overflow.i64(i64 %ahi, i64 %blo)
  %l1 = extractvalue { i64, i1 } %m1, 0
  %o1 = extractvalue { i64, i1 } %m1, 1
  %m2 = call { i64, i1 } @llvm.umul.with.overflow.i64(i64 %alo, i64 %bhi)
  %l2 = extractvalue { i64, i1 } %m2, 0
  %o2 = extractvalue { i64, i1 } %m2, 1
  %s1 = call { i64, i1 } @llvm.uadd.with.overflow.i64(i64 %phi, i64 %l1)
  %h1 = extractvalue { i64, i1 } %s1, 0
  %c1 = extractvalue { i64, i1 } %s1, 1
  %s2 = call { i64, i1 } @llvm.uadd.with.overflow.i64(i64 %h1, i64 %l2)
  %h2 = extractvalue { i64, i1 } %s2, 0
  %c2 = extractvalue { i64, i1 } %s2, 1
  %anz = icmp ne i64 %ahi, 0
  %bnz = icmp ne i64 %bhi, 0
  %both = and i1 %anz, %bnz
  %v1 = or i1 %both, %o1
  %v2 = or i1 %v1, %o2
  %v3 = or i1 %v2, %c1
  %over = or i1 %v3, %c2
  %rh = zext i64 %h2 to i128
  %rhs = shl i128 %rh, 64
  %rl = zext i64 %plo to i128
  %r = or i128 %rhs, %rl
  %res0 = insertvalue { i128, i1 } poison, i128 %r, 0
  %res = insertvalue { i128, i1 } %res0, i1 %over, 1
  ret { i128, i1 } %res
}
`

const smulo128IR = `
define internal { i128, i1 } @kg.smulo128(i128 %a, i128 %b) {
entry:
  %an = icmp slt i128 %a, 0
  %bn = icmp slt i128 %b, 0
  %na = sub i128 0, %a
  %nb = sub i128 0, %b
  %ma = select i1 %an, i128 %na, i128 %a
  %mb = select i1 %bn, i128 %nb, i128 %b
  %u = call { i128, i1 } @kg.umulo128(i128 %ma, i128 %mb)
  %p = extractvalue { i128, i1 } %u, 0
  %o = extractvalue { i128, i1 } %u, 1
  %neg = xor i1 %an, %bn
  %np = sub i128 0, %p
  %r = select i1 %neg, i128 %np, i128 %p
  %bigneg = icmp ugt i128 %p, ` + wideMin + `
  %bigpos = icmp slt i128 %p, 0
  %big = select i1 %neg, i1 %bigneg, i1 %bigpos
  %over = or i1 %o, %big
  %res0 = insertvalue { i128, i1 } poison, i128 %r, 0
  %res = insertvalue { i128, i1 } %res0, i1 %over, 1
  ret { i128, i1 } %res
}
`
