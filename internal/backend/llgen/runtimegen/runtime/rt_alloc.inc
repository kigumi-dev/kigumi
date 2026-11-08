// Allocators: objects and their slot buffers come from the current
// allocator and remember it, so escaping values free through their origin.
// String bytes stay on libc because every producer of owned text uses malloc.
typedef struct rt_allocator {
    void* (*alloc)(struct rt_allocator*, size_t);
    void (*free)(struct rt_allocator*, void*);
    KG_I64 live, bytes;
    int closed;
} rt_allocator;
static void* gen_alloc(rt_allocator* a, size_t n) {
    void* p = calloc(1, n);
    if (p) KG_INC(a->live);
    return p;
}
static void gen_free(rt_allocator* a, void* p) {
    KG_DEC(a->live);
    free(p);
}
#if defined(KIGUMI_FAULT_INJECT) && !defined(KIGUMI_FREESTANDING)
// Test-only; this macro is never set in a normal build.
static int arena_fault_injected(void) {
    return getenv("KIGUMI_FAIL_ARENA_CREATE") != NULL;
}
// fault_tick counts allocation attempts and reports whether this is the one
// KIGUMI_FAIL_ALLOC_AT names (1-based); the env var is read once and cached.
static int64_t g_fault_at = -2, g_fault_n = 0;
// With KIGUMI_FAIL_ALLOC_ARM set, nothing counts until the first eprint returns.
static int g_fault_waiting;
static void fault_init(void) {
    if (g_fault_at != -2) return;
    const char* e = getenv("KIGUMI_FAIL_ALLOC_AT");
    g_fault_at = e ? strtoll(e, NULL, 10) : -1;
    g_fault_waiting = getenv("KIGUMI_FAIL_ALLOC_ARM") != NULL;
}
static int fault_tick(void) {
    fault_init();
    if (g_fault_at <= 0 || g_fault_waiting) return 0;
    return ++g_fault_n == g_fault_at;
}
static void fault_arm(void) {
    fault_init();
    if (!g_fault_waiting) return;
    g_fault_waiting = 0;
    g_fault_n = 0;
}
#else
static int arena_fault_injected(void) {
    return 0;
}
static int fault_tick(void) {
    return 0;
}
static void fault_arm(void) {}
#endif
static rt_allocator g_general = {.alloc = gen_alloc, .free = gen_free};
static KG_TLS rt_allocator* g_alloc_stack[64] = {&g_general};
static KG_TLS int g_alloc_sp = 0;
static rt_allocator* cur_alloc(void) {
    return g_alloc_stack[g_alloc_sp];
}
static void* amalloc(rt_allocator* a, size_t n) {
    if (fault_tick()) return NULL;
    KG_ADD(a->bytes, (int64_t)n);
    return a->alloc(a, n);
}
static void afree(rt_allocator* a, void* p) {
    if (p) a->free(a, p);
}
static void* arealloc(rt_allocator* a, void* p, size_t oldn, size_t newn) {
    void* q = amalloc(a, newn);
    if (!q) return NULL;
    if (p) {
        memcpy(q, p, oldn);
        afree(a, p);
    }
    return q;
}
typedef struct chunk {
    struct chunk* next;
    size_t cap, used;
    char data[];
} chunk;
typedef struct arena {
    rt_allocator a;
    chunk* head;
} arena;
static void* arena_alloc(rt_allocator* a, size_t n) {
    arena* ar = (arena*)a;
    n = (n + 15) & ~(size_t)15;
    chunk* c = ar->head;
    if (!c || c->used + n > c->cap) {
        size_t cap = n > 65536 ? n : 65536;
        c = malloc(sizeof(chunk) + cap);
        if (!c) return NULL;
        c->next = ar->head;
        c->cap = cap;
        c->used = 0;
        ar->head = c;
    }
    void* p = c->data + c->used;
    c->used += n;
    memset(p, 0, n);
    KG_INC(a->live);
    return p;
}
static void arena_drop_chunks(arena* ar) {
    for (chunk* c = ar->head; c;) {
        chunk* nx = c->next;
        free(c);
        c = nx;
    }
    free(ar);
}
// Live objects keep the chunks; a closed arena is freed by its last object.
static void arena_free(rt_allocator* a, void* p) {
    (void)p;
    if (KG_DEC(a->live) == 1 && a->closed) arena_drop_chunks((arena*)a);
}
static void arena_close(arena* ar) {
    ar->a.closed = 1;
    if (ar->a.live == 0) arena_drop_chunks(ar);
}
#ifdef KIGUMI_RC_CHECK
static long g_live_objs;
static long g_live_kinds[K_PTR + 1];
// Bytes start as strings and are retagged, so they share a bucket.
#define LIVE_OBJ(kind, d)                                                                       \
    do {                                                                                        \
        __atomic_add_fetch(&g_live_objs, (d), __ATOMIC_RELAXED);                                \
        __atomic_add_fetch(&g_live_kinds[(kind) == K_BYTES ? K_STR : (kind)], (d), __ATOMIC_RELAXED); \
    } while (0)
#else
#define LIVE_OBJ(kind, d) ((void)0)
#endif
static val alloc_in(rt_allocator* a, int kind) {
    val v = amalloc(a, sizeof(struct obj));
    if (!v) rt_panic("out of memory");
    LIVE_OBJ(kind, 1);
    v->kind = kind;
    v->rc = 1;
    v->owned = 0;
    v->al = a;
    return v;
}
static void discard_header(val v) {
    LIVE_OBJ(v->kind, -1);
    afree(v->al, v);
}
static val alloc(int kind) {
    return alloc_in(cur_alloc(), kind);
}
// An allocator's own bookkeeping objects come from that allocator, so an
// `allocator` block needs no ambient allocator (`allocator = none`).
static val opaque_in(rt_allocator* a, const char* kind, void* data) {
    val v = alloc_in(a, K_OPAQUE);
    v->u.op.kind = kind;
    v->u.op.data = data;
    return v;
}
// Reference counts: every heap object starts at 1; static objects use -1
// and are never freed. Moves keep the count, copies/hand-outs retain, MIR
// drops release; over-retaining only leaks.
// -DKIGUMI_RC_CHECK quarantines freed objects instead of freeing them and
// panics on later use, standing in for a sanitizer.
void rt_panic(const char* msg);
void rt_panic_index(int64_t i, int64_t len, const char* site);
#ifdef KIGUMI_RC_CHECK
#define CHK(v)                                           \
    do {                                                 \
        if ((v) && (v)->kind == 0) {                     \
            rt_sys_write(2, "use after free in ", 18);   \
            rt_sys_write(2, __func__, strlen(__func__)); \
            rt_sys_write(2, "\n", 1);                    \
            rt_sys_exit(2);                              \
        }                                                \
    } while (0)
#else
#define CHK(v) \
    do {       \
    } while (0)
#endif
