val rt_unit(void) {
    static struct obj u = {.kind = K_UNIT, .rc = -1};
    return &u;
}
// Integers in [-128, 1024) at widths 8 to 64 (the range the VM interns) and ASCII chars are static immortals, so boxing one allocates nothing.
#define SMALL_INT_MIN (-128)
#define SMALL_INT_N 1152
static struct obj g_small_int[8][SMALL_INT_N];
static struct obj g_small_char[128];
static KG_AINT g_small_filled[9];
#ifdef KIGUMI_THREADS
static pthread_mutex_t g_small_lock = PTHREAD_MUTEX_INITIALIZER;
#define SMALL_LOCK() (kg_stack_check_locked(), pthread_mutex_lock(&g_small_lock))
#define SMALL_UNLOCK() pthread_mutex_unlock(&g_small_lock)
#else
#define SMALL_LOCK() ((void)0)
#define SMALL_UNLOCK() ((void)0)
#endif
static int small_int_slot(int nk) {
    int w = nk & 255;
    if ((nk & ~(255 | 256)) || (w != 8 && w != 16 && w != 32 && w != 64)) return -1;
    int s = w == 8 ? 0 : w == 16 ? 1 : w == 32 ? 2 : 3;
    return nk & 256 ? s + 4 : s;
}
static void fill_small(int s) {
    if (KG_ALOAD(g_small_filled[s])) return;
    SMALL_LOCK();
    if (KG_ALOAD(g_small_filled[s])) {
        SMALL_UNLOCK();
        return;
    }
    if (s == 8) {
        for (int c = 0; c < 128; c++) g_small_char[c] = (struct obj){.kind = K_CHAR, .rc = -1, .u.c = (uint32_t)c};
    } else {
        int nk = (8 << (s & 3)) | (s >= 4 ? 256 : 0);
        for (int n = 0; n < SMALL_INT_N; n++)
            g_small_int[s][n] = (struct obj){.kind = K_INT, .nk = nk, .rc = -1, .u.i = n + SMALL_INT_MIN};
    }
    KG_ASTORE(g_small_filled[s], 1);
    SMALL_UNLOCK();
}
static val small_int(int64_t i, int nk) {
    if (i < SMALL_INT_MIN || i >= SMALL_INT_MIN + SMALL_INT_N) return NULL;
    int s = small_int_slot(nk);
    if (s < 0) return NULL;
    fill_small(s);
    return &g_small_int[s][i - SMALL_INT_MIN];
}
val rt_int(int64_t i, int nk) {
    if (!nk) nk = 64 | 256;
    val p = small_int(i, nk);
    if (p) return p;
    val v = alloc(K_INT);
    v->u.i = i;
    v->nk = nk;
    return v;
}
// An f32 value is stored as the double nearest to its single-precision
// rounding, so every operation on it rounds the way IEEE single does.
val rt_float(double f, int nk) {
    val v = alloc(K_FLOAT);
    v->u.f = (nk & 255) == 32 ? (double)(float)f : f;
    v->nk = nk;
    return v;
}
val rt_bool(int b) {
    static struct obj t = {K_BOOL, 0, -1, 0, NULL, {.b = 1}}, f = {K_BOOL, 0, -1, 0, NULL, {.b = 0}};
    return b ? &t : &f;
}
val rt_char(uint32_t c) {
    if (c < 128) {
        fill_small(8);
        return &g_small_char[c];
    }
    val v = alloc(K_CHAR);
    v->u.c = c;
    return v;
}
val rt_str(const char* data, int64_t len) {
    val v = alloc(K_STR);
    v->u.s.data = data;
    v->u.s.len = len;
    return v;
}
val rt_bytes(const char* data, int64_t len) {
    val v = alloc(K_BYTES);
    v->u.s.data = data;
    v->u.s.len = len;
    return v;
}
val rt_str_take(char* data, int64_t len) {
    val v = rt_str(data, len);
    v->owned = 1;
    return v;
}
val strdup_val(const char* s, int64_t len) {
    if (len < 0) rt_panic("out of memory");
    char* d = kg_malloc_n(len);
    if (!d) rt_panic("out of memory");
    memcpy(d, s, len);
    d[len] = 0;
    return rt_str_take(d, len);
}
val bytes_val(const char* s, int64_t len) {
    val v = strdup_val(s, len);
    v->kind = K_BYTES;
    return v;
}
val rt_cell(val x) {
    val v = alloc(K_CELL);
    v->u.cell.v = x;
    return v;
}
// A `mut self` receiver's cell never outlives the call it is built for, so
// the caller hands over frame storage instead of allocating.
void rt_cell_init(void* slot, val x) {
    val v = (val)slot;
    v->kind = K_CELL;
    v->rc = -1;
    v->u.cell.v = x;
}
val rt_cellget(val c) {
    return rt_retain(c->u.cell.v);
}
// The caller takes back what it lent a `mut self` receiver: the cell's own
// reference moves to the result, so a frame-resident cell leaks nothing.
val rt_cell_take(val c) {
    val v = c->u.cell.v;
    c->u.cell.v = NULL;
    return v;
}
val rt_cellset_swap(val c, val x) {
    val old = c->u.cell.v;
    c->u.cell.v = x;
    return old;
}
void rt_cellset(val c, val x) {
    rt_release(rt_cellset_swap(c, x));
}
val rt_deref(val v);
val rt_ptr(void* p) {
    val v = alloc(K_PTR);
    v->u.p = p;
    return v;
}
void* rt_ptr_val(val v) {
    v = rt_deref(v);
    return v->kind == K_PTR ? v->u.p : NULL;
}
val rt_opaque(const char* kind, void* data) {
    val v = alloc(K_OPAQUE);
    v->u.op.kind = kind;
    v->u.op.data = data;
    return v;
}

static void release_moved(val* items, int64_t n, uint64_t borrowed) {
    for (int64_t i = 0; i < n; i++) {
        if (items[i] && !(i < 64 && ((borrowed >> i) & 1))) rt_release(items[i]);
    }
}
val rt_array(int64_t n, val* items) {
    val v = alloc(K_ARRAY);
    if (n == 0) {
        v->u.arr.cap = 0;
        v->u.arr.items = NULL;
        v->u.arr.len = 0;
        return v;
    }
    v->u.arr.cap = n < 4 ? 4 : n;
    v->u.arr.items = amalloc(v->al, v->u.arr.cap * sizeof(val));
    if (!v->u.arr.items) {
        discard_header(v);
        rt_panic("out of memory");
    }
    for (int64_t i = 0; i < n; i++) v->u.arr.items[i] = items[i];
    v->u.arr.len = n;
    return v;
}
// try_grow makes room for extra more items; 0 when the allocator refuses.
static int try_grow(val a, uint64_t extra) {
    uint64_t len = (uint64_t)a->u.arr.len;
    if (extra > UINT64_MAX - len) return 0;
    uint64_t need = len + extra;
    if (need <= (uint64_t)a->u.arr.cap) return 1;
    if (a->u.arr.head) {
        val* base = a->u.arr.items - a->u.arr.head;
        for (int64_t j = 0; j < a->u.arr.len; j++) base[j] = a->u.arr.items[j];
        a->u.arr.items = base;
        a->u.arr.cap += a->u.arr.head;
        a->u.arr.head = 0;
        if (need <= (uint64_t)a->u.arr.cap) return 1;
    }
    if (need > SIZE_MAX / sizeof(val) / 2) return 0;
    uint64_t cap = (uint64_t)a->u.arr.cap * 2 > need ? (uint64_t)a->u.arr.cap * 2 : need;
    val* items = arealloc(a->al, a->u.arr.items, a->u.arr.len * sizeof(val), cap * sizeof(val));
    if (!items) return 0;
    a->u.arr.items = items;
    a->u.arr.cap = (int64_t)cap;
    return 1;
}
void rt_push(val a, val x) {
    if (!try_grow(a, 1)) rt_panic("out of memory");
    a->u.arr.items[a->u.arr.len++] = x;
}
void rt_array_push(val a, val x) {
    rt_push(a, x);
}
val rt_at(val a, int64_t i) {
    if (!a || i < 0 || i >= a->u.arr.len) return rt_unit();
    return rt_retain(a->u.arr.items[i]);
}
// A record never owns what a borrow field holds (rt.h's borrow_fields), so
// overwriting one releases nothing; a read still hands the frame its own count.
static int borrow_field(val r, int64_t i) {
    return r->kind == K_RECORD && i < 64 && ((r->u.rec.type->borrow_fields >> i) & 1);
}
static val record_build(rt_type* t, int64_t n, val* fields, int owned) {
    if (!t) rt_panic("a runtime-named type was dropped by reachability");
    val v = alloc(K_RECORD);
    v->u.rec.type = t;
    v->u.rec.n = n;
    v->u.rec.fields = amalloc(v->al, (n ? n : 1) * sizeof(val));
    if (!v->u.rec.fields) {
        discard_header(v);
        if (owned) release_moved(fields, n, t->borrow_fields);
        rt_panic("out of memory");
    }
    for (int64_t i = 0; i < n; i++) {
        v->u.rec.fields[i] = fields[i];
    }
    return v;
}
val rt_record(rt_type* t, int64_t n, val* fields) {
    return record_build(t, n, fields, 1);
}
// For callers whose fields still belong to someone else: a refused allocation releases nothing.
static val rt_record_borrowed(rt_type* t, int64_t n, val* fields) {
    return record_build(t, n, fields, 0);
}
// Every payload-free variant's immortal, built before any Kigumi code
// runs: an interrupt handler must never find one half-initialized.
void rt_init_nullary(int ntypes, rt_type** types) {
    for (int i = 0; i < ntypes; i++) {
        for (int j = 0; j < types[i]->nvariants; j++) {
            rt_vdesc* d = types[i]->variants[j];
            d->nullary = (struct obj){.kind = K_VARIANT, .rc = -1, .u.var = {.v = d, .n = 0, .payload = NULL}};
        }
    }
}
// A payload-free variant has no per-value state, so every construction
// hands back the one immortal value living in its descriptor.
val rt_nullary(rt_vdesc* d) {
    return &d->nullary;
}
// A variant whose payload is itself fixed at compile time has one value
// per program; slot and payload are emitter-reserved storage.
void rt_const_variant(void* slot, val* payload, rt_vdesc* d, int64_t n) {
    val v = (val)slot;
    v->kind = K_VARIANT;
    v->rc = -1;
    v->u.var.v = d;
    v->u.var.n = n;
    v->u.var.payload = payload;
}
void rt_const_record(void* slot, val* fields, rt_type* t, int64_t n) {
    val v = (val)slot;
    v->kind = K_RECORD;
    v->rc = -1;
    v->u.rec.type = t;
    v->u.rec.n = n;
    v->u.rec.fields = fields;
}
// A literal has one value per program, so every site that builds it reads
// the same immortal out of emitter-reserved storage.
void rt_const_num(void* slot, int kind, int nk, int64_t i, double f) {
    val v = (val)slot;
    v->kind = kind;
    v->rc = -1;
    if (kind == K_FLOAT) {
        v->u.f = (nk & 255) == 32 ? (double)(float)f : f;
        v->nk = nk;
        return;
    }
    if (kind == K_CHAR) {
        v->u.c = (uint32_t)i;
        return;
    }
    v->u.i = i;
    v->nk = nk ? nk : (64 | 256);
}
void rt_const_text(void* slot, int kind, const char* data, int64_t len) {
    val v = (val)slot;
    v->kind = kind;
    v->rc = -1;
    v->u.s.data = data;
    v->u.s.len = len;
}
val rt_variant(rt_vdesc* d, int64_t n, val* payload) {
    if (!d) rt_panic("a runtime-named type was dropped by reachability");
    if (n == 0) return rt_nullary(d);
    val v = alloc(K_VARIANT);
    v->u.var.v = d;
    v->u.var.n = n;
    v->u.var.payload = amalloc(v->al, (n ? n : 1) * sizeof(val));
    if (!v->u.var.payload) {
        discard_header(v);
        rt_panic("out of memory");
    }
    for (int64_t i = 0; i < n; i++) {
        v->u.var.payload[i] = payload[i];
    }
    return v;
}
val rt_deref(val v) {
    CHK(v);
    while (v && v->kind == K_CELL) {
        v = v->u.cell.v;
        CHK(v);
    }
    return v;
}
val rt_field(val r, int64_t i) {
    r = rt_deref(r);
    if (r->kind == K_BOX) r = r->u.box.v;
    return rt_retain(r->u.rec.fields[i]);
}
// Takes field i out of r without a retain: r's slot goes NULL, which
// rt_release already treats as a no-op, so the field drops once through
// whichever of r or the returned value reaches a drop first.
val rt_field_move(val r, int64_t i) {
    r = rt_deref(r);
    if (r->kind == K_BOX) r = r->u.box.v;
    val v = r->u.rec.fields[i];
    r->u.rec.fields[i] = NULL;
    return v;
}
// Storing into a place releases what it held: the caller hands over an owned
// reference, so without this the overwritten value is never dropped.
val rt_setfield_swap(val r, int64_t i, val x) {
    r = rt_deref(r);
    val old = r->u.rec.fields[i];
    r->u.rec.fields[i] = x;
    return borrow_field(r, i) ? NULL : old;
}
void rt_setfield(val r, int64_t i, val x) {
    rt_release(rt_setfield_swap(r, i, x));
}
val rt_payload(val v, int64_t i) {
    v = rt_deref(v);
    return rt_retain(v->u.var.payload[i]);
}
val rt_isvariant(val v, rt_vdesc* d) {
    v = rt_deref(v);
    return rt_bool(v->kind == K_VARIANT && v->u.var.v == d);
}
// The tag an inline ADT keeps beside its payload area: a variant's
// declaration index, which is also the order llgen lays the cases out in.
int rt_variant_index(val v) {
    v = rt_deref(v);
    // A value that is no variant matches no case, as a boxed `match` finds.
    if (!v || v->kind != K_VARIANT) rt_panic("no match arm matched");
    return v->u.var.v->index;
}
val rt_box(val v, rt_type* dyn, void** vt) {
    if (v->kind == K_BOX) return v;
    val b = alloc(K_BOX);
    b->u.box.dyn = dyn;
    b->u.box.v = v;
    b->u.box.vt = vt;
    b->u.box.src = NULL;
    return b;
}
// An `&Interface` view in its caller's frame: immortal, and owned marks that it holds no count of v.
void rt_box_lent(void* slot, val v, rt_type* dyn, void** vt) {
    val b = (val)slot;
    b->kind = K_BOX;
    b->rc = -1;
    b->owned = 1;
    b->al = NULL;
    b->u.box.dyn = dyn;
    b->u.box.v = rt_deref(v);
    b->u.box.vt = vt;
    b->u.box.src = v && v->kind == K_CELL ? v : NULL;
}
val rt_unbox(val b) {
    return b->kind == K_BOX ? rt_retain(b->u.box.v) : b;
}
val rt_istype(val b, rt_type* t) {
    b = rt_deref(b);
    return rt_bool(b->kind == K_BOX && b->u.box.dyn == t);
}
// std/error.as[E]: e is E(x) doesn't type-check over a type parameter
// (E101), so this runtime downcast runs here instead. target
// and ctx are compile-time descriptors; wrapped is Context's "wrapped"
// field index, so a field reorder in error.kg still walks the right slot.
// sem requires E: Copy, so the match is duplicated with rt_copy, never
// aliased into the caller's borrow.
val rt_error_as(val e, rt_type* target, rt_type* ctx, int64_t wrapped) {
    val cur = rt_deref(e);
    for (;;) {
        if (cur->kind != K_BOX) return rt_none();
        if (cur->u.box.dyn == target) return rt_some(rt_copy(cur->u.box.v));
        if (!ctx || cur->u.box.dyn != ctx) return rt_none();
        cur = rt_deref(cur->u.box.v->u.rec.fields[wrapped]);
    }
}
static int boundary(const char* s, int64_t n, int64_t i) {
    return i == n || (s[i] & 0xC0) != 0x80;
}
val rt_index(val base, val idx, const char* site) {
    base = rt_deref(base);
    idx = rt_deref(idx);
    if (idx->kind == K_RECORD) {
        int64_t lo = idx->u.rec.fields[0]->u.i, hi = idx->u.rec.fields[1]->u.i;
        if (idx->u.rec.fields[2]->u.b) hi++;
        if (base->kind == K_STR || base->kind == K_BYTES) {
            if (lo < 0 || hi > base->u.s.len || lo > hi) rt_panic("slice out of range");
            if (base->kind == K_STR && (!boundary(base->u.s.data, base->u.s.len, lo) || !boundary(base->u.s.data, base->u.s.len, hi))) rt_panic("slice is not on character boundaries");
            val r = strdup_val(base->u.s.data + lo, hi - lo);
            r->kind = base->kind;
            return r;
        }
        if (lo < 0 || hi > base->u.arr.len || lo > hi) rt_panic("slice out of range");
        {
            val out = rt_array(hi - lo, base->u.arr.items + lo);
            for (int64_t j = 0; j < out->u.arr.len; j++) rt_retain(out->u.arr.items[j]);
            return out;
        }
    }
    int64_t i = idx->u.i;
    if (base->kind == K_STR || base->kind == K_BYTES) {
        if (i < 0 || i >= base->u.s.len) rt_panic_index(i, base->u.s.len, site);
        return rt_int((unsigned char)base->u.s.data[i], 8);
    }
    if (base->kind != K_ARRAY || i < 0 || i >= base->u.arr.len) rt_panic_index(i, base->kind == K_ARRAY ? base->u.arr.len : 0, site);
    return rt_retain(base->u.arr.items[i]);
}
val rt_setindex_swap(val base, val idx, val v, const char* site) {
    base = rt_deref(base);
    int64_t i = rt_deref(idx)->u.i;
    if (base->kind != K_ARRAY || i < 0 || i >= base->u.arr.len) rt_panic_index(i, base->kind == K_ARRAY ? base->u.arr.len : 0, site);
    val old = base->u.arr.items[i];
    base->u.arr.items[i] = v;
    return old;
}
void rt_setindex(val base, val idx, val v, const char* site) {
    rt_release(rt_setindex_swap(base, idx, v, site));
}
// Fields of a record built only from scalar boxes and such records can be taken over where they stand.
static int fields_fit(val dst, val src) {
    if (dst == src) return 1;
    if (dst->rc != 1 || dst->kind != K_RECORD || src->kind != K_RECORD) return 0;
    if (dst->u.rec.type != src->u.rec.type || dst->u.rec.type->kind == 1 || dst->u.rec.type->borrow_fields) return 0;
    for (int64_t i = 0; i < dst->u.rec.n; i++) {
        val d = dst->u.rec.fields[i], s = src->u.rec.fields[i];
        if (d == s) continue;
        if (!d || !s || d->kind != s->kind) return 0;
        if (d->kind == K_RECORD ? !fields_fit(d, s) : d->kind != K_INT && d->kind != K_FLOAT && d->kind != K_BOOL && d->kind != K_CHAR && d->kind != K_UNIT) return 0;
    }
    return 1;
}
static void take_fields(val dst, val src) {
    if (dst == src) return;
    for (int64_t i = 0; i < dst->u.rec.n; i++) {
        val d = dst->u.rec.fields[i], s = src->u.rec.fields[i];
        if (d == s) continue;
        if (d->kind == K_RECORD) {
            take_fields(d, s);
        } else {
            dst->u.rec.fields[i] = rt_retain(s);
            rt_release(d);
        }
    }
}
// base[idx] = copy of src; the slot keeps its own box when src's fields fit into it.
val rt_setindex_copy_swap(val base, val idx, val src, const char* site) {
    base = rt_deref(base);
    int64_t i = rt_deref(idx)->u.i;
    if (base->kind != K_ARRAY || i < 0 || i >= base->u.arr.len) rt_panic_index(i, base->kind == K_ARRAY ? base->u.arr.len : 0, site);
    val old = base->u.arr.items[i];
    if (old && fields_fit(old, src)) {
        take_fields(old, src);
        return NULL;
    }
    base->u.arr.items[i] = rt_copy(src);
    return old;
}
void rt_setindex_copy(val base, val idx, val src, const char* site) {
    rt_release(rt_setindex_copy_swap(base, idx, src, site));
}
// r.fields[i] = copy of src; the field keeps its own box when src's fields fit into it.
val rt_setfield_copy_swap(val r, int64_t i, val src) {
    r = rt_deref(r);
    val old = r->u.rec.fields[i];
    if (old && !borrow_field(r, i) && fields_fit(old, src)) {
        take_fields(old, src);
        return NULL;
    }
    r->u.rec.fields[i] = rt_copy(src);
    return borrow_field(r, i) ? NULL : old;
}
void rt_setfield_copy(val r, int64_t i, val src) {
    rt_release(rt_setfield_copy_swap(r, i, src));
}
int64_t rt_int_val(val v) {
    v = rt_deref(v);
    return v->kind == K_INT ? v->u.i : 0;
}
double rt_float_val(val v) {
    v = rt_deref(v);
    return v->kind == K_FLOAT ? v->u.f : (double)v->u.i;
}
uint32_t rt_char_val(val v) {
    v = rt_deref(v);
    return v->kind == K_CHAR ? v->u.c : 0;
}
val rt_from_cstr(const char* s) {
    return s ? rt_str(s, strlen(s)) : rt_str("", 0);
}
int rt_truth(val v) {
    v = rt_deref(v);
    return v->kind == K_BOOL && v->u.b;
}
// Captures may alias live locals (method values, borrowed captures), so
// the environment retains them.
val rt_closure(adapter_fn fn, int n, val* caps) {
    val v = alloc(K_CLOSURE);
    v->u.clo.fn = fn;
    v->u.clo.env = rt_array(n, caps);
    for (int i = 0; i < n; i++) rt_retain(caps[i]);
    return v;
}

val rt_closure_env(val f) {
    f = rt_deref(f);
    return f->u.clo.env;
}

// slot is the storage the emitter reserved for a capture-free function
// value; kg_init_immortals fills it before any Kigumi code runs.
void rt_fnitem_init(void* slot, adapter_fn fn, int async) {
    static struct obj empty_env = {.kind = K_ARRAY, .rc = -1};
    val v = (val)slot;
    v->kind = K_CLOSURE;
    v->rc = -1;
    v->u.clo.fn = fn;
    v->u.clo.env = &empty_env;
    v->u.clo.async = async;
}

// slot gets a capture-free std primitive used as a witness, so the
// dictionary a generic call passes is a constant, not a per-call closure.
#ifdef KIGUMI_STD_DYNAMIC
void rt_stditem_init(void* slot, const char* key, uint32_t lent) {
    rt_fnitem_init(slot, NULL, 0);
    val v = (val)slot;
    v->u.clo.key = key;
    v->u.clo.lent = lent;
}
#endif

// lent marks the arguments a user function value's parameters only borrow, which rt_release_lent_arg reads.
void rt_fnitem_lent(void* slot, int lent) {
    ((val)slot)->u.clo.lent = (uint32_t)lent;
}

val rt_closure_lent(val v, int lent) {
    rt_deref(v)->u.clo.lent = (uint32_t)lent;
    return v;
}


// rt_closure_async is rt_closure for an async fn/method used as a value
// (A25-b): rt_callv on the result builds a Future instead of running fn.
val rt_closure_async(adapter_fn fn, int n, val* caps) {
    val v = rt_closure(fn, n, caps);
    v->u.clo.async = 1;
    return v;
}

// async 2 marks a bound async fn: its captured witnesses are the trailing parameters, after the call's arguments.
val rt_closure_bind_async(adapter_fn fn, int n, val* caps) {
    val v = rt_closure(fn, n, caps);
    v->u.clo.async = 2;
    return v;
}

val rt_copy(val v) {
    if (!v) return v;
    // An immortal is never written through, so a copy of it can be itself.
    if (v->rc < 0) return v;
    switch (v->kind) {
    case K_RECORD: {
        if (v->u.rec.type->kind == 1) return rt_retain(v);
        val c = rt_record_borrowed(v->u.rec.type, v->u.rec.n, v->u.rec.fields);
        uint64_t bf = v->u.rec.type->borrow_fields;
        for (int64_t i = 0; i < c->u.rec.n; i++) {
            if ((bf >> i) & 1) continue;
            c->u.rec.fields[i] = rt_copy(c->u.rec.fields[i]);
        }
        return c;
    }
    case K_VARIANT: {
        val c = rt_variant(v->u.var.v, v->u.var.n, v->u.var.payload);
        for (int64_t i = 0; i < c->u.var.n; i++) c->u.var.payload[i] = rt_copy(c->u.var.payload[i]);
        return c;
    }
    case K_ARRAY: {
        val c = rt_array(v->u.arr.len, v->u.arr.items);
        for (int64_t i = 0; i < c->u.arr.len; i++) c->u.arr.items[i] = rt_copy(c->u.arr.items[i]);
        return c;
    }
    case K_BYTES: {
        // Each copy gets its own val, even when unowned, so secureErase on
        // one cannot reach through a shared struct into another's fields.
        val c = alloc(K_BYTES);
        c->u.s.len = v->u.s.len;
        if (!v->owned) {
            c->u.s.data = v->u.s.data;
            return c;
        }
        char* d = malloc((uint64_t)v->u.s.len + 1);
        if (!d) rt_panic("out of memory");
        memcpy(d, v->u.s.data, v->u.s.len);
        d[v->u.s.len] = 0;
        c->u.s.data = d;
        c->owned = 1;
        return c;
    }
    case K_BOX: {
        val c = alloc(K_BOX);
        c->u.box.dyn = v->u.box.dyn;
        c->u.box.vt = v->u.box.vt;
        c->u.box.src = NULL;
        c->u.box.v = rt_copy(v->u.box.v);
        return c;
    }
    }
    return rt_retain(v);
}

#define KG_WRITABLE_RECORD 4
// Bytes (secureErase) and a FixedArray are written in place, as is a record with a `mut` field; a Box is copied whole.
static int writable_inside(val v) {
    if (!v || v->rc < 0) return 0;
    switch (v->kind) {
    case K_ARRAY:
    case K_BYTES:
    case K_BOX: return 1;
    case K_RECORD: {
        const rt_type* t = v->u.rec.type;
        if (t->kind == KG_WRITABLE_RECORD) return 1;
        if (t->kind == 1) return 0;
        for (int64_t i = 0; i < v->u.rec.n; i++)
            if (!((t->borrow_fields >> i) & 1) && writable_inside(v->u.rec.fields[i])) return 1;
        return 0;
    }
    case K_VARIANT:
        for (int64_t i = 0; i < v->u.var.n; i++)
            if (writable_inside(v->u.var.payload[i])) return 1;
        return 0;
    }
    return 0;
}
val rt_copy_mut(val v) {
    if (writable_inside(v)) return rt_copy(v);
    return rt_retain(v);
}

#ifdef KIGUMI_INT128
static int64_t truncate_to(int64_t v, int nk);
#include "rt_int128.inc"
void rt_const_int128(void* slot, int nk, int64_t lo, int64_t hi) {
    val v = (val)slot;
    v->kind = K_INT;
    v->rc = -1;
    v->u.w.lo = lo;
    v->u.w.hi = hi;
    v->nk = nk;
}
// Entry points for llgen's register code, which passes a 128-bit value as two words.
val rt_box_int128(int64_t lo, int64_t hi, int nk) {
    return w_box(w_new((uint64_t)lo, (uint64_t)hi), nk);
}
int64_t rt_int128_lo(val v) {
    return rt_deref(v)->u.w.lo;
}
int64_t rt_int128_hi(val v) {
    return rt_deref(v)->u.w.hi;
}
#endif
int rt_equal(val a, val b) {
    a = rt_deref(a);
    b = rt_deref(b);
    if (a->kind != b->kind) return 0;
    switch (a->kind) {
    case K_INT:
#ifdef KIGUMI_INT128
        if (W_IS(a->nk)) return w_equal(a, b);
#endif
        return a->u.i == b->u.i;
    case K_FLOAT:
        return a->u.f == b->u.f;
    case K_BOOL:
        return a->u.b == b->u.b;
    case K_CHAR:
        return a->u.c == b->u.c;
    case K_UNIT:
        return 1;
    case K_PTR:
        return a->u.p == b->u.p;
    case K_STR:
    case K_BYTES:
        return a->u.s.len == b->u.s.len && memcmp(a->u.s.data, b->u.s.data, a->u.s.len) == 0;
    }
    // Records, ADTs and arrays compare through their `equals`.
    return a == b;
}

