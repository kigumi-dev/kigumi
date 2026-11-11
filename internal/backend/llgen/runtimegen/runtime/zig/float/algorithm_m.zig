//! Correctly rounded decimal -> binary parsing ("Algorithm M": exact
//! rational num/den, rounded to nearest-even), shared by f64 and f32.

const bn = @import("bignum.zig");
const Bn = bn.Bn;

/// More than this many significant digits can't change a double's correctly
/// rounded result (the excess is captured by `sticky` for tie-breaking).
const max_sig_digits = 768;
/// abs(decExp10 + digit count) past this is unambiguously overflow/underflow
/// for both f32 and f64 (real boundaries sit under 330).
const exp10_shortcut = 350;

/// Scans an unsigned decimal literal at sp, filling D (significant digits,
/// capped at max_sig_digits), dec_exp (value == D * 10^dec_exp), sticky (a
/// nonzero digit was dropped past the cap) and sig_count_out.
pub fn scanUnsignedDecimal(sp: *[*c]const u8, D: *Bn, dec_exp: *i32, sticky: *c_int, sig_count_out: *i32) c_int {
    var p = sp.*;
    bn.zero(D);
    var seen_digit = false;
    var seen_nonzero = false;
    var sig_count: i32 = 0;
    var total_frac: i32 = 0;
    var dropped: i32 = 0;
    var after_point = false;
    sticky.* = 0;
    while (true) {
        const c = p[0];
        if (c == '.' and !after_point) {
            after_point = true;
            p += 1;
            continue;
        }
        if (c < '0' or c > '9') break;
        seen_digit = true;
        const dgt: i32 = @intCast(c - '0');
        if (after_point) total_frac += 1;
        if (!seen_nonzero and dgt == 0) {
            p += 1;
            continue;
        }
        seen_nonzero = true;
        if (sig_count < max_sig_digits) {
            bn.mulU32(D, 10);
            bn.addU32(D, @intCast(dgt));
            sig_count += 1;
        } else {
            dropped += 1;
            if (dgt != 0) sticky.* = 1;
        }
        p += 1;
    }
    if (!seen_digit) return 0;
    var explicit_exp: i32 = 0;
    if (p[0] == 'e' or p[0] == 'E') {
        var q = p + 1;
        var esign: i32 = 1;
        if (q[0] == '+' or q[0] == '-') {
            esign = if (q[0] == '-') -1 else 1;
            q += 1;
        }
        if (q[0] >= '0' and q[0] <= '9') {
            var val: i64 = 0;
            while (q[0] >= '0' and q[0] <= '9') : (q += 1) {
                if (val < 2000000000) val = val * 10 + @as(i64, @intCast(q[0] - '0'));
            }
            p = q;
            val *= esign;
            explicit_exp = @intCast(if (val > 1000000000) 1000000000 else if (val < -1000000000) -1000000000 else val);
        }
    }
    dec_exp.* = -total_frac + dropped + explicit_exp;
    sig_count_out.* = sig_count;
    sp.* = p;
    return 1;
}

/// Case-insensitive literal match; returns its length, or 0 on mismatch.
fn ciLit(s: [*c]const u8, lit: []const u8) usize {
    var i: usize = 0;
    while (i < lit.len) : (i += 1) {
        var a = s[i];
        if (a >= 'A' and a <= 'Z') a = a + 32;
        if (a != lit[i]) return 0;
    }
    return i;
}

/// Converts the exact value D * 10^dec_exp to the nearest p-bit significand.
/// out_e uses value == mant * 2^(out_e - (p-1)); the caller compares out_e
/// to the format's max exponent for overflow and out_mant to 2^(p-1) for
/// the subnormal/normal split.
pub fn decimalToBinary(D: *const Bn, dec_exp: i32, sticky: c_int, p: i32, min_exp_normal: i32, out_mant: *u64, out_e: *i32) void {
    var num: Bn = .{};
    var den: Bn = .{};
    if (dec_exp >= 0) {
        bn.copy(&num, D);
        bn.mulPow10(&num, dec_exp);
        bn.fromU64(&den, 1);
    } else {
        bn.copy(&num, D);
        bn.fromU64(&den, 1);
        bn.mulPow10(&den, -dec_exp);
    }
    const s_cap = (p - 1) - min_exp_normal;
    var s: i32 = (p - 1) - (bn.bitlen(&num) - bn.bitlen(&den));
    if (s > s_cap) s = s_cap;

    var q: Bn = .{};
    var r: Bn = .{};
    var d_used: Bn = .{};
    while (true) {
        var n2: Bn = .{};
        var d2: Bn = .{};
        if (s >= 0) {
            bn.copy(&n2, &num);
            bn.shl(&n2, @intCast(s));
            bn.copy(&d2, &den);
        } else {
            bn.copy(&n2, &num);
            bn.copy(&d2, &den);
            bn.shl(&d2, @intCast(-s));
        }
        bn.divmod(&n2, &d2, &q, &r);
        const qbits = bn.bitlen(&q);
        if (qbits > p) {
            s -= qbits - p;
            continue;
        }
        if (qbits < p and s < s_cap) {
            const want = s + (p - qbits);
            s = if (want > s_cap) s_cap else want;
            continue;
        }
        bn.copy(&d_used, &d2);
        break;
    }
    var qval: u64 = if (q.n > 0) @as(u64, q.d[0]) else 0;
    if (q.n > 1) qval |= @as(u64, q.d[1]) << 32;
    const cmpres = bn.cmp2x(&r, &d_used);
    const roundup = cmpres > 0 or (cmpres == 0 and (sticky != 0 or (qval & 1) != 0));
    if (roundup) qval += 1;
    var E = (p - 1) - s;
    if (qval == (@as(u64, 1) << @as(u6, @intCast(p)))) {
        qval >>= 1;
        E += 1;
    }
    out_mant.* = qval;
    out_e.* = E;
}

/// The shared core for rt_parse_f64/rt_parse_f32: strtod/strtof grammar
/// minus hex floats and the NaN(n-char-seq) payload.
pub fn parseGeneric(s0: [*c]const u8, end: *[*c]const u8, p: i32, min_exp_normal: i32, bias: i32, max_biased_exp: u32, sign_pos: u6, bits_out: *u64, erange: *c_int) c_int {
    erange.* = 0;
    var s = s0;
    while (s[0] == ' ' or s[0] == '\t' or s[0] == '\n' or s[0] == 0x0b or s[0] == 0x0c or s[0] == '\r') s += 1;
    var neg = false;
    if (s[0] == '+' or s[0] == '-') {
        neg = s[0] == '-';
        s += 1;
    }
    var n = ciLit(s, "infinity");
    if (n == 0) n = ciLit(s, "inf");
    if (n != 0) {
        s += n;
        bits_out.* = (@as(u64, @intFromBool(neg)) << sign_pos) |
            (@as(u64, max_biased_exp) << @as(u6, @intCast(p - 1)));
        end.* = s;
        return 1;
    }
    n = ciLit(s, "nan");
    if (n != 0) {
        s += n;
        if (s[0] == '(') {
            var q = s + 1;
            while (q[0] != 0 and q[0] != ')') q += 1;
            if (q[0] == ')') s = q + 1;
        }
        bits_out.* = (@as(u64, @intFromBool(neg)) << sign_pos) |
            (@as(u64, max_biased_exp) << @as(u6, @intCast(p - 1))) |
            (@as(u64, 1) << @as(u6, @intCast(p - 2)));
        end.* = s;
        return 1;
    }
    var D: Bn = .{};
    var dec_exp: i32 = 0;
    var sticky: c_int = 0;
    var sig_count: i32 = 0;
    if (scanUnsignedDecimal(&s, &D, &dec_exp, &sticky, &sig_count) == 0) {
        end.* = s0;
        return 0;
    }
    end.* = s;
    if (bn.isZero(&D)) {
        bits_out.* = @as(u64, @intFromBool(neg)) << sign_pos;
        return 1;
    }
    const point_exp = dec_exp + sig_count;
    var mant: u64 = 0;
    var E: i32 = 0;
    if (point_exp > exp10_shortcut) {
        mant = @as(u64, 1) << @as(u6, @intCast(p - 1));
        E = @as(i32, @intCast(max_biased_exp)) - bias + 1; // force the overflow branch below
    } else if (point_exp < -exp10_shortcut) {
        mant = 0;
        E = min_exp_normal;
    } else {
        decimalToBinary(&D, dec_exp, sticky, p, min_exp_normal, &mant, &E);
    }
    if (@as(i64, E) + bias >= @as(i64, max_biased_exp)) { // overflow to infinity, incl. round-to-even at the largest finite value
        bits_out.* = (@as(u64, @intFromBool(neg)) << sign_pos) |
            (@as(u64, max_biased_exp) << @as(u6, @intCast(p - 1)));
        erange.* = 1; // matches glibc: HUGE_VAL result sets errno to ERANGE
        return 1;
    }
    var biased_exp: u64 = 0;
    var field: u64 = 0;
    if ((mant >> @as(u6, @intCast(p - 1))) != 0) { // normal: mant has the implicit top bit set
        biased_exp = @intCast(E + bias);
        field = mant & ((@as(u64, 1) << @as(u6, @intCast(p - 1))) - 1);
    } else {
        biased_exp = 0;
        field = mant;
        erange.* = 1; // matches glibc: a nonzero D underflowing to subnormal-or-0 sets ERANGE too
    }
    bits_out.* = (@as(u64, @intFromBool(neg)) << sign_pos) | (biased_exp << @as(u6, @intCast(p - 1))) | field;
    return 1;
}
