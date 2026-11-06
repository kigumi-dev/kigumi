// Fallible text and collection primitives. NULL or alloc_error() means the
// allocator refused; nothing built so far stays live and no argument changed.

static val try_array_done(val out, int ok) {
    if (ok) return try_ok(out);
    rt_release(out);
    return alloc_error();
}
static int utf8_width(unsigned char lead) {
    return lead >= 240 ? 4 : lead >= 224 ? 3 : lead >= 192 ? 2 : 1;
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
static int sort_less(val order, val b, val a) {
    val args[2] = {b, a};
    val r = rt_callv_lent(order, 2, args);
    int less = rt_deref(r)->u.var.v == V_Less;
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
    val out = array_try_copy(items, 0, items->u.arr.len);
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
static val try_builder_freeze(val rec) {
    val arr = rt_deref(rec->u.rec.fields[0]);
    int64_t n = arr->u.arr.len;
    char* d = n < 0 || fault_tick() ? NULL : kg_malloc_n(n);
    if (!d) return oom_error();
    for (int64_t i = 0; i < n; i++) d[i] = (char)rt_deref(arr->u.arr.items[i])->u.i;
    d[n] = 0;
    if (!rt_utf8_valid(d, n)) {
        free(d);
        return rt_err_msg("invalid UTF-8");
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
    rt_bputs(&b, "");
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
