// ---- strings and rt_display ----
static KG_TLS int g_buf_try, g_buf_failed;
void rt_bput(buf* b, const char* s, size_t n) {
    if (g_buf_try && g_buf_failed) return;
    if (b->len + n + 1 > b->cap) {
        if (g_buf_try && fault_tick()) {
            g_buf_failed = 1;
            return;
        }
        if (n > SIZE_MAX - 1 || b->len > SIZE_MAX - 1 - n) rt_panic("out of memory");
        size_t need = b->len + n + 1;
        if (need > (SIZE_MAX - 16) / 2) rt_panic("out of memory");
        size_t cap = need * 2 + 16;
        char* p = realloc(b->p, cap);
        if (!p && g_buf_try) {
            g_buf_failed = 1;
            return;
        }
        if (!p) rt_panic("out of memory");
        b->p = p;
        b->cap = cap;
    }
    memcpy(b->p + b->len, s, n);
    b->len += n;
    b->p[b->len] = 0;
}
void rt_bputs(buf* b, const char* s) {
    rt_bput(b, s, strlen(s));
}
static const char* short_name(const char* name) {
    const char* d = strrchr(name, '.');
    return d ? d + 1 : name;
}
// Base-10 text of v into out (NUL-terminated); out must hold 21 bytes.
static void u64_dec(uint64_t v, char* out) {
    char tmp[20];
    int n = 0;
    do {
        tmp[n++] = (char)('0' + v % 10);
        v /= 10;
    } while (v);
    for (int i = 0; i < n; i++) out[i] = tmp[n - 1 - i];
    out[n] = 0;
}
static void i64_dec(int64_t v, char* out) {
    if (v >= 0) {
        u64_dec((uint64_t)v, out);
        return;
    }
    *out++ = '-';
    // INT64_MIN negated overflows int64_t; take the magnitude via uint64_t.
    u64_dec(v == INT64_MIN ? (uint64_t)INT64_MAX + 1 : (uint64_t)(-v), out);
}
// "index out of range: index I, length N at SITE"; the VM prints the same text.
void rt_panic_index(int64_t i, int64_t len, const char* site) {
    char msg[320];
    size_t n = 0;
    const char* head = "index out of range: index ";
    memcpy(msg, head, strlen(head));
    n = strlen(head);
    i64_dec(i, msg + n);
    n += strlen(msg + n);
    memcpy(msg + n, ", length ", 9);
    n += 9;
    i64_dec(len, msg + n);
    n += strlen(msg + n);
    if (site && *site) {
        size_t k = strlen(site);
        if (k > 200) k = 200;
        memcpy(msg + n, " at ", 4);
        memcpy(msg + n + 4, site, k);
        n += 4 + k;
    }
    msg[n] = 0;
    rt_panic(msg);
}
// "0x" + lowercase hex of p, no leading zeros (out must hold 2+16+1 bytes).
static void ptr_hex(const void* p, char* out) {
    uintptr_t v = (uintptr_t)p;
    *out++ = '0';
    *out++ = 'x';
    char tmp[16];
    int n = 0;
    do {
        tmp[n++] = "0123456789abcdef"[v & 0xf];
        v >>= 4;
    } while (v);
    for (int i = 0; i < n; i++) out[i] = tmp[n - 1 - i];
    out[n] = 0;
}
static int char_utf8(uint32_t c, char u[4]) {
    if (c < 0x80) {
        u[0] = c;
        return 1;
    }
    if (c < 0x800) {
        u[0] = 0xC0 | (c >> 6);
        u[1] = 0x80 | (c & 0x3F);
        return 2;
    }
    if (c < 0x10000) {
        u[0] = 0xE0 | (c >> 12);
        u[1] = 0x80 | ((c >> 6) & 0x3F);
        u[2] = 0x80 | (c & 0x3F);
        return 3;
    }
    u[0] = 0xF0 | (c >> 18);
    u[1] = 0x80 | ((c >> 12) & 0x3F);
    u[2] = 0x80 | ((c >> 6) & 0x3F);
    u[3] = 0x80 | (c & 0x3F);
    return 4;
}
static double canon_float(double f) {
    if (f == 0.0) return 0.0;
    if (f != f) return __builtin_nan("");
    return f;
}
static int is_tuple_name(const char* name) {
    if (strncmp(name, "Tuple", 5) != 0 || !name[5]) return 0;
    for (const char* c = name + 5; *c; c++)
        if (*c < '0' || *c > '9') return 0;
    return 1;
}
// `Map { k: v, ... }` in insertion order; the hash index is not part of what a map is.
static void display_map(buf* b, val v) {
    val keys = rt_deref(v->u.rec.fields[0]), vals = rt_deref(v->u.rec.fields[1]);
    rt_bputs(b, "Map {");
    for (int64_t i = 0; i < keys->u.arr.len; i++) {
        if (i) rt_bputs(b, ",");
        rt_bputs(b, " ");
        display_into(b, keys->u.arr.items[i], 0);
        rt_bputs(b, ": ");
        display_into(b, vals->u.arr.items[i], 0);
    }
    if (keys->u.arr.len > 0) rt_bputs(b, " ");
    rt_bputs(b, "}");
}
// `P { x: 1, y: 2 }`, or `(1, 2)` for a tuple; hashing (canon) keeps the bare `P { 1, 2 }` form.
static void display_record(buf* b, val v, int canon) {
    const rt_type* t = v->u.rec.type;
    if (!canon && v->u.rec.n == 5 && strcmp(t->name, "std/map.Map") == 0) {
        display_map(b, v);
        return;
    }
    const char* name = short_name(t->name);
    int named = !canon && t->fields && t->nfields == v->u.rec.n;
    if (named && is_tuple_name(name)) {
        rt_bputs(b, "(");
        for (int64_t i = 0; i < v->u.rec.n; i++) {
            if (i) rt_bputs(b, ", ");
            display_into(b, v->u.rec.fields[i], canon);
        }
        rt_bputs(b, ")");
        return;
    }
    rt_bputs(b, name);
    rt_bputs(b, " {");
    for (int64_t i = 0; i < v->u.rec.n; i++) {
        if (i) rt_bputs(b, ",");
        rt_bputs(b, " ");
        if (named) {
            rt_bputs(b, t->fields[i]);
            rt_bputs(b, ": ");
            if ((t->private_fields >> i) & 1) {
                rt_bputs(b, "<private>");
                continue;
            }
        }
        display_into(b, v->u.rec.fields[i], canon);
    }
    rt_bputs(b, " }");
}
// canon folds -0.0 and every NaN so hashing matches total float equality.
void display_into(buf* b, val v, int canon) {
    char tmp[64];
    v = rt_deref(v);
    switch (v->kind) {
    case K_INT:
#ifdef KIGUMI_INT128
        if (W_IS(v->nk)) {
            char wide[48];
            w_dec(w_of(v), W_SIGNED(v->nk), wide);
            rt_bputs(b, wide);
            break;
        }
#endif
        if (v->nk & 256) i64_dec(v->u.i, tmp);
        else u64_dec((uint64_t)v->u.i, tmp);
        rt_bputs(b, tmp);
        break;
    case K_FLOAT: {
        char text[64];
        double f = v->u.f;
        if (canon) f = canon_float(f);
        rt_format_float(f, (v->nk & 255) == 32, text, sizeof text);
        rt_bputs(b, text);
        break;
    }
    case K_BOOL:
        rt_bputs(b, v->u.b ? "true" : "false");
        break;
    case K_CHAR: {
        char u[4];
        rt_bput(b, u, char_utf8(v->u.c, u));
        break;
    }
    case K_STR:
    case K_BYTES:
        rt_bput(b, v->u.s.data, v->u.s.len);
        break;
    case K_UNIT:
        rt_bputs(b, "()");
        break;
    case K_PTR:
        rt_bputs(b, "<ptr ");
        ptr_hex(v->u.p, tmp);
        rt_bputs(b, tmp);
        rt_bputs(b, ">");
        break;
    case K_VARIANT:
        rt_bputs(b, v->u.var.v->name);
        if (v->u.var.n) {
            rt_bputs(b, "(");
            for (int64_t i = 0; i < v->u.var.n; i++) {
                if (i) rt_bputs(b, ", ");
                display_into(b, v->u.var.payload[i], canon);
            }
            rt_bputs(b, ")");
        }
        break;
    case K_RECORD:
        display_record(b, v, canon);
        break;
    case K_ARRAY:
        rt_bputs(b, "[");
        for (int64_t i = 0; i < v->u.arr.len; i++) {
            if (i) rt_bputs(b, ", ");
            display_into(b, v->u.arr.items[i], canon);
        }
        rt_bputs(b, "]");
        break;
    case K_BOX:
        display_into(b, v->u.box.v, canon);
        break;
    case K_OPAQUE:
        if (strcmp(v->u.op.kind, "Box") == 0) display_into(b, ((val)v->u.op.data)->u.cell.v, canon);
        else if (strcmp(v->u.op.kind, "Error") == 0 || strcmp(v->u.op.kind, "Path") == 0) rt_bputs(b, (const char*)v->u.op.data);
        else {
            rt_bputs(b, "<");
            rt_bputs(b, v->u.op.kind);
            rt_bputs(b, ">");
        }
        break;
    default:
        rt_bputs(b, "<value>");
    }
}
val rt_display(val v) {
    buf b = {0};
    rt_bputs(&b, "");
    display_into(&b, v, 0);
    return rt_str_take(b.p, b.len);
}
val rt_interp(int n, val* items) {
    buf b = {0};
    rt_bputs(&b, "");
    for (int i = 0; i < n; i++) display_into(&b, items[i], 0);
    return rt_str_take(b.p, b.len);
}
const char* rt_cstr(val s) {
    s = rt_deref(s);
    if (s->kind == K_BOX) s = s->u.box.v;
    if (s->kind == K_OPAQUE) return (const char*)s->u.op.data;
    if (s->u.s.len < 0) rt_panic("out of memory");
    char* d = malloc((uint64_t)s->u.s.len + 1);
    if (!d) rt_panic("out of memory");
    memcpy(d, s->u.s.data, s->u.s.len);
    d[s->u.s.len] = 0;
    return d;
}

