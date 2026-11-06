// ---- numbers ----
static int bits_of(int nk) {
    return nk & 255;
}
static int is_signed(int nk) {
    return (nk >> 8) & 1;
}
// A width-64 shift into the sign bit is signed-shift UB; INT64_MIN sidesteps it.
static int64_t min_of(int nk) {
    if (!is_signed(nk)) return 0;
    int b = bits_of(nk);
    return b >= 64 ? INT64_MIN : -((int64_t)1 << (b - 1));
}
static uint64_t max_of(int nk) {
    int b = bits_of(nk);
    if (is_signed(nk)) return b >= 64 ? (uint64_t)INT64_MAX : ((uint64_t)1 << (b - 1)) - 1;
    if (b >= 64) return ~(uint64_t)0;
    return ((uint64_t)1 << b) - 1;
}
static int64_t truncate_to(int64_t v, int nk) {
    int b = bits_of(nk);
    if (b >= 64) return v;
    int64_t mask = ((int64_t)1 << b) - 1;
    v &= mask;
    if (is_signed(nk) && (v & ((int64_t)1 << (b - 1)))) v -= (int64_t)1 << b;
    return v;
}
static int64_t checked_i64(int64_t v, int nk, int over) {
    if (!over) {
        if (is_signed(nk)) over = v < min_of(nk) || v > (int64_t)max_of(nk);
        else if (bits_of(nk) < 64) over = v < 0 || (uint64_t)v > max_of(nk);
    }
    if (over) rt_panic("integer overflow");
    return v;
}
static val checked(int64_t v, int nk, int over) {
    return rt_int(checked_i64(v, nk, over), nk);
}
static int64_t int_add(int64_t x, int64_t y, int nk) {
    int sg = is_signed(nk);
    int64_t r;
    uint64_t u = (uint64_t)x + (uint64_t)y;
    int over = sg ? __builtin_add_overflow(x, y, &r) : (bits_of(nk) == 64 && u < (uint64_t)x);
    if (!sg) r = (int64_t)u;
    return checked_i64(r, nk, over);
}
static int64_t int_sub(int64_t x, int64_t y, int nk) {
    int sg = is_signed(nk);
    int64_t r;
    int over = sg ? __builtin_sub_overflow(x, y, &r) : ((uint64_t)x < (uint64_t)y);
    if (!sg) r = (int64_t)((uint64_t)x - (uint64_t)y);
    return checked_i64(r, nk, over);
}
static int64_t int_mul(int64_t x, int64_t y, int nk) {
    int sg = is_signed(nk);
    int64_t r;
    int over;
    if (sg) {
        over = __builtin_mul_overflow(x, y, &r);
    } else {
        uint64_t p;
        over = __builtin_mul_overflow((uint64_t)x, (uint64_t)y, &p);
        r = (int64_t)p;
    }
    return checked_i64(r, nk, over);
}
static int64_t int_div(int64_t x, int64_t y, int nk) {
    if (y == 0) rt_panic("division by zero");
    if (is_signed(nk) && y == -1 && x == min_of(nk)) rt_panic("integer overflow");
    if (is_signed(nk)) return x / y;
    return (int64_t)((uint64_t)x / (uint64_t)y);
}
static int64_t int_mod(int64_t x, int64_t y, int nk) {
    if (y == 0) rt_panic("division by zero");
    if (is_signed(nk) && y == -1 && x == min_of(nk)) rt_panic("integer overflow");
    if (is_signed(nk)) return x % y;
    return (int64_t)((uint64_t)x % (uint64_t)y);
}
static int64_t int_xor(int64_t x, int64_t y, int nk) {
    return checked_i64(x ^ y, nk, 0);
}
static int64_t int_shl(int64_t x, int64_t y, int nk) {
    if (y < 0 || y >= bits_of(nk)) rt_panic("shift count out of range");
    return checked_i64((int64_t)((uint64_t)x << y), nk, 0);
}
static int64_t int_shr(int64_t x, int64_t y, int nk) {
    if (y < 0 || y >= bits_of(nk)) rt_panic("shift count out of range");
    return is_signed(nk) ? x >> y : (int64_t)((uint64_t)x >> y);
}
static int64_t int_neg(int64_t x, int nk) {
    return checked_i64((int64_t)(0 - (uint64_t)x), nk, x == min_of(nk));
}
static int64_t int_not(int64_t x, int nk) {
    return truncate_to(~x, nk);
}
// prelude.<T>.wrapping* / rotate*: modular arithmetic and shifts masked to the width.
// Op ids index wrapping_names.
static const char* const wrapping_names[] = {"wrappingAdd", "wrappingSub", "wrappingMul", "wrappingShiftLeft", "wrappingShiftRight", "rotateLeft", "rotateRight"};
static int64_t wrapping_op(int op, int64_t xs, int64_t ys, int nk) {
    int b = bits_of(nk);
    uint64_t x = (uint64_t)xs, y = (uint64_t)ys;
    unsigned c = (unsigned)(y & (uint64_t)(b - 1));
    uint64_t ux = b == 64 ? x : x & (((uint64_t)1 << b) - 1);
    switch (op) {
    case 0: return truncate_to((int64_t)(x + y), nk);
    case 1: return truncate_to((int64_t)(x - y), nk);
    case 2: return truncate_to((int64_t)(x * y), nk);
    case 3: return truncate_to((int64_t)(ux << c), nk);
    case 4: return truncate_to(is_signed(nk) ? truncate_to((int64_t)ux, nk) >> c : (int64_t)(ux >> c), nk);
    case 5: return truncate_to((int64_t)(c ? (ux << c) | (ux >> (b - c)) : ux), nk);
    default: return truncate_to((int64_t)(c ? (ux >> c) | (ux << (b - c)) : ux), nk);
    }
}
static val int_fn(const char* k, val a0, val a1) {
    const char* op = strrchr(k, '.');
    if (!op++) return NULL;
#ifdef KIGUMI_INT128
    if (W_IS(a0->nk)) return w_wrapping(op, a0, a1);
#endif
    for (int i = 0; i < 7; i++)
        if (!strcmp(op, wrapping_names[i])) return rt_int(wrapping_op(i, a0->u.i, a1->u.i, a0->nk), a0->nk);
    return NULL;
}
static int cmp_vals(val a, val b) {
    a = rt_deref(a);
    b = rt_deref(b);
    switch (a->kind) {
    case K_INT:
#ifdef KIGUMI_INT128
        if (W_IS(a->nk)) return w_compare(a, b);
#endif
        if (is_signed(a->nk)) return (a->u.i > b->u.i) - (a->u.i < b->u.i);
        return ((uint64_t)a->u.i > (uint64_t)b->u.i) - ((uint64_t)a->u.i < (uint64_t)b->u.i);
    case K_FLOAT:
        return (a->u.f > b->u.f) - (a->u.f < b->u.f);
    case K_CHAR:
        return (a->u.c > b->u.c) - (a->u.c < b->u.c);
    case K_BOOL:
        return a->u.b - b->u.b;
    case K_STR:
    case K_BYTES: {
        int64_t n = a->u.s.len < b->u.s.len ? a->u.s.len : b->u.s.len;
        int c = memcmp(a->u.s.data, b->u.s.data, n);
        if (c) return c < 0 ? -1 : 1;
        return (a->u.s.len > b->u.s.len) - (a->u.s.len < b->u.s.len);
    }
    }
    return 0;
}
// float_total_equal/float_total_compare are the Eq/Ord protocol's `f64`/
// `f32` total order: every NaN
// equals every other NaN and sorts above every non-NaN value; -0.0 equals
// 0.0. `==`/`<`/... in rt_binop stay IEEE and do not call these.
static int float_total_equal(double a, double b) {
    int an = a != a, bn = b != b;
    if (an || bn) return an && bn;
    if (a == 0.0) a = 0.0;
    if (b == 0.0) b = 0.0;
    return a == b;
}
static int float_total_compare(double a, double b) {
    int an = a != a, bn = b != b;
    if (an && bn) return 0;
    if (an) return 1;
    if (bn) return -1;
    if (a == 0.0) a = 0.0;
    if (b == 0.0) b = 0.0;
    return (a > b) - (a < b);
}
static int in_range(int64_t v, int s, int d) {
    if (!is_signed(s) && v < 0) return 0;
    if (is_signed(d)) return v >= min_of(d) && v <= (int64_t)max_of(d);
    return v >= 0 && (bits_of(d) == 64 || (uint64_t)v <= max_of(d));
}
// rt_convert is the prelude's numeric `toX`: dnk is the target kind (bit 10
// for Char), checked says the declaration returns an Option.
val rt_convert(val a0, int dnk, int checked) {
    a0 = rt_deref(a0);
    if (a0->kind == K_BOX) a0 = a0->u.box.v;
    if (dnk & 1024) return rt_char((uint32_t)a0->u.i);
    if (a0->kind == K_CHAR) return rt_int(a0->u.c, dnk);
#ifdef KIGUMI_INT128
    if (W_IS(dnk) || (a0->kind == K_INT && W_IS(a0->nk))) return w_convert(a0, dnk, checked);
#endif
    if (dnk & 512) {
        if (a0->kind == K_FLOAT) return rt_float(a0->u.f, dnk);
        if ((dnk & 255) == 32) {
            // One rounding to f32; converting through f64 can cross a midpoint.
            float f = is_signed(a0->nk) ? (float)a0->u.i : (float)(uint64_t)a0->u.i;
            return rt_float((double)f, dnk);
        }
        return rt_float(is_signed(a0->nk) ? (double)a0->u.i : (double)(uint64_t)a0->u.i, dnk);
    }
    if (a0->kind == K_FLOAT) {
        double x = trunc(a0->u.f);
        if (x != x || x < -9223372036854775808.0 || x >= 9223372036854775808.0) return rt_none();
        return rt_some(rt_int((int64_t)x, dnk));
    }
    if (!checked) return rt_int(truncate_to(a0->u.i, dnk), dnk);
    if (!in_range(a0->u.i, a0->nk, dnk)) return rt_none();
    return rt_some(rt_int(a0->u.i, dnk));
}
static val direct_arg(val v) {
    v = rt_deref(v);
    return v->kind == K_BOX ? v->u.box.v : v;
}
static uint64_t double_bits(double d) {
    uint64_t b;
    memcpy(&b, &d, sizeof b);
    return b;
}
static uint64_t leaf_bits(val v) {
    v = rt_deref(v);
    switch (v->kind) {
    case K_FLOAT: return double_bits(v->u.f);
    case K_BOOL: return (uint64_t)(v->u.b != 0);
    case K_CHAR: return v->u.c;
    default: return (uint64_t)v->u.i;
    }
}
int64_t rt_d_len(val s) {
    return direct_arg(s)->u.s.len;
}
int64_t rt_d_array_len(val a) {
    return direct_arg(a)->u.arr.len;
}
int rt_d_bytes_get(val s, int64_t i, uint64_t* out) {
    s = direct_arg(s);
    *out = 0;
    if (i < 0 || i >= s->u.s.len) return 0;
    *out = (uint8_t)s->u.s.data[i];
    return 1;
}
int rt_d_array_get(val a, int64_t i, uint64_t* out) {
    a = direct_arg(a);
    *out = 0;
    if (i < 0 || i >= a->u.arr.len) return 0;
    *out = leaf_bits(a->u.arr.items[i]);
    return 1;
}
static int float_text_ok(const char* s, int64_t len) {
    int64_t i = 0;
    if (i < len && s[i] == '-') i++;
    int64_t start = i;
    while (i < len && s[i] >= '0' && s[i] <= '9') i++;
    if (i == start) return 0;
    if (i < len && s[i] == '.') {
        i++;
        start = i;
        while (i < len && s[i] >= '0' && s[i] <= '9') i++;
        if (i == start) return 0;
    }
    if (i < len && (s[i] == 'e' || s[i] == 'E')) {
        i++;
        if (i < len && (s[i] == '+' || s[i] == '-')) i++;
        start = i;
        while (i < len && s[i] >= '0' && s[i] <= '9') i++;
        if (i == start) return 0;
    }
    return i == len;
}
static int parse_float_text(val text, double* d) {
    val t = direct_arg(text);
    const char* s = rt_cstr(text);
    int64_t len = t->kind == K_OPAQUE ? (int64_t)strlen(s) : t->u.s.len;
    int ok = 0;
    if (float_text_ok(s, len)) {
        const char* end;
        // erange also fires on underflow; Go errors only on overflow, so reject only inf.
        ok = rt_parse_f64(s, &end, d, NULL) && !*end && !isinf(*d);
    }
    if (t->kind != K_OPAQUE) free((void*)s);
    return ok;
}
int rt_d_parse_float(val s, uint64_t* out) {
    double d;
    *out = 0;
    if (!parse_float_text(direct_arg(s), &d)) return 0;
    *out = double_bits(d);
    return 1;
}
// The direct hash of a scalar leaf: FNV-1a over the text display_into renders, with no box to hold it.
static int64_t fnv_bytes(const char* s, int n) {
    uint64_t h = 14695981039346656037ULL;
    for (int i = 0; i < n; i++) {
        h ^= (unsigned char)s[i];
        h *= 1099511628211ULL;
    }
    return (int64_t)h;
}
static int64_t fnv_text(const char* s) {
    return fnv_bytes(s, (int)strlen(s));
}
int64_t rt_d_hash_int(int64_t v, int nk) {
    char tmp[24];
    if (nk & 256) i64_dec(v, tmp);
    else u64_dec((uint64_t)v, tmp);
    return fnv_text(tmp);
}
int64_t rt_d_hash_bool(int v) {
    return fnv_text(v ? "true" : "false");
}
int64_t rt_d_hash_char(int c) {
    char u[4];
    return fnv_bytes(u, char_utf8((uint32_t)c, u));
}
int64_t rt_d_hash_float(double f, int nk) {
    char text[64];
    f = canon_float(f);
    rt_format_float(f, (nk & 255) == 32, text, sizeof text);
    return fnv_text(text);
}
static val rt_range_new(val lo, val hi, int inclusive);
val rt_binop(const char* op, val a, val b) {
    a = rt_deref(a);
    b = rt_deref(b);
    if (!strcmp(op, "==")) return rt_bool(rt_equal(a, b));
    if (!strcmp(op, "!=")) return rt_bool(!rt_equal(a, b));
    if (!strcmp(op, "..")) return rt_range_new(a, b, 0);
    if (!strcmp(op, "..=")) return rt_range_new(a, b, 1);
    if (a->kind != K_FLOAT) {
        if (!strcmp(op, "<")) return rt_bool(cmp_vals(a, b) < 0);
        if (!strcmp(op, "<=")) return rt_bool(cmp_vals(a, b) <= 0);
        if (!strcmp(op, ">")) return rt_bool(cmp_vals(a, b) > 0);
        if (!strcmp(op, ">=")) return rt_bool(cmp_vals(a, b) >= 0);
    }
    if (a->kind == K_FLOAT) {
        double x = a->u.f, y = b->u.f;
        /* IEEE ordering: every comparison with NaN is false, which a three-way cmp_vals cannot say. */
        if (!strcmp(op, "<")) return rt_bool(x < y);
        if (!strcmp(op, "<=")) return rt_bool(x <= y);
        if (!strcmp(op, ">")) return rt_bool(x > y);
        if (!strcmp(op, ">=")) return rt_bool(x >= y);
        switch (op[0]) {
        case '+':
            return rt_float(x + y, a->nk);
        case '-':
            return rt_float(x - y, a->nk);
        case '*':
            return rt_float(x * y, a->nk);
        case '/':
            return rt_float(x / y, a->nk);
        }
    }
    if (a->kind != K_INT) rt_panic("unsupported operands");
#ifdef KIGUMI_INT128
    if (W_IS(a->nk)) return w_binop(op, a, b);
#endif
    int64_t x = a->u.i, y = b->u.i;
    int nk = a->nk;
    switch (op[0]) {
    case '+':
        return rt_int(int_add(x, y, nk), nk);
    case '-':
        return rt_int(int_sub(x, y, nk), nk);
    case '*':
        return rt_int(int_mul(x, y, nk), nk);
    case '/':
        return rt_int(int_div(x, y, nk), nk);
    case '%':
        return rt_int(int_mod(x, y, nk), nk);
    case '&':
        return rt_int(x & y, nk);
    case '|':
        return rt_int(x | y, nk);
    case '^':
        return rt_int(int_xor(x, y, nk), nk);
    case '<':
        return rt_int(int_shl(x, y, nk), nk);
    case '>':
        return rt_int(int_shr(x, y, nk), nk);
    }
    rt_panicf("unsupported operator %s", op);
    return NULL;
}
val rt_unop(const char* op, val a) {
    a = rt_deref(a);
    if (op[0] == '!') return rt_bool(!a->u.b);
    if (op[0] == '-') {
        if (a->kind == K_FLOAT) return rt_float(-a->u.f, a->nk);
#ifdef KIGUMI_INT128
        if (W_IS(a->nk)) return w_unop('-', a);
#endif
        return rt_int(int_neg(a->u.i, a->nk), a->nk);
    }
#ifdef KIGUMI_INT128
    if (op[0] == '~' && W_IS(a->nk)) return w_unop('~', a);
#endif
    if (op[0] == '~') return rt_int(int_not(a->u.i, a->nk), a->nk);
    rt_panicf("unsupported unary %s", op);
    return NULL;
}
val rt_cast(val a, int nk) {
    a = rt_deref(a);
#ifdef KIGUMI_INT128
    if (a->kind == K_INT && (W_IS(a->nk) || W_IS(nk))) return w_convert(a, nk, 0);
#endif
    if (nk & 512) return rt_float(a->kind == K_FLOAT ? a->u.f : (double)a->u.i, nk);
    return rt_int(a->u.i, nk);
}

