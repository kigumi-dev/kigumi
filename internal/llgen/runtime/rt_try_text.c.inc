// Fallible text and collection primitives. NULL or alloc_error() means the
// allocator refused; nothing built so far stays live and no argument changed.

static char* try_bytes(int64_t n) {
    if (n < 0 || fault_tick()) return NULL;
    return kg_malloc_n(n);
}
static val try_piece(const char* s, int64_t n) {
    char* d = try_dup(s, n);
    if (!d) return NULL;
    val v = rt_str_take_try(d, n);
    if (!v) free(d);
    return v;
}
static val try_str_take(char* d, int64_t n) {
    d[n] = 0;
    val v = rt_str_take_try(d, n);
    if (!v) {
        free(d);
        return alloc_error();
    }
    return try_ok(v);
}
static int try_push_val(val arr, val x) {
    if (!x) return 0;
    if (!try_grow(arr, 1)) {
        rt_release(x);
        return 0;
    }
    arr->u.arr.items[arr->u.arr.len++] = x;
    return 1;
}
static int try_push_piece(val arr, const char* s, int64_t n) {
    return try_push_val(arr, try_piece(s, n));
}
static val try_array_done(val out, int ok) {
    if (ok) return try_ok(out);
    rt_release(out);
    return alloc_error();
}
static int utf8_width(unsigned char lead) {
    return lead >= 240 ? 4 : lead >= 224 ? 3 : lead >= 192 ? 2 : 1;
}
static val try_string_trim(val s) {
    const char* d = s->u.s.data;
    int64_t lo = 0, hi = s->u.s.len;
    while (lo < hi && (d[lo] == 32 || d[lo] == 9 || d[lo] == 13 || d[lo] == 10)) lo++;
    while (hi > lo && (d[hi - 1] == 32 || d[hi - 1] == 9 || d[hi - 1] == 13 || d[hi - 1] == 10)) hi--;
    val v = try_piece(d + lo, hi - lo);
    return v ? try_ok(v) : alloc_error();
}
static val try_string_shift(val s, unsigned char lo, unsigned char hi, int down) {
    char* d = try_dup(s->u.s.data, s->u.s.len);
    if (!d) return alloc_error();
    for (int64_t i = 0; i < s->u.s.len; i++) {
        unsigned char b = (unsigned char)d[i];
        if (b >= lo && b <= hi) d[i] = (char)(down ? b + 32 : b - 32);
    }
    val v = rt_str_take_try(d, s->u.s.len);
    if (!v) {
        free(d);
        return alloc_error();
    }
    return try_ok(v);
}
static val try_string_chars(val s) {
    const unsigned char* d = (const unsigned char*)s->u.s.data;
    int64_t n = s->u.s.len, count = 0;
    for (int64_t i = 0; i < n; i += utf8_width(d[i])) count++;
    val out = array_try_new(count);
    if (!out) return alloc_error();
    for (int64_t i = 0; i < n;) {
        int w = utf8_width(d[i]);
        uint32_t point = w == 4 ? d[i] & 7 : w == 3 ? d[i] & 15 : w == 2 ? d[i] & 31 : d[i];
        for (int k = 1; k < w && i + k < n; k++) point = (point << 6) | (d[i + k] & 63);
        val c = alloc_try(K_CHAR);
        if (!c) return try_array_done(out, 0);
        c->u.c = point;
        out->u.arr.items[out->u.arr.len++] = c;
        i += w;
    }
    return try_ok(out);
}
static val try_string_split(val s, val sep) {
    val out = array_try_new(0);
    if (!out) return alloc_error();
    const char* d = s->u.s.data;
    int64_t n = s->u.s.len, m = sep->u.s.len;
    if (m == 0) return try_array_done(out, try_push_val(out, rt_retain(s)));
    int64_t start = 0, i = 0;
    while (i + m <= n) {
        if (memcmp(d + i, sep->u.s.data, (size_t)m) != 0) {
            i++;
            continue;
        }
        if (!try_push_piece(out, d + start, i - start)) return try_array_done(out, 0);
        i += m;
        start = i;
    }
    return try_array_done(out, try_push_piece(out, d + start, n - start));
}
static val try_text_lines(val s) {
    val out = array_try_new(0);
    if (!out) return alloc_error();
    const char* d = s->u.s.data;
    int64_t end = s->u.s.len, start = 0;
    if (end == 0) return try_ok(out);
    if (d[end - 1] == 10) end--;
    for (int64_t i = 0; i <= end; i++) {
        if (i != end && d[i] != 10) continue;
        int64_t stop = i;
        if (stop > start && d[stop - 1] == 13) stop--;
        if (!try_push_piece(out, d + start, stop - start)) return try_array_done(out, 0);
        start = i + 1;
    }
    return try_ok(out);
}
static val try_text_graphemes(val s) {
    const unsigned char* d = (const unsigned char*)s->u.s.data;
    int64_t n = s->u.s.len, count = 0;
    for (int64_t i = 0; i < n; i += utf8_width(d[i])) count++;
    val out = array_try_new(count);
    if (!out) return alloc_error();
    for (int64_t i = 0; i < n;) {
        int64_t w = utf8_width(d[i]);
        if (i + w > n) w = n - i;
        if (!try_push_piece(out, (const char*)d + i, w)) return try_array_done(out, 0);
        i += w;
    }
    return try_ok(out);
}
static val try_text_replace(val s, val old, val repl) {
    int64_t n = s->u.s.len, m = old->u.s.len, r = repl->u.s.len, hits = 0;
    if (m == 0) return try_ok(rt_retain(s));
    const char* d = s->u.s.data;
    for (int64_t i = 0; i + m <= n;) {
        if (memcmp(d + i, old->u.s.data, (size_t)m) == 0) {
            hits++;
            i += m;
        } else {
            i++;
        }
    }
    char* out = try_bytes(n + hits * (r - m));
    if (!out) return alloc_error();
    int64_t o = 0, i = 0;
    while (i < n) {
        if (i + m <= n && memcmp(d + i, old->u.s.data, (size_t)m) == 0) {
            memcpy(out + o, repl->u.s.data, (size_t)r);
            o += r;
            i += m;
        } else {
            out[o++] = d[i++];
        }
    }
    return try_str_take(out, o);
}
static val try_text_trim_edge(val s, val edge, int suffix) {
    int64_t n = s->u.s.len, m = edge->u.s.len;
    const char* at = suffix ? s->u.s.data + n - m : s->u.s.data;
    if (m == 0 || m > n || memcmp(at, edge->u.s.data, (size_t)m) != 0) return try_ok(rt_retain(s));
    val v = try_piece(suffix ? s->u.s.data : s->u.s.data + m, n - m);
    return v ? try_ok(v) : alloc_error();
}
static val try_array_join(val items, val sep) {
    int64_t n = items->u.arr.len, total = 0;
    for (int64_t i = 0; i < n; i++) total += deref(items->u.arr.items[i])->u.s.len;
    if (n > 1) total += (n - 1) * sep->u.s.len;
    char* out = try_bytes(total);
    if (!out) return alloc_error();
    int64_t o = 0;
    for (int64_t i = 0; i < n; i++) {
        val x = deref(items->u.arr.items[i]);
        if (i) {
            memcpy(out + o, sep->u.s.data, (size_t)sep->u.s.len);
            o += sep->u.s.len;
        }
        memcpy(out + o, x->u.s.data, (size_t)x->u.s.len);
        o += x->u.s.len;
    }
    return try_str_take(out, o);
}
static int sort_less(val order, val b, val a) {
    val args[2] = {b, a};
    val r = rt_callv(order, 2, args);
    int less = deref(r)->u.var.v == V_Less;
    rt_release(r);
    return less;
}
static void sort_run(val order, val* items, val* tmp, int64_t n) {
    if (n <= 1) return;
    int64_t mid = n / 2;
    sort_run(order, items, tmp, mid);
    sort_run(order, items + mid, tmp, n - mid);
    int64_t l = 0, r = mid, o = 0;
    while (l < mid && r < n) tmp[o++] = sort_less(order, items[r], items[l]) ? items[r++] : items[l++];
    while (l < mid) tmp[o++] = items[l++];
    while (r < n) tmp[o++] = items[r++];
    memcpy(items, tmp, (size_t)n * sizeof(val));
}
#ifdef KG_CATCH
static void unwind_sort(kg_unwind* u) {
    free(u->p);
    rt_release(u->v);
}
#endif
static val try_array_sorted_by(val items, val order) {
    val out = array_try_copy(items, 0, items->u.arr.len, 0);
    if (!out) return alloc_error();
    val* tmp = NULL;
    if (out->u.arr.len > 1) {
        tmp = fault_tick() ? NULL : malloc((size_t)out->u.arr.len * sizeof(val));
        if (!tmp) return try_array_done(out, 0);
    }
    UNWIND_PUSH(u, unwind_sort, tmp, out, 0);
    sort_run(order, out->u.arr.items, tmp, out->u.arr.len);
    UNWIND_POP(u);
    free(tmp);
    return try_ok(out);
}
static val try_builder_push(val rec, val s) {
    val arr = deref(rec->u.rec.fields[0]);
    int64_t n = s->u.s.len, len = arr->u.arr.len;
    val unit = rt_unit();
    val done = rt_variant_try(V_Ok, 1, &unit);
    if (!done) return alloc_error();
    if (!try_grow(arr, (uint64_t)n)) {
        rt_release(done);
        return alloc_error();
    }
    for (int64_t i = 0; i < n; i++) {
        val b = alloc_try(K_INT);
        if (!b) {
            for (int64_t j = 0; j < i; j++) rt_release(arr->u.arr.items[len + j]);
            rt_release(done);
            return alloc_error();
        }
        b->u.i = (uint8_t)s->u.s.data[i];
        b->nk = 8;
        arr->u.arr.items[len + i] = b;
    }
    arr->u.arr.len = len + n;
    return done;
}
static val try_builder_push_byte(val rec, val b) {
    val arr = deref(rec->u.rec.fields[0]);
    val unit = rt_unit();
    val done = rt_variant_try(V_Ok, 1, &unit);
    if (!done) return alloc_error();
    val x = try_grow(arr, 1) ? alloc_try(K_INT) : NULL;
    if (!x) {
        rt_release(done);
        return alloc_error();
    }
    x->u.i = (uint8_t)b->u.i;
    x->nk = 8;
    arr->u.arr.items[arr->u.arr.len++] = x;
    return done;
}
static val try_builder_freeze(val rec) {
    val arr = deref(rec->u.rec.fields[0]);
    int64_t n = arr->u.arr.len;
    char* d = try_bytes(n);
    if (!d) return oom_error();
    for (int64_t i = 0; i < n; i++) d[i] = (char)deref(arr->u.arr.items[i])->u.i;
    d[n] = 0;
    if (!utf8_valid(d, n)) {
        free(d);
        return err_msg("invalid UTF-8");
    }
    val v = rt_str_take_try(d, n);
    if (!v) {
        free(d);
        return oom_error();
    }
    val r = rt_variant_try(V_Ok, 1, &v);
    if (!r) {
        rt_release(v);
        return oom_error();
    }
    return r;
}
static val try_debug(val v) {
    buf b = {0};
    g_buf_try = 1;
    g_buf_failed = 0;
    bputs(&b, "");
    display_into(&b, v, 0);
    g_buf_try = 0;
    if (g_buf_failed) {
        g_buf_failed = 0;
        free(b.p);
        return alloc_error();
    }
    val s = rt_str_take_try(b.p, (int64_t)b.len);
    if (!s) {
        free(b.p);
        return alloc_error();
    }
    return try_ok(s);
}
