// i128 / u128 as two 64-bit words (u.w.lo, u.w.hi) with no __int128 division and no compiler-rt;
// built only under KIGUMI_INT128, so a program without 128-bit integers carries none of it.
typedef struct {
    uint64_t lo, hi;
} w128;
#define W_IS(nk) (((nk) & 255) == 128)
#define W_SIGNED(nk) (((nk) >> 8) & 1)
static w128 w_new(uint64_t lo, uint64_t hi) {
    w128 w = {lo, hi};
    return w;
}
static w128 w_of(val v) {
    if (W_IS(v->nk)) return w_new((uint64_t)v->u.w.lo, (uint64_t)v->u.w.hi);
    if (W_SIGNED(v->nk) && v->u.i < 0) return w_new((uint64_t)v->u.i, ~(uint64_t)0);
    return w_new((uint64_t)v->u.i, 0);
}
static val w_box(w128 w, int nk) {
    val v = alloc(K_INT);
    v->u.w.lo = (int64_t)w.lo;
    v->u.w.hi = (int64_t)w.hi;
    v->nk = nk;
    return v;
}
static int w_neg(w128 a) { return (int)(a.hi >> 63); }
static int w_zero(w128 a) { return (a.lo | a.hi) == 0; }
static w128 w_not(w128 a) { return w_new(~a.lo, ~a.hi); }
static w128 w_add(w128 a, w128 b) {
    uint64_t lo = a.lo + b.lo;
    return w_new(lo, a.hi + b.hi + (lo < a.lo));
}
static w128 w_sub(w128 a, w128 b) {
    return w_new(a.lo - b.lo, a.hi - b.hi - (a.lo < b.lo));
}
static w128 w_negate(w128 a) { return w_add(w_not(a), w_new(1, 0)); }
static w128 w_abs(w128 a) { return w_neg(a) ? w_negate(a) : a; }
static int w_cmp_u(w128 a, w128 b) {
    if (a.hi != b.hi) return a.hi < b.hi ? -1 : 1;
    return (a.lo > b.lo) - (a.lo < b.lo);
}
static int w_cmp_s(w128 a, w128 b) {
    if (a.hi != b.hi) return (int64_t)a.hi < (int64_t)b.hi ? -1 : 1;
    return (a.lo > b.lo) - (a.lo < b.lo);
}
static w128 w_shl(w128 a, unsigned n) {
    if (n == 0) return a;
    if (n >= 64) return w_new(0, a.lo << (n - 64));
    return w_new(a.lo << n, a.hi << n | a.lo >> (64 - n));
}
static w128 w_shr(w128 a, unsigned n) {
    if (n == 0) return a;
    if (n >= 64) return w_new(a.hi >> (n - 64), 0);
    return w_new(a.lo >> n | a.hi << (64 - n), a.hi >> n);
}
static w128 w_sar(w128 a, unsigned n) {
    uint64_t sign = (uint64_t)((int64_t)a.hi >> 63);
    if (n == 0) return a;
    if (n >= 64) return w_new((uint64_t)((int64_t)a.hi >> (n - 64)), sign);
    return w_new(a.lo >> n | a.hi << (64 - n), (uint64_t)((int64_t)a.hi >> n));
}
static w128 w_rotl(w128 a, unsigned n) {
    if (n == 0) return a;
    w128 l = w_shl(a, n), r = w_shr(a, 128 - n);
    return w_new(l.lo | r.lo, l.hi | r.hi);
}
static void w_mul64(uint64_t a, uint64_t b, uint64_t* hi, uint64_t* lo) {
#ifdef __SIZEOF_INT128__
    unsigned __int128 p = (unsigned __int128)a * b;
    *lo = (uint64_t)p;
    *hi = (uint64_t)(p >> 64);
#else
    uint64_t a0 = (uint32_t)a, a1 = a >> 32, b0 = (uint32_t)b, b1 = b >> 32;
    uint64_t p00 = a0 * b0, p01 = a0 * b1, p10 = a1 * b0, p11 = a1 * b1;
    uint64_t mid = (p00 >> 32) + (uint32_t)p01 + (uint32_t)p10;
    *lo = mid << 32 | (uint32_t)p00;
    *hi = p11 + (p01 >> 32) + (p10 >> 32) + (mid >> 32);
#endif
}
static w128 w_mul_u(w128 a, w128 b, int* over) {
    uint64_t h1, lo, h2, l2, h3, l3;
    w_mul64(a.lo, b.lo, &h1, &lo);
    w_mul64(a.hi, b.lo, &h2, &l2);
    w_mul64(a.lo, b.hi, &h3, &l3);
    uint64_t hi = h1 + l2;
    int c1 = hi < h1;
    uint64_t hi2 = hi + l3;
    int c2 = hi2 < hi;
    *over = (a.hi != 0 && b.hi != 0) || h2 != 0 || h3 != 0 || c1 || c2;
    return w_new(lo, hi2);
}
static w128 w_mul_s(w128 a, w128 b, int* over) {
    int neg = w_neg(a) != w_neg(b);
    w128 p = w_mul_u(w_abs(a), w_abs(b), over);
    if (*over) return p;
    if (neg) {
        *over = p.hi > ((uint64_t)1 << 63) || (p.hi == ((uint64_t)1 << 63) && p.lo != 0);
        return w_negate(p);
    }
    *over = p.hi >= ((uint64_t)1 << 63);
    return p;
}
// Hacker's Delight divlu: (u1:u0) / v for u1 < v, rem in *r.
static uint64_t w_divlu(uint64_t u1, uint64_t u0, uint64_t v, uint64_t* r) {
    const uint64_t b = (uint64_t)1 << 32;
    int s = __builtin_clzll(v);
    v <<= s;
    uint64_t vn1 = v >> 32, vn0 = v & 0xFFFFFFFF;
    uint64_t un64 = s > 0 ? (u1 << s) | (u0 >> (64 - s)) : u1;
    uint64_t un10 = u0 << s;
    uint64_t un1 = un10 >> 32, un0 = un10 & 0xFFFFFFFF;
    uint64_t q1 = un64 / vn1, rhat = un64 - q1 * vn1;
    while (q1 >= b || q1 * vn0 > b * rhat + un1) {
        q1--;
        rhat += vn1;
        if (rhat >= b) break;
    }
    uint64_t un21 = un64 * b + un1 - q1 * v;
    uint64_t q0 = un21 / vn1;
    rhat = un21 - q0 * vn1;
    while (q0 >= b || q0 * vn0 > b * rhat + un0) {
        q0--;
        rhat += vn1;
        if (rhat >= b) break;
    }
    *r = (un21 * b + un0 - q0 * v) >> s;
    return q1 * b + q0;
}
static w128 w_div64(w128 a, uint64_t v, uint64_t* r) {
    w128 q = w_new(0, 0);
    if (a.hi < v) {
        q.lo = w_divlu(a.hi, a.lo, v, r);
        return q;
    }
    q.hi = a.hi / v;
    uint64_t rem = a.hi % v;
    q.lo = w_divlu(rem, a.lo, v, r);
    return q;
}
static w128 w_divmod_u(w128 a, w128 b, w128* rem) {
    if (b.hi == 0) {
        uint64_t r;
        w128 q = w_div64(a, b.lo, &r);
        *rem = w_new(r, 0);
        return q;
    }
    unsigned n = (unsigned)__builtin_clzll(b.hi);
    w128 v1 = w_shl(b, n), u1 = w_shr(a, 1);
    uint64_t r1;
    uint64_t tq = w_divlu(u1.hi, u1.lo, v1.hi, &r1);
    tq >>= 63 - n;
    if (tq != 0) tq--;
    int over;
    w128 r = w_sub(a, w_mul_u(b, w_new(tq, 0), &over));
    if (w_cmp_u(r, b) >= 0) {
        tq++;
        r = w_sub(r, b);
    }
    *rem = r;
    return w_new(tq, 0);
}
static w128 w_divmod_s(w128 a, w128 b, w128* rem) {
    w128 q = w_divmod_u(w_abs(a), w_abs(b), rem);
    if (w_neg(a) != w_neg(b)) q = w_negate(q);
    if (w_neg(a)) *rem = w_negate(*rem);
    return q;
}
// out must hold 41 bytes: a sign, 39 digits and the NUL.
static void w_dec(w128 w, int is_signed, char* out) {
    char tmp[40];
    int n = 0;
    if (is_signed && w_neg(w)) {
        *out++ = '-';
        w = w_negate(w);
    }
    do {
        uint64_t r;
        w = w_div64(w, 10, &r);
        tmp[n++] = (char)('0' + r);
    } while (!w_zero(w));
    for (int i = 0; i < n; i++) out[i] = tmp[n - 1 - i];
    out[n] = 0;
}
static val w_checked(w128 w, int nk, int over) {
    if (over) rt_panic("integer overflow");
    return w_box(w, nk);
}
static val w_binop(const char* op, val a, val b) {
    w128 x = w_of(a);
    int nk = a->nk, sg = W_SIGNED(nk), over;
    if (op[0] == '<' || op[0] == '>') {
        if (b->u.i < 0 || b->u.i >= 128) rt_panic("shift count out of range");
        unsigned n = (unsigned)b->u.i;
        if (op[0] == '<') return w_box(w_shl(x, n), nk);
        return w_box(sg ? w_sar(x, n) : w_shr(x, n), nk);
    }
    w128 y = w_of(b), r, rem;
    switch (op[0]) {
    case '+':
        r = w_add(x, y);
        over = sg ? w_neg(x) == w_neg(y) && w_neg(r) != w_neg(x) : w_cmp_u(r, x) < 0;
        return w_checked(r, nk, over);
    case '-':
        r = w_sub(x, y);
        over = sg ? w_neg(x) != w_neg(y) && w_neg(r) != w_neg(x) : w_cmp_u(x, y) < 0;
        return w_checked(r, nk, over);
    case '*':
        r = sg ? w_mul_s(x, y, &over) : w_mul_u(x, y, &over);
        return w_checked(r, nk, over);
    case '/':
    case '%':
        if (w_zero(y)) rt_panic("division by zero");
        if (sg && y.lo == ~(uint64_t)0 && y.hi == ~(uint64_t)0 && x.lo == 0 && x.hi == (uint64_t)1 << 63) rt_panic("integer overflow");
        r = sg ? w_divmod_s(x, y, &rem) : w_divmod_u(x, y, &rem);
        return w_box(op[0] == '/' ? r : rem, nk);
    case '&':
        return w_box(w_new(x.lo & y.lo, x.hi & y.hi), nk);
    case '|':
        return w_box(w_new(x.lo | y.lo, x.hi | y.hi), nk);
    case '^':
        return w_box(w_new(x.lo ^ y.lo, x.hi ^ y.hi), nk);
    }
    panicf("unsupported operator %s", op);
    return NULL;
}
static val w_unop(char op, val a) {
    w128 x = w_of(a);
    if (op == '-') return w_checked(w_negate(x), a->nk, x.lo == 0 && x.hi == (uint64_t)1 << 63);
    return w_box(w_not(x), a->nk);
}
static int w_compare(val a, val b) {
    return W_SIGNED(a->nk) ? w_cmp_s(w_of(a), w_of(b)) : w_cmp_u(w_of(a), w_of(b));
}
static int w_equal(val a, val b) {
    w128 x = w_of(a), y = w_of(b);
    return x.lo == y.lo && x.hi == y.hi;
}
static val w_wrapping(const char* op, val a0, val a1) {
    w128 x = w_of(a0), y = w_of(a1);
    int nk = a0->nk;
    unsigned c = (unsigned)(y.lo & 127);
    int over;
    if (!strcmp(op, "wrappingAdd")) return w_box(w_add(x, y), nk);
    if (!strcmp(op, "wrappingSub")) return w_box(w_sub(x, y), nk);
    if (!strcmp(op, "wrappingMul")) return w_box(w_mul_u(x, y, &over), nk);
    if (!strcmp(op, "wrappingShiftLeft")) return w_box(w_shl(x, c), nk);
    if (!strcmp(op, "wrappingShiftRight")) return w_box(W_SIGNED(nk) ? w_sar(x, c) : w_shr(x, c), nk);
    if (!strcmp(op, "rotateLeft")) return w_box(w_rotl(x, c), nk);
    if (!strcmp(op, "rotateRight")) return w_box(w_rotl(x, (128 - c) & 127), nk);
    return NULL;
}
static unsigned w_bitlen(w128 w) {
    if (w.hi) return 128 - (unsigned)__builtin_clzll(w.hi);
    return w.lo ? 64 - (unsigned)__builtin_clzll(w.lo) : 0;
}
static double w_pow2(unsigned n) {
    uint64_t bits = (uint64_t)(n + 1023) << 52;
    double d;
    memcpy(&d, &bits, sizeof d);
    return d;
}
// The magnitude rounded to mant bits, ties to even, as the hardware converts.
static double w_to_double(int neg, w128 mag, unsigned mant) {
    unsigned n = w_bitlen(mag);
    double f;
    if (n <= mant) {
        f = (double)mag.lo;
    } else {
        unsigned shift = n - mant;
        uint64_t top = w_shr(mag, shift).lo;
        w128 rem = w_sub(mag, w_shl(w_new(top, 0), shift));
        w128 half = w_shl(w_new(1, 0), shift - 1);
        int c = w_cmp_u(rem, half);
        if (c > 0 || (c == 0 && (top & 1))) top++;
        f = (double)top * w_pow2(shift);
    }
    return neg ? -f : f;
}
// Truncates x; 0 for NaN, an infinity or a magnitude of 2^128 and up.
static int w_from_double(double x, int* neg, w128* mag) {
    uint64_t b;
    memcpy(&b, &x, sizeof b);
    int e = (int)((b >> 52) & 0x7ff);
    *mag = w_new(0, 0);
    *neg = 0;
    if (e == 0x7ff) return 0;
    if (e == 0) return 1;
    *neg = (int)(b >> 63);
    uint64_t m = (b & (((uint64_t)1 << 52) - 1)) | ((uint64_t)1 << 52);
    e -= 1075;
    if (e >= 76) return 0;
    if (e >= 0) *mag = w_shl(w_new(m, 0), (unsigned)e);
    else if (e > -64) *mag = w_new(m >> -e, 0);
    return 1;
}
static int w_fits(int neg, w128 mag, int dnk) {
    unsigned b = (unsigned)(dnk & 255);
    if (W_SIGNED(dnk)) {
        int c = w_cmp_u(mag, w_shl(w_new(1, 0), b - 1));
        return neg ? c <= 0 : c < 0;
    }
    if (neg) return w_zero(mag);
    return b == 128 || w_cmp_u(mag, w_shl(w_new(1, 0), b)) < 0;
}
static val w_from_mag(int neg, w128 mag, int dnk) {
    if (neg) mag = w_negate(mag);
    if (W_IS(dnk)) return w_box(mag, dnk);
    return rt_int(truncate_to((int64_t)mag.lo, dnk), dnk);
}
static val w_convert(val a0, int dnk, int checked) {
    int neg;
    w128 mag;
    if (dnk & 512) {
        w128 w = w_of(a0);
        neg = W_SIGNED(a0->nk) && w_neg(w);
        return rt_float(w_to_double(neg, neg ? w_negate(w) : w, (dnk & 255) == 32 ? 24 : 53), dnk);
    }
    if (a0->kind == K_FLOAT) {
        if (!w_from_double(a0->u.f, &neg, &mag) || !w_fits(neg, mag, dnk)) return none();
        return some(w_from_mag(neg, mag, dnk));
    }
    w128 w = w_of(a0);
    neg = W_SIGNED(a0->nk) && w_neg(w);
    mag = neg ? w_negate(w) : w;
    if (checked && !w_fits(neg, mag, dnk)) return none();
    return checked ? some(w_from_mag(neg, mag, dnk)) : w_from_mag(neg, mag, dnk);
}
static int64_t w_range_len(val lo, val hi, int inclusive) {
    if (w_compare(hi, lo) < 0) return 0;
    w128 d = w_sub(w_of(hi), w_of(lo));
    if (d.hi != 0 || d.lo >= (uint64_t)INT64_MAX) return INT64_MAX;
    return (int64_t)d.lo + (inclusive != 0);
}
