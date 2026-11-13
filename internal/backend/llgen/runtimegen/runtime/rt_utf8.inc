
// RFC 3629 / Unicode Table 3-7: rejects overlong encodings, surrogates
// (D800-DFFF) and codepoints above U+10FFFF, matching Go's unicode/utf8.Valid.
int rt_utf8_valid(const char* s, int64_t n) {
    int64_t i = 0;
    while (i < n) {
        unsigned char c = s[i];
        if (c < 0x80) {
            i++;
            continue;
        }
        int len;
        uint32_t min, cp;
        if ((c & 0xE0) == 0xC0) {
            len = 2;
            min = 0x80;
            cp = c & 0x1F;
        } else if ((c & 0xF0) == 0xE0) {
            len = 3;
            min = 0x800;
            cp = c & 0x0F;
        } else if ((c & 0xF8) == 0xF0) {
            len = 4;
            min = 0x10000;
            cp = c & 0x07;
        } else {
            return 0;
        }
        if (i + len > n) return 0;
        for (int k = 1; k < len; k++) {
            unsigned char cc = s[i + k];
            if ((cc & 0xC0) != 0x80) return 0;
            cp = (cp << 6) | (cc & 0x3F);
        }
        if (cp < min || cp > 0x10FFFF || (cp >= 0xD800 && cp <= 0xDFFF)) return 0;
        i += len;
    }
    return 1;
}
// rt_hash_of is FNV-1a-64 (standard offset basis 0xcbf29ce484222325 /
// 14695981039346656037) over the canon rendering of v, so this and
// internal/hashkey.Sum64 (interp, vm) agree byte for byte.
uint64_t rt_hash_of(val v) {
    buf b = {0};
    rt_bputs(&b, "");
    display_into(&b, v, 1);
    uint64_t h = 14695981039346656037ULL;
    for (size_t i = 0; i < b.len; i++) {
        h ^= (unsigned char)b.p[i];
        h *= 1099511628211ULL;
    }
    free(b.p);
    return h;
}
// A fixed seed stands in on targets without an entropy source; workers racing the first draw keep the CAS winner.
static uint64_t map_seed(void) {
    static KG_I64 seed;
    int64_t s = KG_ALOAD(seed);
    if (s) return (uint64_t)s;
    int64_t drawn = rt_sys_seed();
    if (!drawn) drawn = (int64_t)0x1e3779b97f4a7c15ULL;
#ifdef KIGUMI_THREADS
    if (!atomic_compare_exchange_strong(&seed, &s, drawn)) return (uint64_t)s;
#else
    seed = drawn;
#endif
    return (uint64_t)drawn;
}
int64_t rt_d_map_seed(void) { return (int64_t)map_seed(); }
// Constant-time compare: time
// depends only on the lengths (an early return there is fine, lengths
// aren't secret), never on where the bytes differ. `diff` is volatile so
// the OR loop can't be short-circuited or vectorized away at any -O level.
static int subtle_constant_time_eq(val a, val b) {
    int64_t n = a->u.s.len;
    if (n != b->u.s.len) return 0;
    volatile uint8_t diff = 0;
    for (int64_t i = 0; i < n; i++) {
        diff |= (uint8_t)a->u.s.data[i] ^ (uint8_t)b->u.s.data[i];
    }
    return diff == 0;
}
static uint64_t secret_eq_bytes(val a, val b) {
    return (uint64_t)0 - (uint64_t)subtle_constant_time_eq(a, b);
}
