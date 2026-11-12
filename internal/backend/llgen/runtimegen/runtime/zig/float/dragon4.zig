//! IEEE-754 decomposition, Dragon4 shortest round-trip digits (Steele &
//! White) and the `%g`-shaped placement Go's strconv uses.

const bn = @import("bignum.zig");
const Bn = bn.Bn;

/// A finite nonzero magnitude as value = m * 2^e (m the full p-bit
/// significand). min_exp distinguishes a genuine power-of-two boundary
/// (asymmetric neighbor spacing) from the smallest normal, which has none.
pub const Decomp = struct {
    m: u64,
    e: i32,
    p: i32,
    min_exp: i32,
};

pub fn decomposeF64(x: f64) Decomp {
    const bits: u64 = @bitCast(x);
    const biased: i32 = @intCast((bits >> 52) & 0x7FF);
    const frac = bits & 0xFFFFFFFFFFFFF;
    var r = Decomp{ .m = 0, .e = 0, .p = 53, .min_exp = -1074 };
    if (biased == 0) {
        r.m = frac;
        r.e = -1074;
    } else {
        r.m = frac | (@as(u64, 1) << 52);
        r.e = biased - 1023 - 52;
    }
    return r;
}

pub fn decomposeF32(x: f32) Decomp {
    const bits: u32 = @bitCast(x);
    const biased: i32 = @intCast((bits >> 23) & 0xFF);
    const frac = bits & 0x7FFFFF;
    var r = Decomp{ .m = 0, .e = 0, .p = 24, .min_exp = -149 };
    if (biased == 0) {
        r.m = frac;
        r.e = -149;
    } else {
        r.m = @as(u64, frac | @as(u32, 1) << 23);
        r.e = biased - 127 - 23;
    }
    return r;
}

/// Shortest decimal digits such that the value equals 0.<digits> * 10^dp;
/// digits must hold fd.p+1 bytes (the +1 covers a carry-out inserting a
/// leading "1", e.g. 9.99...9 rounding up to 10.0...0).
pub fn shortest(fd: Decomp, digits: *[64]u8, ndigits: *i32, dp: *i32) void {
    const pow2 = fd.m == (@as(u64, 1) << @as(u6, @intCast(fd.p - 1)));
    const asymmetric = pow2 and fd.e > fd.min_exp;

    var R: Bn = .{ .n = 0 };
    var S: Bn = .{ .n = 0 };
    var mP: Bn = .{ .n = 0 };
    var mM: Bn = .{ .n = 0 };
    bn.fromU64(&R, fd.m);
    bn.mulU32(&R, 4);
    bn.fromU64(&S, 4);
    bn.fromU64(&mP, 2);
    bn.fromU64(&mM, if (asymmetric) 1 else 2);
    if (fd.e >= 0) {
        bn.shl(&R, @intCast(fd.e));
        bn.shl(&mP, @intCast(fd.e));
        bn.shl(&mM, @intCast(fd.e));
    } else {
        bn.shl(&S, @intCast(-fd.e));
    }

    // Decimal-point estimate from bit length (within ~1 of exact), fixed
    // up below by exact bignum comparison.
    const bits = bn.bitlen(&R) - bn.bitlen(&S);
    var k: i32 = @intCast(@divTrunc(@as(i64, bits) * 30103 + 99999, 100000) + 1);
    if (k >= 0) {
        bn.mulPow10(&S, k);
    } else {
        bn.mulPow10(&R, -k);
        bn.mulPow10(&mP, -k);
        bn.mulPow10(&mM, -k);
    }
    while (true) {
        var t: Bn = .{ .n = 0 };
        bn.copy(&t, &R);
        bn.add(&t, &mP);
        if (bn.cmp(&t, &S) > 0) {
            bn.mulU32(&S, 10);
            k += 1;
            continue;
        }
        var t10: Bn = .{ .n = 0 };
        bn.copy(&t10, &t);
        bn.mulU32(&t10, 10);
        if (bn.cmp(&t10, &S) <= 0) {
            bn.mulU32(&R, 10);
            bn.mulU32(&mP, 10);
            bn.mulU32(&mM, 10);
            k -= 1;
            continue;
        }
        break;
    }

    const round_even = (fd.m & 1) == 0;
    var n: usize = 0;
    while (true) {
        bn.mulU32(&R, 10);
        bn.mulU32(&mP, 10);
        bn.mulU32(&mM, 10);
        // R < S*10 is the loop invariant (R<S held before scaling by 10),
        // so the digit is a single decimal digit found by subtraction
        // rather than a full division.
        var d: u32 = 0;
        while (bn.cmp(&R, &S) >= 0) {
            bn.sub(&R, &S);
            d += 1;
        }
        const low = if (round_even) bn.cmp(&R, &mM) <= 0 else bn.cmp(&R, &mM) < 0;
        var t: Bn = .{ .n = 0 };
        bn.copy(&t, &R);
        bn.add(&t, &mP);
        const high = if (round_even) bn.cmp(&t, &S) >= 0 else bn.cmp(&t, &S) > 0;
        if (!low and !high) {
            digits[n] = '0' + @as(u8, @intCast(d));
            n += 1;
            continue;
        }
        if (low and !high) {
            digits[n] = '0' + @as(u8, @intCast(d));
        } else if (high and !low) {
            digits[n] = '0' + @as(u8, @intCast(d + 1));
        } else {
            // 2R==S is an exact decimal tie (only possible when x's binary
            // expansion terminates in decimal). Round-half-to-even on d
            // matches Go's strconv except for f32's 2^-12 (fd.p==24,
            // fd.e==-35): Go's Dragonbox rounds up there instead, a pinned
            // hardcoded exception, not a general rule (see `exp == -77`
            // in Go's internal/strconv/ftoadbox.go).
            const cmp2 = bn.cmp2x(&R, &S);
            const up = cmp2 > 0 or (cmp2 == 0 and ((fd.p == 24 and fd.e == -35) or (d & 1) != 0));
            digits[n] = '0' + @as(u8, @intCast(d + @intFromBool(up)));
        }
        n += 1;
        break;
    }
    // A rounded-up final digit of value 10 carries into earlier digits
    // (e.g. 9.9995 rounding its last kept digit up becomes 10.000).
    var i = n - 1;
    while (digits[i] == '0' + 10) {
        digits[i] = '0';
        if (i == 0) {
            var j = n;
            while (j > 0) {
                j -= 1;
                digits[j + 1] = digits[j];
            }
            digits[0] = '1';
            n += 1;
            k += 1;
            break;
        }
        i -= 1;
        digits[i] += 1;
    }
    ndigits.* = @intCast(n);
    dp.* = k;
}

fn copyLit(out: []u8, s: []const u8) void {
    if (out.len == 0) return;
    var i: usize = 0;
    while (i < s.len and i + 1 < out.len) : (i += 1) out[i] = s[i];
    out[i] = 0;
}

/// Places digits[0..nd] at decimal point dp, in Go strconv's %g layout:
/// scientific when the leading digit's power is below -4 or at least 6,
/// plain digits otherwise.
fn placeDigits(out: []u8, neg: bool, digits: []const u8, dp: i32) void {
    if (out.len == 0) return;
    const end = out.len - 1;
    var p: usize = 0;
    if (neg and p < end) {
        out[p] = '-';
        p += 1;
    }
    const exp = dp - 1;
    if (exp < -4 or exp >= 6) {
        if (p < end) {
            out[p] = digits[0];
            p += 1;
        }
        if (digits.len > 1) {
            if (p < end) {
                out[p] = '.';
                p += 1;
            }
            var i: usize = 1;
            while (i < digits.len and p < end) : (i += 1) {
                out[p] = digits[i];
                p += 1;
            }
        }
        if (p < end) {
            out[p] = 'e';
            p += 1;
        }
        if (p < end) {
            out[p] = if (exp < 0) '-' else '+';
            p += 1;
        }
        const ae: i32 = if (exp < 0) -exp else exp;
        var ebuf: [8]u8 = undefined;
        var en: usize = 0;
        var v = ae;
        while (true) {
            ebuf[en] = '0' + @as(u8, @intCast(@rem(v, 10)));
            en += 1;
            v = @divTrunc(v, 10);
            if (v == 0) break;
        }
        while (en < 2) {
            ebuf[en] = '0';
            en += 1;
        }
        var i = en;
        while (i > 0 and p < end) {
            i -= 1;
            out[p] = ebuf[i];
            p += 1;
        }
    } else if (dp <= 0) {
        if (p < end) {
            out[p] = '0';
            p += 1;
        }
        if (p < end) {
            out[p] = '.';
            p += 1;
        }
        var i: i32 = 0;
        while (i < -dp and p < end) : (i += 1) {
            out[p] = '0';
            p += 1;
        }
        var j: usize = 0;
        while (j < digits.len and p < end) : (j += 1) {
            out[p] = digits[j];
            p += 1;
        }
    } else if (dp >= @as(i32, @intCast(digits.len))) {
        var j: usize = 0;
        while (j < digits.len and p < end) : (j += 1) {
            out[p] = digits[j];
            p += 1;
        }
        const zeros = dp - @as(i32, @intCast(digits.len));
        var i: i32 = 0;
        while (i < zeros and p < end) : (i += 1) {
            out[p] = '0';
            p += 1;
        }
    } else {
        var i: i32 = 0;
        var j: usize = 0;
        while (i < dp and p < end) : (i += 1) {
            out[p] = digits[j];
            p += 1;
            j += 1;
        }
        if (p < end) {
            out[p] = '.';
            p += 1;
        }
        var j2: usize = @intCast(dp);
        while (j2 < digits.len and p < end) : (j2 += 1) {
            out[p] = digits[j2];
            p += 1;
        }
    }
    out[p] = 0;
}

/// Formats x the way Go's strconv.FormatFloat(x, 'g', -1, bits) does, into
/// out (a NUL-terminated C buffer of out.len bytes).
pub fn formatFloat(x: f64, is_f32: bool, out: []u8) void {
    const bits: u64 = @bitCast(x);
    const neg = (bits >> 63) != 0;
    if (((bits >> 52) & 0x7FF) == 0x7FF) {
        copyLit(out, if ((bits & 0xFFFFFFFFFFFFF) != 0) "NaN" else if (neg) "-Inf" else "+Inf");
        return;
    }
    if ((bits << 1) == 0) {
        copyLit(out, if (neg) "-0" else "0");
        return;
    }
    const fd = if (is_f32) decomposeF32(@as(f32, @floatCast(x))) else decomposeF64(x);
    var digits: [64]u8 = undefined;
    var nd: i32 = 0;
    var dp: i32 = 0;
    shortest(fd, &digits, &nd, &dp);
    placeDigits(out, neg, digits[0..@intCast(nd)], dp);
}
