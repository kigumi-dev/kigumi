package llgen

// The caller rules out b == 0 for udivmod128, and b == 0 and min / -1 for sdivmod128.
const udivmod128IR = `
define internal i128 @kg.udivmod128(i128 %a, i128 %b, i1 %rem) {
entry:
  %lt = icmp ult i128 %a, %b
  br i1 %lt, label %less, label %ge
less:
  %r0 = select i1 %rem, i128 %a, i128 0
  ret i128 %r0
ge:
  %ab = or i128 %a, %b
  %abh = lshr i128 %ab, 64
  %fit = icmp eq i128 %abh, 0
  br i1 %fit, label %narrow, label %notnarrow
narrow:
  %a64 = trunc i128 %a to i64
  %b64 = trunc i128 %b to i64
  %nq = udiv i64 %a64, %b64
  %nr = urem i64 %a64, %b64
  %n = select i1 %rem, i64 %nr, i64 %nq
  %nz = zext i64 %n to i128
  ret i128 %nz
notnarrow:
  %bh = lshr i128 %b, 32
  %small = icmp eq i128 %bh, 0
  br i1 %small, label %limbs, label %loop
limbs:
  %d = trunc i128 %b to i64
  %s3 = lshr i128 %a, 96
  %x3 = trunc i128 %s3 to i64
  %q3 = udiv i64 %x3, %d
  %r3 = urem i64 %x3, %d
  %s2 = lshr i128 %a, 64
  %t2 = trunc i128 %s2 to i64
  %l2 = and i64 %t2, 4294967295
  %h2 = shl i64 %r3, 32
  %x2 = or i64 %h2, %l2
  %q2 = udiv i64 %x2, %d
  %r2 = urem i64 %x2, %d
  %s1 = lshr i128 %a, 32
  %t1 = trunc i128 %s1 to i64
  %l1 = and i64 %t1, 4294967295
  %h1 = shl i64 %r2, 32
  %x1 = or i64 %h1, %l1
  %q1 = udiv i64 %x1, %d
  %r1 = urem i64 %x1, %d
  %t0 = trunc i128 %a to i64
  %l0 = and i64 %t0, 4294967295
  %h0 = shl i64 %r1, 32
  %x0 = or i64 %h0, %l0
  %q0 = udiv i64 %x0, %d
  %rl = urem i64 %x0, %d
  %z3 = zext i64 %q3 to i128
  %w3 = shl i128 %z3, 96
  %z2 = zext i64 %q2 to i128
  %w2 = shl i128 %z2, 64
  %z1 = zext i64 %q1 to i128
  %w1 = shl i128 %z1, 32
  %z0 = zext i64 %q0 to i128
  %o32 = or i128 %w3, %w2
  %o10 = or i128 %w1, %z0
  %lq = or i128 %o32, %o10
  %lr = zext i64 %rl to i128
  %lres = select i1 %rem, i128 %lr, i128 %lq
  ret i128 %lres
loop:
  %i = phi i32 [ 128, %notnarrow ], [ %i1, %loop ]
  %r = phi i128 [ 0, %notnarrow ], [ %r2x, %loop ]
  %q = phi i128 [ %a, %notnarrow ], [ %q2x, %loop ]
  %carry = icmp slt i128 %r, 0
  %top = lshr i128 %q, 127
  %rs = shl i128 %r, 1
  %r1x = or i128 %rs, %top
  %qs = shl i128 %q, 1
  %fits = icmp uge i128 %r1x, %b
  %take = or i1 %carry, %fits
  %rd = sub i128 %r1x, %b
  %r2x = select i1 %take, i128 %rd, i128 %r1x
  %bit = zext i1 %take to i128
  %q2x = or i128 %qs, %bit
  %i1 = sub i32 %i, 1
  %done = icmp eq i32 %i1, 0
  br i1 %done, label %out, label %loop
out:
  %res = select i1 %rem, i128 %r2x, i128 %q2x
  ret i128 %res
}
`

const sdivmod128IR = `
define internal i128 @kg.sdivmod128(i128 %a, i128 %b, i1 %rem) {
entry:
  %an = icmp slt i128 %a, 0
  %bn = icmp slt i128 %b, 0
  %na = sub i128 0, %a
  %nb = sub i128 0, %b
  %ma = select i1 %an, i128 %na, i128 %a
  %mb = select i1 %bn, i128 %nb, i128 %b
  %u = call i128 @kg.udivmod128(i128 %ma, i128 %mb, i1 %rem)
  %qneg = xor i1 %an, %bn
  %neg = select i1 %rem, i1 %an, i1 %qneg
  %nu = sub i128 0, %u
  %r = select i1 %neg, i128 %nu, i128 %u
  ret i128 %r
}
`
