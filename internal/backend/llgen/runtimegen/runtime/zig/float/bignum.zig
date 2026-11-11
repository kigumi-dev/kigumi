//! Minimal unsigned big integer: base 2^32 limbs, little-endian, no sign.
//! 192 limbs (6144 bits, ~1849 decimal digits) covers every value Dragon4
//! and Algorithm M build, with headroom for the shifts they apply to align
//! binary and decimal exponents. Limb indices are bounds-checked by the
//! compiler's safety checks, so a logic bug panics instead of overwriting
//! the stack.

pub const limb_count = 192;

pub const Bn = struct {
    d: [limb_count]u32 = undefined,
    /// Used limbs; n == 0 means the value 0.
    n: usize = 0,
};

pub fn zero(a: *Bn) void {
    a.n = 0;
}

pub fn trim(a: *Bn) void {
    while (a.n > 0 and a.d[a.n - 1] == 0) a.n -= 1;
}

pub fn fromU64(a: *Bn, value: u64) void {
    a.n = 0;
    var v = value;
    while (v != 0) {
        a.d[a.n] = @truncate(v);
        a.n += 1;
        v >>= 32;
    }
}

pub fn isZero(a: *const Bn) bool {
    return a.n == 0;
}

pub fn copy(dst: *Bn, src: *const Bn) void {
    dst.n = src.n;
    @memcpy(dst.d[0..src.n], src.d[0..src.n]);
}

pub fn cmp(a: *const Bn, b: *const Bn) i32 {
    if (a.n != b.n) return if (a.n < b.n) -1 else 1;
    var i = a.n;
    while (i > 0) {
        i -= 1;
        if (a.d[i] != b.d[i]) return if (a.d[i] < b.d[i]) -1 else 1;
    }
    return 0;
}

pub fn bitlen(a: *const Bn) i32 {
    if (a.n == 0) return 0;
    var top = a.d[a.n - 1];
    var bits: i32 = @intCast((a.n - 1) * 32);
    while (top != 0) {
        bits += 1;
        top >>= 1;
    }
    return bits;
}

/// a += b
pub fn add(a: *Bn, b: *const Bn) void {
    const n = @max(a.n, b.n);
    var carry: u64 = 0;
    var i: usize = 0;
    while (i < n) : (i += 1) {
        const av: u64 = if (i < a.n) a.d[i] else 0;
        const bv: u64 = if (i < b.n) b.d[i] else 0;
        const x = av + bv + carry;
        a.d[i] = @truncate(x);
        carry = x >> 32;
    }
    a.n = n;
    if (carry != 0) {
        a.d[a.n] = @truncate(carry);
        a.n += 1;
    }
}

/// a -= b, requires a >= b
pub fn sub(a: *Bn, b: *const Bn) void {
    var borrow: i64 = 0;
    var i: usize = 0;
    while (i < a.n) : (i += 1) {
        const bv: i64 = if (i < b.n) b.d[i] else 0;
        var x: i64 = @as(i64, a.d[i]) - bv - borrow;
        if (x < 0) {
            x += @as(i64, 1) << 32;
            borrow = 1;
        } else {
            borrow = 0;
        }
        a.d[i] = @intCast(x);
    }
    trim(a);
}

/// a *= m, m a small (< 2^32) multiplier
pub fn mulU32(a: *Bn, m: u32) void {
    var carry: u64 = 0;
    var i: usize = 0;
    while (i < a.n) : (i += 1) {
        const x = @as(u64, a.d[i]) * m + carry;
        a.d[i] = @truncate(x);
        carry = x >> 32;
    }
    while (carry != 0) {
        a.d[a.n] = @truncate(carry);
        a.n += 1;
        carry >>= 32;
    }
}

/// a += m (small)
pub fn addU32(a: *Bn, m: u32) void {
    var carry: u64 = m;
    var i: usize = 0;
    while (i < a.n and carry != 0) : (i += 1) {
        const x = @as(u64, a.d[i]) + carry;
        a.d[i] = @truncate(x);
        carry = x >> 32;
    }
    while (carry != 0) {
        a.d[a.n] = @truncate(carry);
        a.n += 1;
        carry >>= 32;
    }
}

/// a <<= bits
pub fn shl(a: *Bn, bits: usize) void {
    if (bits == 0 or a.n == 0) return;
    const limb_shift = bits / 32;
    const bit_shift: u5 = @intCast(bits % 32);
    const new_n = a.n + limb_shift + @intFromBool(bit_shift != 0);
    var i = new_n;
    while (i > 0) {
        i -= 1;
        var lo: u32 = 0;
        var hi: u32 = 0;
        const src: isize = @as(isize, @intCast(i)) - @as(isize, @intCast(limb_shift));
        if (src >= 0 and src < @as(isize, @intCast(a.n))) lo = a.d[@intCast(src)];
        if (bit_shift != 0 and src - 1 >= 0 and src - 1 < @as(isize, @intCast(a.n))) hi = a.d[@intCast(src - 1)];
        const v: u32 = if (bit_shift != 0)
            (lo << bit_shift) | @as(u32, @truncate(@as(u64, hi) >> (@as(u6, 32) - @as(u6, bit_shift))))
        else
            lo;
        a.d[i] = v;
    }
    a.n = new_n;
    trim(a);
}

pub fn mulPow10(a: *Bn, k: i32) void {
    var i: i32 = 0;
    while (i < k) : (i += 1) mulU32(a, 10);
}

/// Full binary long division: q = a / b, r = a % b. q's buffer may alias a.
pub fn divmod(a: *const Bn, b: *const Bn, q: *Bn, r: *Bn) void {
    zero(q);
    zero(r);
    const bits = bitlen(a);
    var i: usize = @intCast(if (bits > 0) bits else 0);
    while (i > 0) {
        i -= 1;
        shl(r, 1);
        const limb = i / 32;
        const off: u5 = @intCast(i % 32);
        if (limb < a.n and (a.d[limb] >> off) & 1 != 0) {
            if (r.n == 0) {
                r.d[0] = 1;
                r.n = 1;
            } else {
                r.d[0] |= 1;
            }
        }
        const bit = cmp(r, b) >= 0;
        if (bit) sub(r, b);
        if (bit) {
            while (q.n <= limb) {
                q.d[q.n] = 0;
                q.n += 1;
            }
            q.d[limb] |= @as(u32, 1) << off;
        }
    }
    trim(q);
}

/// 2*r compared against b, without doubling into a possibly-overflowing bignum.
pub fn cmp2x(r: *const Bn, b: *const Bn) i32 {
    var t: Bn = .{};
    copy(&t, r);
    shl(&t, 1);
    return cmp(&t, b);
}
