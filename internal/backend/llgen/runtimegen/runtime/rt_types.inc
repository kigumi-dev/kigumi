_Static_assert(offsetof(rt_tnode, rc) == offsetof(struct obj, rc), "a type value must read as an immortal");
_Static_assert(offsetof(rt_tnode, kind) == offsetof(struct obj, kind), "a type value must read as an immortal");
// A static node the compiler lists: its constructor, the descriptors it has, and its argument nodes as indices
// into the list, or for a const (nargs -1) its low and high words as four 32-bit pieces.
typedef struct rt_tndef {
    uint64_t ctor;
    rt_type* base;
    rt_type* desc;
    int32_t arg;
    int32_t nargs;
} rt_tndef;
// The table probes linearly over node pointers: a lookup reads one run of slots, and the table stays small, since
// what a deep polymorphic recursion pays for is mostly the pages its nodes and table touch.
static rt_tnode** g_tn;
static uint64_t g_ntn, g_captn;
#ifdef KIGUMI_THREADS
static pthread_mutex_t g_tn_lock = PTHREAD_MUTEX_INITIALIZER;
#define TN_LOCK() pthread_mutex_lock(&g_tn_lock)
#define TN_UNLOCK() pthread_mutex_unlock(&g_tn_lock)
#else
#define TN_LOCK() ((void)0)
#define TN_UNLOCK() ((void)0)
#endif
// The table lives as long as the program, so it is not counted among the allocations a leak check reports, and its
// nodes come from chunks: a node is never freed.
static void* tn_alloc(size_t n) {
    void* p = calloc(1, n);
    if (!p) rt_panic("out of memory");
    return p;
}
static char *g_tn_chunk, *g_tn_end;
static void* tn_node_alloc(size_t n) {
    n = (n + 7) & ~(size_t)7;
    if ((size_t)(g_tn_end - g_tn_chunk) < n) {
        size_t size = n > 65536 ? n : 65536;
        g_tn_chunk = tn_alloc(size);
        g_tn_end = g_tn_chunk + size;
    }
    void* p = g_tn_chunk;
    g_tn_chunk += n;
    return p;
}
#define TN_LO(x) (*(uint64_t*)&(x)->args[0])
#define TN_HI(x) (*(uint64_t*)&(x)->args[1])
#define TN_VALUE(x) (*(val*)&(x)->args[2])
static uint32_t tn_hash(uint64_t ctor, uint64_t lo, uint64_t hi, int32_t n, rt_tnode** args) {
    uint64_t h = ctor * 0x9E3779B97F4A7C15ULL ^ lo ^ (hi * 0xC2B2AE3D27D4EB4FULL) ^ (uint64_t)(uint32_t)n;
    for (int32_t i = 0; i < n; i++) h = (h ^ (uint64_t)(uintptr_t)args[i]) * 0x100000001B3ULL;
    return (uint32_t)(h ^ h >> 32);
}
static void tn_grow(void) {
    uint64_t cap = g_captn ? g_captn * 2 : 1024;
    rt_tnode** grown = tn_alloc(cap * sizeof(rt_tnode*));
    for (uint64_t i = 0; i < g_captn; i++) {
        if (!g_tn[i]) continue;
        uint64_t k = g_tn[i]->hash & (cap - 1);
        while (grown[k]) k = (k + 1) & (cap - 1);
        grown[k] = g_tn[i];
    }
    free(g_tn);
    g_tn = grown;
    g_captn = cap;
}
static int tn_same(rt_tnode* x, uint64_t ctor, uint64_t lo, uint64_t hi, int32_t n, rt_tnode** args) {
    if (x->ctor != ctor || x->nargs != n) return 0;
    if (n < 0) return TN_LO(x) == lo && TN_HI(x) == hi;
    for (int32_t i = 0; i < n; i++) {
        if (x->args[i] != args[i]) return 0;
    }
    return 1;
}
// tn_intern finds or makes the node; the caller holds the lock.
static rt_tnode* tn_intern(uint64_t ctor, uint64_t lo, uint64_t hi, int32_t n, rt_tnode** args) {
    uint32_t h = tn_hash(ctor, lo, hi, n, args);
    if ((g_ntn + 1) * 4 > g_captn * 3) tn_grow();
    uint64_t k = h & (g_captn - 1);
    for (; g_tn[k]; k = (k + 1) & (g_captn - 1)) {
        if (g_tn[k]->hash == h && tn_same(g_tn[k], ctor, lo, hi, n, args)) return g_tn[k];
    }
    rt_tnode* x = tn_node_alloc(sizeof(rt_tnode) + (size_t)(n < 0 ? 3 : n) * sizeof(rt_tnode*));
    x->kind = K_INT;
    x->nk = 64 | 256;
    x->rc = -1;
    x->ctor = ctor;
    x->nargs = n;
    x->hash = h;
    if (n < 0) {
        TN_LO(x) = lo;
        TN_HI(x) = hi;
    }
    for (int32_t i = 0; i < n; i++) x->args[i] = args[i];
    g_tn[k] = x;
    g_ntn++;
    return x;
}
rt_tnode* rt_tn_make(int64_t ctor, rt_type* base, int64_t n, rt_tnode** args) {
    TN_LOCK();
    rt_tnode* x = tn_intern((uint64_t)ctor, 0, 0, (int32_t)n, args);
    if (base && !x->desc) TN_SET_DESC(x, base);
    TN_UNLOCK();
    return x;
}
// A template is a runtime type the compiler composes from holes, laid out as an rt_tndef whose arguments are each a
// static node (k >= 0), hole h (-1 - h), or template j (TT_NEST - j).
#define TT_NEST (-(1 << 24))
static rt_tnode** g_tn_static;
static const rt_tndef* g_tt;
static const int32_t* g_tt_args;
// tt_memo is the last node a one-hole template made, by its hole: polymorphic recursion composes the same type
// several times per level from one environment. With threads a record is immutable, so a reader needs no lock.
typedef struct {
    rt_tnode* hole;
    rt_tnode* node;
} tt_memo;
#ifdef KIGUMI_THREADS
static tt_memo** g_tt_memo;
#else
static tt_memo* g_tt_memo;
#endif
rt_tnode* rt_tn_tmpl(int i, rt_tnode* const* holes) {
    const rt_tndef* d = &g_tt[i];
    rt_tnode* stack[STACK_ARGS];
    rt_tnode** as = d->nargs <= STACK_ARGS ? stack : tn_alloc((size_t)d->nargs * sizeof(rt_tnode*));
    for (int32_t k = 0; k < d->nargs; k++) {
        int32_t code = g_tt_args[d->arg + k];
        as[k] = code >= 0 ? g_tn_static[code] : code > TT_NEST ? holes[-1 - code] : rt_tn_tmpl(TT_NEST - code, holes);
    }
    rt_tnode* x = rt_tn_make((int64_t)d->ctor, d->base, d->nargs, as);
    if (as != stack) free(as);
    return x;
}
rt_tnode* rt_tn_tmpl1(int i, rt_tnode* hole) {
#ifdef KIGUMI_THREADS
    tt_memo* m = __atomic_load_n(&g_tt_memo[i], __ATOMIC_ACQUIRE);
    if (m && m->hole == hole) return m->node;
    rt_tnode* x = rt_tn_tmpl(i, &hole);
    TN_LOCK();
    m = tn_node_alloc(sizeof(tt_memo));
    TN_UNLOCK();
    m->hole = hole;
    m->node = x;
    __atomic_store_n(&g_tt_memo[i], m, __ATOMIC_RELEASE);
    return x;
#else
    tt_memo* m = &g_tt_memo[i];
    if (m->hole != hole) {
        m->node = rt_tn_tmpl(i, &hole);
        m->hole = hole;
    }
    return m->node;
#endif
}
rt_tnode* rt_tn_const_words(int64_t ctor, int64_t lo, int64_t hi) {
    TN_LOCK();
    rt_tnode* x = tn_intern((uint64_t)ctor, (uint64_t)lo, (uint64_t)hi, -1, NULL);
    TN_UNLOCK();
    return x;
}
// rt_tn_const is the node of the const argument value v of number kind nk, read at its full width.
rt_tnode* rt_tn_const(val v, int64_t ctor, int nk) {
    v = rt_deref(v);
    if (v->kind == K_BOOL) return rt_tn_const_words(ctor, v->u.b != 0, 0);
    int64_t hi = (nk & 256) && v->u.i < 0 ? -1 : 0;
#ifdef KIGUMI_INT128
    if ((nk & 255) == 128) hi = v->u.w.hi;
#endif
    return rt_tn_const_words(ctor, v->u.i, hi);
}
// rt_tn_value is the value of const node x as a value of number kind nk, a Bool for nk 0: one immortal per node.
val rt_tn_value(rt_tnode* x, int nk) {
    if (!x || x->nargs >= 0) rt_panic("internal error: a const parameter has no value");
    TN_LOCK();
    val v = TN_VALUE(x);
    if (!v) {
        v = tn_alloc(sizeof(struct obj));
        v->rc = -1;
        if (nk == 0) {
            v->kind = K_BOOL;
            v->u.b = TN_LO(x) != 0;
        } else {
            v->kind = K_INT;
            v->nk = nk;
            v->u.i = (int64_t)TN_LO(x);
#ifdef KIGUMI_INT128
            if ((nk & 255) == 128) v->u.w.hi = (int64_t)TN_HI(x);
#endif
        }
        TN_VALUE(x) = v;
    }
    TN_UNLOCK();
    return v;
}
rt_tnode* rt_tn_arg(rt_tnode* x, int64_t i) {
    if (!x || i >= x->nargs) rt_panic("internal error: a runtime environment lacks an argument");
    return x->args[i];
}
// rt_tn_desc is the descriptor a record of type x carries; the first call copies the generic type's for x, so the
// record's drop reads the instantiation back through tnode.
rt_type* rt_tn_desc(rt_tnode* x) {
    TN_LOCK();
    rt_type* d = TN_DESC(x);
    if (!d) rt_panic("internal error: a runtime type has no descriptor");
    if (d->tnode != x) {
        rt_type* base = d;
        d = tn_alloc(sizeof(rt_type));
        *d = *base;
        d->base = base;
        d->tnode = x;
        TN_SET_DESC(x, d);
    }
    TN_UNLOCK();
    return d;
}
// tn_base is the generic type an instantiation x's values are made from, or x's own descriptor.
static rt_type* tn_base(rt_tnode* x) {
    rt_type* d = TN_DESC(x);
    return d && d->tnode == x && d->base ? d->base : d;
}
void rt_tn_init(int64_t n, const rt_tndef* defs, const int32_t* args, rt_tnode** out, int64_t ntt, const rt_tndef* tt,
                const int32_t* tt_args) {
    g_tn_static = out;
    g_tt = tt;
    g_tt_args = tt_args;
    g_tt_memo = tn_alloc((size_t)(ntt > 0 ? ntt : 1) * sizeof(*g_tt_memo));
    rt_tnode* stack[STACK_ARGS];
    for (int64_t k = 0; k < n; k++) {
        const rt_tndef* d = &defs[k];
        rt_tnode* x;
        if (d->nargs < 0) {
            const uint32_t* w = (const uint32_t*)&args[d->arg];
            x = rt_tn_const_words((int64_t)d->ctor, (int64_t)((uint64_t)w[0] | (uint64_t)w[1] << 32), (int64_t)((uint64_t)w[2] | (uint64_t)w[3] << 32));
        } else {
            rt_tnode** as = d->nargs <= STACK_ARGS ? stack : tn_alloc((size_t)d->nargs * sizeof(rt_tnode*));
            for (int32_t i = 0; i < d->nargs; i++) as[i] = out[args[d->arg + i]];
            x = rt_tn_make((int64_t)d->ctor, d->base, d->nargs, as);
            if (as != stack) free(as);
        }
        if (d->desc) {
            d->desc->tnode = x;
            TN_SET_DESC(x, d->desc);
        }
        out[k] = x;
    }
}
// rt_tn_boxarg is argument i of the runtime type of box b, which a test of base's instantiations opened.
rt_tnode* rt_tn_boxarg(val b, rt_type* base, int64_t i) {
    b = rt_deref(b);
    rt_tnode* x = b->kind == K_BOX ? b->u.box.dyn : NULL;
    if (!x || tn_base(x) != base) rt_panic("internal error: a type test opened a box of another type");
    return rt_tn_arg(x, i);
}
// rt_tn_env is the environment a method reached through the runtime type x of its receiver runs with: x itself,
// or the method applied to the arguments of x that map picks.
rt_tnode* rt_tn_env(rt_tnode* x, int64_t ctor, int64_t n, const int32_t* map) {
    if (!x) rt_panic("internal error: a value without its runtime type reached a generic body");
    if (n < 0) return x;
    rt_tnode* stack[STACK_ARGS];
    rt_tnode** as = n <= STACK_ARGS ? stack : tn_alloc((size_t)n * sizeof(rt_tnode*));
    for (int64_t i = 0; i < n; i++) as[i] = rt_tn_arg(x, map[i]);
    rt_tnode* y = rt_tn_make(ctor, NULL, n, as);
    if (as != stack) free(as);
    return y;
}
// rt_istype_any matches any instantiation of the generic type whose entity descriptor is base.
val rt_istype_any(val b, rt_type* base) {
    b = rt_deref(b);
    rt_tnode* x = b->kind == K_BOX ? b->u.box.dyn : NULL;
    return rt_bool(x && tn_base(x) == base);
}
val rt_istype_tn(val b, rt_tnode* x) {
    b = rt_deref(b);
    return rt_bool(b->kind == K_BOX && b->u.box.dyn == x);
}
val rt_error_as_tn(val e, rt_tnode* x, rt_type* ctx, int64_t wrapped) {
    val cur = rt_deref(e);
    for (;;) {
        if (cur->kind != K_BOX) return rt_none();
        if (cur->u.box.dyn == x) return rt_some(rt_copy(cur->u.box.v));
        if (!ctx || TN_DESC(cur->u.box.dyn) != ctx) return rt_none();
        cur = rt_deref(cur->u.box.v->u.rec.fields[wrapped]);
    }
}
