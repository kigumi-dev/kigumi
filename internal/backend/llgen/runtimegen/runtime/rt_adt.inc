// ---- ADT helpers and init ----
rt_type* rt_find_type(const char* name) {
    for (int i = 0; i < g_ntypes; i++)
        if (!strcmp(short_name(g_types[i]->name), name) || !strcmp(g_types[i]->name, name)) return g_types[i];
    return NULL;
}
static rt_vdesc* find_variant(rt_type* t, const char* name) {
    if (!t) return NULL;
    for (int i = 0; i < t->nvariants; i++)
        if (!strcmp(t->variants[i]->name, name)) return t->variants[i];
    return NULL;
}
// Err(AllocError.OutOfMemory) as a file-scope immortal rt_init fills: reporting
// a failed allocation must not need one of its own, nor race a half-built one.
static struct obj g_alloc_error;
static val g_alloc_error_payload[1];
// The same refusal as a plain Error, for results typed `T!`.
static struct obj g_oom_text, g_oom_box, g_oom_error;
static val g_oom_error_payload[1];
#ifndef KIGUMI_ABI_FINGERPRINT
#define KIGUMI_ABI_FINGERPRINT "kgabi1:unbound"
#endif
const char kg_abi_fingerprint[] = KIGUMI_ABI_FINGERPRINT;
void rt_abi_check(const char* expected) {
    if (!strcmp(expected, kg_abi_fingerprint)) return;
    rt_sys_write(2, "fatal: runtime ABI mismatch (E1880): this program expects ", 58);
    rt_sys_write(2, expected, strlen(expected));
    rt_sys_write(2, ", the linked runtime is ", 24);
    rt_sys_write(2, kg_abi_fingerprint, strlen(kg_abi_fingerprint));
    rt_sys_write(2, "; rebuild both with one toolchain\n", 34);
    rt_sys_exit(2);
}
void rt_init(int argc, char** argv, int ntypes, rt_type** types) {
    rt_sys_stack_guard();
    g_argc = argc;
    g_argv = argv;
    g_ntypes = ntypes;
    g_types = types;
    rt_type *opt = rt_find_type("Option"), *res = rt_find_type("Result"), *ord = rt_find_type("Ordering");
    V_Some = find_variant(opt, "Some");
    V_None = find_variant(opt, "None");
    V_Ok = find_variant(res, "Ok");
    V_Err = find_variant(res, "Err");
    V_Less = find_variant(ord, "Less");
    V_Equal = find_variant(ord, "Equal");
    V_Greater = find_variant(ord, "Greater");
    rt_vdesc* oom = find_variant(rt_find_type("AllocError"), "OutOfMemory");
    if (oom && V_Err) {
        g_alloc_error_payload[0] = rt_nullary(oom);
        rt_const_variant(&g_alloc_error, g_alloc_error_payload, V_Err, 1);
    }
    g_oom_text.kind = K_OPAQUE;
    g_oom_text.rc = -1;
    g_oom_text.u.op.kind = "Error";
    g_oom_text.u.op.data = (void*)"out of memory";
    g_oom_box.kind = K_BOX;
    g_oom_box.rc = -1;
    g_oom_box.u.box.dyn = &T_Error;
    g_oom_box.u.box.v = &g_oom_text;
    g_oom_box.u.box.vt = T_Error_vt;
    g_oom_error_payload[0] = &g_oom_box;
    if (V_Err) rt_const_variant(&g_oom_error, g_oom_error_payload, V_Err, 1);
}
val rt_some(val v) {
    return rt_variant(V_Some, 1, &v);
}
val rt_none(void) {
    return rt_variant(V_None, 0, NULL);
}
val rt_ok(val v) {
    return rt_variant(V_Ok, 1, &v);
}
val rt_err_val(val e) {
    return rt_variant(V_Err, 1, &e);
}
val rt_err_msg(const char* msg) {
    val e = rt_box(rt_opaque("Error", (void*)msg), &T_Error, T_Error_vt);
    return rt_err_val(e);
}
static val alloc_error(void) {
    if (g_alloc_error.kind == 0) return rt_err_msg("out of memory");
    return &g_alloc_error;
}
static val oom_error(void) {
    return g_oom_error.kind == 0 ? rt_err_msg("out of memory") : &g_oom_error;
}
// NULL-on-refusal counterparts to alloc_in/rt_str_take/rt_opaque/rt_cell/rt_variant;
// callers release/free what they already hold on a NULL return.
static val alloc_in_try(rt_allocator* a, int kind) {
    val v = amalloc(a, sizeof(struct obj));
    if (!v) return NULL;
    LIVE_OBJ(kind, 1);
    v->kind = kind;
    v->rc = 1;
    v->owned = 0;
    v->al = a;
    return v;
}
static val alloc_try(int kind) {
    return alloc_in_try(cur_alloc(), kind);
}
static val rt_str_take_try(char* data, int64_t len) {
    val v = alloc_try(K_STR);
    if (!v) return NULL;
    v->u.s.data = data;
    v->u.s.len = len;
    v->owned = 1;
    return v;
}
static val rt_cell_try(val x) {
    val v = alloc_try(K_CELL);
    if (!v) return NULL;
    v->u.cell.v = x;
    return v;
}
static val opaque_try(const char* kind, void* data) {
    val v = alloc_try(K_OPAQUE);
    if (!v) return NULL;
    v->u.op.kind = kind;
    v->u.op.data = data;
    return v;
}
static val rt_variant_try(rt_vdesc* d, int64_t n, val* payload) {
    if (n == 0) return rt_nullary(d);
    val v = alloc_try(K_VARIANT);
    if (!v) return NULL;
    v->u.var.v = d;
    v->u.var.n = n;
    v->u.var.payload = amalloc(v->al, (n ? n : 1) * sizeof(val));
    if (!v->u.var.payload) {
        LIVE_OBJ(K_VARIANT, -1);
        afree(v->al, v);
        return NULL;
    }
    for (int64_t i = 0; i < n; i++) v->u.var.payload[i] = payload[i];
    return v;
}
// Set by rt_d_try while its op runs; try_ok clears it and hands back the payload unwrapped.
static KG_TLS int g_try_bare;
// try_ok wraps v as Ok(v), or releases v and reports the refusal when the
// Result wrapper itself can't be allocated.
static val try_ok(val v) {
    if (g_try_bare) {
        g_try_bare = 0;
        return v;
    }
    val r = rt_variant_try(V_Ok, 1, &v);
    if (!r) {
        rt_release(v);
        return alloc_error();
    }
    return r;
}
// Fallible deep copy and constructors behind the try* primitives: NULL means
// the allocator refused, and nothing built so far stays live.
static char* try_dup(const char* s, int64_t len) {
    if (len < 0 || fault_tick()) return NULL;
    char* d = kg_malloc_n(len);
    if (!d) return NULL;
    memcpy(d, s, len);
    d[len] = 0;
    return d;
}
static void discard_header(val v) {
    LIVE_OBJ(v->kind, -1);
    afree(v->al, v);
}
static val array_try_new(int64_t cap) {
    if (cap < 0 || (uint64_t)cap > SIZE_MAX / sizeof(val) / 2) return NULL;
    val v = alloc_try(K_ARRAY);
    if (!v) return NULL;
    if (cap > 0) {
        v->u.arr.items = amalloc(v->al, (size_t)cap * sizeof(val));
        if (!v->u.arr.items) {
            discard_header(v);
            return NULL;
        }
        v->u.arr.cap = cap;
    }
    return v;
}
static val record_try_new(rt_type* t, int64_t n) {
    val v = alloc_try(K_RECORD);
    if (!v) return NULL;
    v->u.rec.type = t;
    v->u.rec.fields = amalloc(v->al, (size_t)(n ? n : 1) * sizeof(val));
    if (!v->u.rec.fields) {
        discard_header(v);
        return NULL;
    }
    v->u.rec.n = n;
    return v;
}
static val copy_try(val v);
static int copy_slot_try(val* dst, val src) {
    *dst = copy_try(src);
    return *dst != NULL || src == NULL;
}
static val copy_try(val v) {
    if (!v || v->rc < 0) return v;
    switch (v->kind) {
    case K_RECORD: {
        if (v->u.rec.type->kind == 1) return rt_retain(v);
        val c = record_try_new(v->u.rec.type, v->u.rec.n);
        if (!c) return NULL;
        for (int64_t i = 0; i < v->u.rec.n; i++) {
            if ((v->u.rec.type->borrow_fields >> i) & 1) {
                c->u.rec.fields[i] = v->u.rec.fields[i];
            } else if (!copy_slot_try(&c->u.rec.fields[i], v->u.rec.fields[i])) {
                rt_release(c);
                return NULL;
            }
        }
        return c;
    }
    case K_VARIANT: {
        val c = rt_variant_try(v->u.var.v, v->u.var.n, v->u.var.payload);
        if (!c) return NULL;
        for (int64_t i = 0; i < c->u.var.n; i++) {
            if (!copy_slot_try(&c->u.var.payload[i], v->u.var.payload[i])) {
                for (int64_t j = i; j < c->u.var.n; j++) c->u.var.payload[j] = NULL;
                rt_release(c);
                return NULL;
            }
        }
        return c;
    }
    case K_ARRAY: {
        val c = array_try_new(v->u.arr.len);
        if (!c) return NULL;
        for (int64_t i = 0; i < v->u.arr.len; i++) {
            if (!copy_slot_try(&c->u.arr.items[i], v->u.arr.items[i])) {
                rt_release(c);
                return NULL;
            }
            c->u.arr.len++;
        }
        return c;
    }
    case K_BYTES: {
        val c = alloc_try(K_BYTES);
        if (!c) return NULL;
        c->u.s.len = v->u.s.len;
        if (!v->owned) {
            c->u.s.data = v->u.s.data;
            return c;
        }
        char* d = try_dup(v->u.s.data, v->u.s.len);
        if (!d) {
            discard_header(c);
            return NULL;
        }
        c->u.s.data = d;
        c->owned = 1;
        return c;
    }
    case K_BOX: {
        val c = alloc_try(K_BOX);
        if (!c) return NULL;
        c->u.box.dyn = v->u.box.dyn;
        c->u.box.vt = v->u.box.vt;
        c->u.box.src = NULL;
        if (!copy_slot_try(&c->u.box.v, v->u.box.v)) {
            discard_header(c);
            return NULL;
        }
        return c;
    }
    }
    return rt_retain(v);
}
static val array_try_copy(val src, int64_t s, int64_t e) {
    val out = array_try_new(e - s);
    if (!out) return NULL;
    for (int64_t i = 0; i < e - s; i++) {
        val x = src->u.arr.items[s + i];
        if (!copy_slot_try(&out->u.arr.items[i], x)) {
            rt_release(out);
            return NULL;
        }
        out->u.arr.len++;
    }
    return out;
}
// empty_record_try builds a std record whose n fields are all empty arrays.
static val empty_record_try(rt_type* t, int64_t n) {
    if (!t) rt_panic("a runtime-named type was dropped by reachability");
    val rec = record_try_new(t, n);
    if (!rec) return alloc_error();
    for (int64_t i = 0; i < n; i++) {
        rec->u.rec.fields[i] = array_try_new(0);
        if (!rec->u.rec.fields[i]) {
            rt_release(rec);
            return alloc_error();
        }
    }
    return try_ok(rec);
}
// try_ok_some wraps v as Ok(Some(v)).
static val try_ok_some(val v) {
    if (g_try_bare) {
        g_try_bare = 0;
        return v;
    }
    val s = rt_variant_try(V_Some, 1, &v);
    if (!s) {
        rt_release(v);
        return alloc_error();
    }
    return try_ok(s);
}
static val rt_ordering(int c) {
    return rt_variant(c < 0 ? V_Less : c > 0 ? V_Greater
                                             : V_Equal,
                      0,
                      NULL);
}
// `..`/`..=` are binary operators, so their operands are not consumed by
// the emitter and the range must retain them.
static val rt_range_new(val lo, val hi, int inclusive) {
    rt_type* t = rt_find_type("Range");
    val f[3] = {rt_retain(lo), rt_retain(hi), rt_bool(inclusive)};
    return rt_record(t, 3, f);
}
val rt_make_record(const char* tname, int n, val* fields) {
    rt_type* t = rt_find_type(tname);
    if (!t) rt_panicf("runtime type %s missing", tname);
    return rt_record(t, n, fields);
}

val rt_call_slot(val recv, int slot, int n, val* args, int recvmode);
#ifdef KIGUMI_STD_DYNAMIC
val rt_std(const char* key, int n, val* args);
#endif
static val error_message(val e) {
    return rt_call_slot(e, 0, 0, NULL, 0);
}
#ifdef KIGUMI_RC_CHECK
static void put_live(const char* label, size_t ln, long v) {
    char buf[48];
    int n = sizeof buf;
    int neg = v < 0;
    if (neg) v = -v;
    buf[--n] = '\n';
    do {
        buf[--n] = (char)('0' + v % 10);
        v /= 10;
    } while (v);
    if (neg) buf[--n] = '-';
    rt_sys_write(2, label, ln);
    rt_sys_write(2, buf + n, (size_t)(sizeof buf - n));
}
// Test-only: KIGUMI_LIVE_REPORT (or =kinds for a per-kind table) prints the objects still live at a clean exit.
static void report_live_objs(val r) {
    const char* want = getenv("KIGUMI_LIVE_REPORT");
    if (!want) return;
    rt_release(r);
    put_live("kigumi-live: ", 13, g_live_objs);
    if (strcmp(want, "kinds") != 0) return;
    for (int k = 1; k <= K_PTR; k++) {
        if (g_live_kinds[k]) {
            put_live("kigumi-live-kind ", 17, k);
            put_live("kigumi-live-count ", 18, g_live_kinds[k]);
        }
    }
}
#endif
// exit_t, ctx, wrapped and code are compile-time: ExitError's and Context's descriptors (null when no value of the type can exist), Context's "wrapped" field index and ExitError's "code" field index.
int rt_exit_code(val r, rt_type* exit_t, rt_type* ctx, int64_t wrapped, int64_t code) {
    r = rt_deref(r);
    if (r && r->kind == K_VARIANT && r->u.var.v == V_Err) {
        for (val cur = rt_deref(r->u.var.payload[0]); exit_t && cur->kind == K_BOX;) {
            if (cur->u.box.dyn == exit_t) return (int)rt_deref(cur->u.box.v->u.rec.fields[code])->u.i;
            if (!ctx || cur->u.box.dyn != ctx) break;
            cur = rt_deref(cur->u.box.v->u.rec.fields[wrapped]);
        }
        val m = rt_display(error_message(r->u.var.payload[0]));
        write_line(2, "error: ", 7, m->u.s.data, m->u.s.len);
        return 1;
    }
#ifdef KIGUMI_RC_CHECK
    report_live_objs(r);
#endif
    return 0;
}
// The argument array lives in this frame: an adapter only rt_at's out of it,
// and it is immortal so none can free it. args are consumed here, as before.
static val invoke_args(adapter_fn fn, val env, int n, val* args, int owned) {
    struct obj arr = {.kind = K_ARRAY, .rc = -1, .u.arr = {.items = args, .len = n, .cap = n}};
#ifdef KG_CATCH
    UNWIND_PUSH(u, owned ? unwind_args_region : unwind_args, args, NULL, n);
    val r = fn(env, &arr);
    u.run(&u);
    UNWIND_POP(u);
#else
    val r = fn(env, &arr);
    for (int i = n - 1; i >= 0; i--) rt_release(args[i]);
    if (owned) gen_free(&g_general, args);
#endif
    return r;
}
