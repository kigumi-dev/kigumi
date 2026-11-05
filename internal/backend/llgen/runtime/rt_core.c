// kigumi runtime core: boxed values, reference counting, allocators, the
// std primitives that need no operating system. System access goes through
// rt_sys_*.c.
#define _GNU_SOURCE
#ifdef KIGUMI_FREESTANDING
#include "rt_freestanding.h"
#else
#include <math.h>
#include <stdlib.h>
#include <string.h>
#endif

#include "rt.h"
#include "rt_internal.h"
#include "rt_std_ops.h"
#ifdef KIGUMI_THREADS
#include <pthread.h>
#define KG_TLS _Thread_local
#define KG_I64 _Atomic int64_t
#define KG_INC(x) atomic_fetch_add_explicit(&(x), 1, memory_order_relaxed)
#define KG_DEC(x) atomic_fetch_sub_explicit(&(x), 1, memory_order_acq_rel)
#define KG_ADD(x, n) atomic_fetch_add_explicit(&(x), (n), memory_order_relaxed)
#define KG_AINT _Atomic int
#define KG_ALOAD(x) atomic_load_explicit(&(x), memory_order_acquire)
#define KG_ASTORE(x, v) atomic_store_explicit(&(x), (v), memory_order_release)
static pthread_mutex_t g_sched = PTHREAD_MUTEX_INITIALIZER;
#define SCHED_LOCK() (kg_stack_check_locked(), pthread_mutex_lock(&g_sched))
#define SCHED_UNLOCK() pthread_mutex_unlock(&g_sched)
#else
#define KG_TLS
#define KG_I64 int64_t
#define KG_INC(x) ((x)++)
#define KG_DEC(x) ((x)--)
#define KG_ADD(x, n) ((x) += (n))
#define KG_AINT int
#define KG_ALOAD(x) (x)
#define KG_ASTORE(x, v) ((x) = (v))
#define SCHED_LOCK() ((void)0)
#define SCHED_UNLOCK() ((void)0)
#endif
#ifndef KIGUMI_FREESTANDING
// Set by rt_stack_guard_hosted.c: the low end of this thread's stack, or 0 while no guard is installed.
KG_TLS uintptr_t rt_stack_floor;
KG_TLS uintptr_t rt_stack_saved;
KG_TLS int rt_stack_forced;
KG_TLS int rt_in_libc;
#define KG_STACK_RESERVE (64 * 1024)
// A fault inside malloc would leave its lock held, so the overflow is raised here, before the call, with room left to unwind.
static inline void kg_stack_need(size_t room) {
    if ((uintptr_t)__builtin_frame_address(0) - rt_stack_floor < room && rt_stack_floor) {
        rt_stack_forced = 1;
        rt_sys_stack_force();
    }
}
#define kg_stack_check() kg_stack_need(KG_STACK_RESERVE)
#define kg_stack_check_locked() kg_stack_need(2 * KG_STACK_RESERVE)
static inline void* kg_malloc(size_t n) {
    kg_stack_check();
    rt_in_libc++;
    void* p = malloc(n);
    rt_in_libc--;
    return p;
}
static inline void* kg_calloc(size_t n, size_t m) {
    kg_stack_check();
    rt_in_libc++;
    void* p = calloc(n, m);
    rt_in_libc--;
    return p;
}
static inline void* kg_realloc(void* q, size_t n) {
    kg_stack_check();
    rt_in_libc++;
    void* p = realloc(q, n);
    rt_in_libc--;
    return p;
}
#define malloc(n) kg_malloc(n)
#define calloc(n, m) kg_calloc(n, m)
#define realloc(q, n) kg_realloc(q, n)
static inline void kg_stack_restore(void) {
    if (rt_stack_saved) {
        rt_stack_floor = rt_stack_saved;
        rt_stack_saved = 0;
    }
}
#else
#define kg_stack_check() ((void)0)
#define kg_stack_check_locked() ((void)0)
#define kg_stack_restore() ((void)0)
#endif
// n + 1 bytes, or NULL above half of this target's size_t: musl's allocators crash on requests just below SIZE_MAX.
static void* kg_malloc_n(int64_t n) {
    return (uint64_t)n + 1 > (uint64_t)SIZE_MAX / 2 ? NULL : malloc((size_t)((uint64_t)n + 1));
}
static void* kg_calloc_n(int64_t n) {
    return (uint64_t)n + 1 > (uint64_t)SIZE_MAX / 2 ? NULL : calloc((size_t)((uint64_t)n + 1), 1);
}
// A contained task panic needs a non-local jump: libc's where there is one, the compiler's own on a bare build.
#if defined(KIGUMI_TASK_CATCH) && defined(KIGUMI_UNWIND) && (!defined(KIGUMI_FREESTANDING) || defined(KIGUMI_TASK_CATCH_BUILTIN))
#define KG_CATCH 1
#ifdef KIGUMI_TASK_CATCH_BUILTIN
typedef void* kg_jmp_buf[5];
#define KG_SETJMP(b) __builtin_setjmp(b)
#define KG_LONGJMP(b) __builtin_longjmp(b, 1)
#else
#include <setjmp.h>
typedef jmp_buf kg_jmp_buf;
#ifdef _WIN32
#define KG_SETJMP(b) setjmp(b)
#define KG_LONGJMP(b) longjmp(b, 1)
#else
#define KG_SETJMP(b) _setjmp(b)
#define KG_LONGJMP(b) _longjmp(b, 1)
#endif
#endif
static KG_TLS kg_jmp_buf* g_task_catch;
static KG_TLS char* g_fault_msg;
static KG_TLS uint64_t g_fault_count;
static KG_TLS int g_drop_depth, g_faulting, g_outer_drop;
static void task_fault(const char* msg);
// Records run before the longjmp; later calls would overwrite the dead frames.
typedef struct kg_unwind {
    struct kg_unwind* prev;
    void (*run)(struct kg_unwind*);
    void* p;
    val v;
    int64_t i;
    uint64_t* m;
} kg_unwind;
static KG_TLS kg_unwind* g_unwind;
static KG_TLS kg_unwind* g_unwind_base;
#define UNWIND_PUSH(u, fn, p_, v_, i_) \
    kg_unwind u = {g_unwind, fn, p_, v_, i_}; \
    g_unwind = &u
#define UNWIND_POP(u) (g_unwind = (u).prev)
#else
#define UNWIND_PUSH(u, fn, p_, v_, i_) ((void)0)
#define UNWIND_POP(u) ((void)0)
#endif
#ifdef KIGUMI_COFF
// PE/COFF objects with floating-point code need this marker; the CRT
// normally defines it, but -nostdlib leaves none.
int _fltused = 1;
#endif
static int g_argc;
static char** g_argv;
static int g_ntypes;
static rt_type** g_types;
static rt_vdesc *V_Some, *V_None, *V_Ok, *V_Err, *V_Less, *V_Equal, *V_Greater;
static rt_type T_Error = {.name = "Error"};
// The runtime's own errors are boxed as Error with `message` in slot 0.
// The retain offsets rt_invoke's release of the argument array, as rt_at does.
static val error_message_adapter(val env, val args) {
    (void)env;
    val e = rt_retain(args->u.arr.items[0]);
    return rt_str((const char*)e->u.op.data, strlen((const char*)e->u.op.data));
}
static void* T_Error_vt[1] = {(void*)error_message_adapter};

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
val rt_retain(val v) {
    CHK(v);
#ifdef KIGUMI_THREADS
    if (v && atomic_load_explicit(&v->rc, memory_order_relaxed) > 0) atomic_fetch_add_explicit(&v->rc, 1, memory_order_relaxed);
#else
    if (v && v->rc > 0) v->rc++;
#endif
    return v;
}
static void free_obj(val v);
val rt_future_new(val callee, int n, val* args);
static int release_dec(val v) {
    CHK(v);
    if (!v) return 0;
#ifdef KIGUMI_THREADS
    if (atomic_load_explicit(&v->rc, memory_order_relaxed) <= 0) return 0;
#ifdef __SANITIZE_THREAD__
    return atomic_fetch_sub_explicit(&v->rc, 1, memory_order_acq_rel) == 1;
#else
    if (atomic_fetch_sub_explicit(&v->rc, 1, memory_order_release) != 1) return 0;
    atomic_thread_fence(memory_order_acquire);
    return 1;
#endif
#else
    if (v->rc <= 0) return 0;
    return --v->rc == 0;
#endif
}
void rt_release(val v) {
    if (release_dec(v)) free_obj(v);
}
#ifdef KIGUMI_LEAK_CHECK
// Test-only: how many allocations are still live at a fatal exit.
static void report_live(void) {
    char buf[32];
    int n = sizeof buf;
    buf[--n] = '\n';
    long long v = (long long)g_general.live;
    do {
        buf[--n] = (char)('0' + v % 10);
        v /= 10;
    } while (v);
    rt_sys_write(2, "live: ", 6);
    rt_sys_write(2, buf + n, (size_t)(sizeof buf - n));
}
#endif
#ifdef KIGUMI_THREADS
static pthread_mutex_t g_out = PTHREAD_MUTEX_INITIALIZER;
#define OUT_LOCK() (kg_stack_check_locked(), pthread_mutex_lock(&g_out))
#define OUT_UNLOCK() pthread_mutex_unlock(&g_out)
#else
#define OUT_LOCK() ((void)0)
#define OUT_UNLOCK() ((void)0)
#endif
// A line is one locked write sequence, so lines from parallel tasks never interleave.
static void write_line(int fd, const char* pre, size_t pn, const char* p, size_t n) {
    char buf[1024];
    OUT_LOCK();
    if (pn + n < sizeof buf) {
        memcpy(buf, pre, pn);
        memcpy(buf + pn, p, n);
        buf[pn + n] = '\n';
        rt_sys_write(fd, buf, pn + n + 1);
    } else {
        if (pn) rt_sys_write(fd, pre, pn);
        rt_sys_write(fd, p, n);
        rt_sys_write(fd, "\n", 1);
    }
    OUT_UNLOCK();
}
static void rt_fatal(const char* msg) {
    write_line(2, "panic: ", 7, msg, strlen(msg));
#ifdef KIGUMI_LEAK_CHECK
    report_live();
#endif
    rt_sys_exit(2);
}
void rt_panic(const char* msg) {
#ifdef KG_CATCH
    if (g_task_catch) task_fault(msg);
#endif
    rt_fatal(msg);
}
#ifdef KG_CATCH
// Mask words and then slots follow the header.
typedef struct kg_uframe {
    struct kg_unwind* prev;
    void (*run)(struct kg_unwind*);
    int64_t n;
} kg_uframe;
static void unwind_slots(val* slots, uint64_t* mask, int64_t n) {
    for (int64_t k = n - 1; k >= 0; k--) {
        uint64_t* w = &mask[k / 64];
        uint64_t bit = (uint64_t)1 << (k % 64);
        if (!(*w & bit)) continue;
        *w &= ~bit;
        val v = slots[k];
        slots[k] = NULL;
        // Bit 0 marks an inline Option or Result: the next slot holds the helper that drops it in place.
        if ((uintptr_t)v & 1) {
            ((void (*)(void*))slots[k + 1])((void*)((uintptr_t)v & ~(uintptr_t)1));
            continue;
        }
        rt_release(v);
    }
}
// A release that panics leaves the other set bits for the nested fault to run.
static void unwind_frame(kg_unwind* u) {
    kg_uframe* f = (kg_uframe*)u;
    uint64_t* mask = (uint64_t*)(f + 1);
    unwind_slots((val*)(mask + (f->n + 63) / 64), mask, f->n);
}
static void unwind_frame_slots(kg_unwind* u) {
    unwind_slots((val*)u->p, u->m, u->i);
}
// The arguments rt_invoke still owes a release; u->i counts the ones left, so a release that panics resumes with the rest.
static void unwind_args(kg_unwind* u) {
    val* args = (val*)u->p;
    while (u->i > 0) rt_release(args[--u->i]);
}
// The same, for an argument region the call also owns; freed once every release has run.
static void unwind_args_region(kg_unwind* u) {
    unwind_args(u);
    gen_free(&g_general, u->p);
}
// The arguments of a std function value, released in order; u->i is the next one, so a release that panics resumes with the rest.
typedef struct {
    val* args;
    int64_t n;
    uint64_t lent;
    val* heap;
} kg_stdargs;
static void unwind_std_args(kg_unwind* u) {
    kg_stdargs* s = (kg_stdargs*)u->p;
    if (s->heap) gen_free(&g_general, s->heap);
    s->heap = NULL;
    while (u->i < s->n) {
        int64_t k = u->i++;
        if (k >= 64 || !((s->lent >> k) & 1)) rt_release(s->args[k]);
    }
}
#define IS_FRAME_REC(u) \
    ((u)->run == unwind_frame || (u)->run == unwind_frame_slots || (u)->run == unwind_args || (u)->run == unwind_args_region || (u)->run == unwind_std_args)
void rt_unwind_enter(void* rec, int64_t n) {
    kg_uframe* f = (kg_uframe*)rec;
    f->prev = g_unwind;
    f->run = unwind_frame;
    f->n = n;
    g_unwind = (kg_unwind*)f;
}
void rt_unwind_enter_slots(void* rec, void* slots, uint64_t* mask, int64_t n) {
    kg_unwind* u = (kg_unwind*)rec;
    u->prev = g_unwind;
    u->run = unwind_frame_slots;
    u->p = slots;
    u->v = NULL;
    u->i = n;
    u->m = mask;
    g_unwind = u;
}
// The record is a kg_unwind followed by the kg_stdargs it points at.
void rt_unwind_enter_std(void* rec, val* args, int64_t n, int64_t lent) {
    kg_unwind* u = (kg_unwind*)rec;
    kg_stdargs* s = (kg_stdargs*)(u + 1);
    s->args = args;
    s->n = n;
    s->lent = (uint64_t)lent;
    s->heap = NULL;
    u->prev = g_unwind;
    u->run = unwind_std_args;
    u->p = s;
    u->v = NULL;
    u->i = 0;
    u->m = NULL;
    g_unwind = u;
}
void rt_unwind_leave(void* rec) {
    g_unwind = ((kg_unwind*)rec)->prev;
}
// Nothing catches a panic outside a task, so the caller sweeps its own frame after a faulting release.
int rt_release_outer(val v) {
    if (g_task_catch) {
        rt_release(v);
        return 0;
    }
    uint64_t seen = g_fault_count;
    g_outer_drop++;
    rt_release(v);
    g_outer_drop--;
    return g_fault_count != seen;
}
void rt_sweep_release(val v) {
    g_outer_drop++;
    rt_release(v);
    g_outer_drop--;
}
void rt_outer_fatal(void) {
    rt_fatal(g_fault_msg);
}
void rt_drop_outer(val v) {
    if (rt_release_outer(v)) rt_outer_fatal();
}
#else
int rt_release_outer(val v) {
    rt_release(v);
    return 0;
}
void rt_sweep_release(val v) {
    rt_release(v);
}
void rt_outer_fatal(void) {
    rt_fatal("panic");
}
void rt_drop_outer(val v) {
    rt_release(v);
}
void rt_unwind_enter(void* rec, int64_t n) {
    (void)rec;
    (void)n;
}
void rt_unwind_enter_slots(void* rec, void* slots, uint64_t* mask, int64_t n) {
    (void)rec;
    (void)slots;
    (void)mask;
    (void)n;
}
void rt_unwind_enter_std(void* rec, val* args, int64_t n, int64_t lent) {
    (void)rec;
    (void)args;
    (void)n;
    (void)lent;
}
void rt_unwind_leave(void* rec) {
    (void)rec;
}
#endif
// rt_panicf's every caller passes a fixed message with exactly one "%s"; this
// avoids a full printf just for that one substitution.
void rt_panicf(const char* fmt, const char* a) {
    char buf[512];
    size_t n = 0;
    for (const char* f = fmt; *f && n + 1 < sizeof buf; f++) {
        if (f[0] == '%' && f[1] == 's') {
            for (const char* s = a; *s && n + 1 < sizeof buf; s++) buf[n++] = *s;
            f++;
            continue;
        }
        buf[n++] = *f;
    }
    buf[n] = 0;
    rt_panic(buf);
}

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
// NULL on refusal instead of strdup_val's panic, for the fallible primitives.
static char* strdup_or_null(const char* s, int64_t len) {
    if (len < 0 || fault_tick()) return NULL;
    char* d = kg_malloc_n(len);
    if (!d) return NULL;
    memcpy(d, s, len);
    d[len] = 0;
    return d;
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
    if (!v->u.arr.items) rt_panic("out of memory");
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
val rt_record(rt_type* t, int64_t n, val* fields) {
    if (!t) rt_panic("a runtime-named type was dropped by reachability");
    val v = alloc(K_RECORD);
    v->u.rec.type = t;
    v->u.rec.n = n;
    v->u.rec.fields = amalloc(v->al, (n ? n : 1) * sizeof(val));
    if (!v->u.rec.fields) rt_panic("out of memory");
    for (int64_t i = 0; i < n; i++) {
        v->u.rec.fields[i] = fields[i];
    }
    return v;
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
    if (!v->u.var.payload) rt_panic("out of memory");
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
        val c = rt_record(v->u.rec.type, v->u.rec.n, v->u.rec.fields);
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

#include "rt_display.inc"
#include "rt_number.inc"

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
val rt_invoke(adapter_fn fn, val env, int n, val* args) {
    return invoke_args(fn, env, n, args, 0);
}
// A call's argument count has no compile-time cap, and a variable-length
// stack array needs __chkstk, which freestanding_symbols.txt refuses.
#define STACK_ARGS 16
typedef struct kg_async_desc {
    val (*poll)(val);
    void (*drop)(val);
    int32_t nslots;
    int32_t mobile;
    int32_t nref;
    const uint8_t* referent;
} kg_async_desc;
// args move into the frame the Future owns; a `&mut` write-back cell the caller takes back at once, bare or behind an
// existential view, goes in as its referent where the body's parameter takes one (the descriptor's referent flags).
static val callv_async(val f, int n, val* args) {
    const kg_async_desc* d = (const kg_async_desc*)(void*)f->u.clo.fn;
    int off = f->u.clo.async == 2 || !f->u.clo.env ? 0 : (int)f->u.clo.env->u.arr.len;
    for (int i = 0; i < n; i++) {
        val a = args[i];
        if (off + i >= d->nref || !d->referent[off + i] || !a) continue;
        if (a->kind == K_CELL) {
            args[i] = rt_retain(a->u.cell.v);
            rt_release(a);
        } else if (a->kind == K_BOX) {
            a->u.box.src = NULL;
        }
    }
    val fut = rt_future_new(f, n, args);
    for (int i = 0; i < n; i++) rt_release(args[i]);
    return fut;
}
// rt_callv_m is rt_callv for a caller declaring each argument lent (a bit in lent) or handed over (in given): a lent one
// a parameter of f consumes is retained first, a handed one a parameter only borrows is released after.
val rt_callv_m(val f, int n, val* args, uint32_t lent, uint32_t given) {
    val g = rt_deref(f);
    uint32_t m = g->kind == K_CLOSURE && !g->u.clo.async ? g->u.clo.lent : 0;
    // An async value's Future keeps every argument, so it borrows none.
    int known = g->kind == K_CLOSURE && (g->u.clo.async || g->u.clo.key || ((m >> 31) & 1));
    for (int i = 0; known && i < n && i < 31; i++)
        if (((lent >> i) & 1) && !((m >> i) & 1)) rt_retain(args[i]);
    val r = rt_callv(f, n, args);
    for (int i = 0; known && i < n && i < 31; i++)
        if (((given >> i) & 1) && ((m >> i) & 1)) rt_release(args[i]);
    return r;
}

// A primitive passes its callback borrows of what it holds.
val rt_callv_lent(val f, int n, val* args) {
    return rt_callv_m(f, n, args, 0x7fffffffu, 0);
}

val rt_callv(val f, int n, val* args) {
    f = rt_deref(f);
    if (f->kind != K_CLOSURE) rt_panic("value is not callable");
    if (f->u.clo.async) return callv_async(f, n, args);
#ifdef KIGUMI_STD_DYNAMIC
    if (f->u.clo.key) {
        int64_t envlen = f->u.clo.env->u.arr.len;
        int64_t total = envlen + n;
        val stackbuf[STACK_ARGS];
        val* all = total <= STACK_ARGS ? stackbuf : gen_alloc(&g_general, total * sizeof(val));
        int m = 0;
        for (int64_t i = 0; i < envlen; i++) all[m++] = f->u.clo.env->u.arr.items[i];
        for (int i = 0; i < n; i++) all[m++] = args[i];
#ifdef KG_CATCH
        kg_stdargs held = {args, n, f->u.clo.lent, all == stackbuf ? NULL : all};
        UNWIND_PUSH(u, unwind_std_args, &held, NULL, 0);
#endif
        val r = rt_std(f->u.clo.key, m, all);
        UNWIND_POP(u);
        for (int i = 0; i < n; i++)
            if (i >= 32 || !((f->u.clo.lent >> i) & 1)) rt_release(args[i]);
        if (all != stackbuf) gen_free(&g_general, all);
        return r;
    }
#endif
    return rt_invoke(f->u.clo.fn, f->u.clo.env, n, args);
}
// rt_call_slot calls requirement slot of the interface a boxed value was
// boxed as. rt_invoke retains the receiver once and releases it, like a direct
// call's borrow. recvmode: 1 `mut self`, 2 `move self` (consumes the box).
val rt_call_slot(val recv, int slot, int n, val* args, int recvmode) {
    val target = rt_deref(recv);
    if (target->kind != K_BOX || !target->u.box.vt) rt_panic("dynamic call on a value without a witness table");
    adapter_fn fn = (adapter_fn)target->u.box.vt[slot];
    int64_t total = n + 1;
    val stackbuf[STACK_ARGS];
    val* all = total <= STACK_ARGS ? stackbuf : gen_alloc(&g_general, total * sizeof(val));
    if (!all) rt_panic("out of memory");
    struct obj cell;
    // A concrete `mut self` body writes self through a cell; the boxed
    // value is behind the existential, so the cell is built over it here.
    if (recvmode == 1 && target->u.box.src) {
        all[0] = target->u.box.src;
    } else if (recvmode == 1) {
        rt_cell_init(&cell, rt_retain(target->u.box.v));
        all[0] = &cell;
    } else {
        all[0] = target->u.box.v;
    }
    for (int i = 0; i < n; i++) all[i + 1] = args[i];
    val r = invoke_args(fn, NULL, (int)total, all, all != stackbuf);
    if (recvmode == 1 && target->u.box.src) {
        target->u.box.v = rt_deref(target->u.box.src);
    } else if (recvmode == 1 && target->owned) {
        rt_release(cell.u.cell.v);
    } else if (recvmode == 1) {
        val old = target->u.box.v;
        target->u.box.v = cell.u.cell.v;
        rt_release(old);
    } else if (recvmode == 2 && recv == target) {
        target->u.box.v = NULL;
        rt_release(recv);
    }
    return r;
}
#ifdef KIGUMI_STD_DYNAMIC
val rt_std_closure(const char* key, int n, val* caps) {
    val v = rt_closure(NULL, n, caps);
    v->u.clo.key = key;
    return v;
}
// rt_std_bind is a primitive used as a witness: lent (bit i for argument
// i) marks the receiver and the borrowed parameters, which rt_callv must
// leave alone as a direct call would.
val rt_std_bind(const char* key, uint32_t lent, int n, val* caps) {
    val v = rt_std_closure(key, n, caps);
    v->u.clo.lent = lent;
    return v;
}
#endif
void rt_drop(val v) {
    rt_release(v);
}
// A frame's slots[] holds nslots words of locals then the arguments; a slot may hold inline aggregate bytes or a raw scalar.
#define KG_FRAME_PANICKED (1 << 30)
typedef struct kgframe {
    const kg_async_desc* desc;
    val awaited;
    int32_t state;
    int32_t nargs;
    int32_t force;
    val* args;
    _Alignas(8) val slots[];
} kgframe;
// A poll returns NULL for Pending and this marker for Cancelled; neither
// can be an ordinary value.
struct obj kg_cancel_marker = {.kind = K_UNIT, .rc = -1};
#define KG_CANCELLED (&kg_cancel_marker)

enum { T_CREATED, T_RUNNABLE, T_RUNNING, T_SUSPENDED, T_COMPLETED, T_CANCELLED, T_FAULTED };
typedef struct kgtask kgtask;
typedef struct kgexec kgexec;
struct kgtask {
    val fut, result;
    kgexec *ex, *pex;
    kgtask *parent, *waiter;
    kgtask** kids;
    char* fault;
    int nkids, capkids;
    // kid_at is the slot in parent->kids; open_kids counts non-terminal kids and gone_kids the NULL holes.
    int kid_at, open_kids, gone_kids;
    int state, ending, notified, queued, draining, wait_kids, closing;
    // detached says the Task handle is gone, so nothing can await this task any more.
    int detached;
    // close_mark is the closing pass that last found this task under its scope.
    unsigned long close_mark;
    // refs and cancel are read outside the scheduler lock: a Task handle can
    // be dropped on any lane, and a poll checks cancellation as it runs.
    KG_AINT refs, cancel;
    // lane is the home lane a pinned task polls on; -1 means any worker may take it.
    int lane;
    // tree is the root of this task's runToCompletion; the root's active counts the polls still finishing, so it outlives them.
    kgtask* tree;
    KG_AINT active;
    int tail_wait;
};
typedef struct kgqueue {
    kgtask** q;
    int head, len, cap;
} kgqueue;
struct kgexec {
    kgqueue pool;
    kgqueue* lane;
    // live counts the tasks admitted here that have not reached a terminal
    // state; dropped says the Executor handle is gone.
    int nlanes, driving, running, external, live;
#ifdef KIGUMI_THREADS
    pthread_cond_t cv;
    pthread_t* threads;
    struct kgworker* wk;
    int nworkers, dropped;
#endif
};
static KG_TLS kgtask* g_cur_task;
static KG_TLS int g_cur_lane;
#ifdef KIGUMI_THREADS
// A worker with an empty queue may leave only once the handle is gone and
// nothing it admitted is still running.
static int exec_finished(kgexec* e) { return e->dropped && !e->live; }
#endif
// The scheduler lock covers every field above, so it also supplies the
// happens-before edge between a result's publish and the awaiter's read.
#ifdef KIGUMI_THREADS
static KG_TLS val* g_defer;
static KG_TLS int g_ndefer, g_capdefer, g_flushing;
// Releasing under the scheduler lock could run user drop code that spawns
// or cancels, so a release from a locked section waits for the unlock.
static void sched_release(val v) {
    if (!v) return;
    if (g_ndefer == g_capdefer) {
        int c = g_capdefer ? g_capdefer * 2 : 16;
        val* p = realloc(g_defer, (size_t)c * sizeof(val));
        if (!p) rt_fatal("out of memory");
        g_defer = p;
        g_capdefer = c;
    }
    g_defer[g_ndefer++] = v;
}
static void sched_flush(void) {
    if (g_flushing) return;
    g_flushing = 1;
    for (int i = 0; i < g_ndefer; i++) {
        val v = g_defer[i];
        g_defer[i] = NULL;
        rt_release(v);
    }
    g_ndefer = 0;
    g_flushing = 0;
}
#else
#define sched_release(v) rt_release(v)
#define sched_flush() ((void)0)
#endif

static kgframe* frame_of(val v) {
    if (!v || v->kind != K_OPAQUE || strcmp(v->u.op.kind, "Future")) rt_panic("await on a value that is not a Future");
    return (kgframe*)v->u.op.data;
}
val* rt_future_slots(val fut) { return frame_of(fut)->slots; }
int rt_future_state(val fut) { return frame_of(fut)->state; }
val rt_future_arg(val fut, int64_t i) { return frame_of(fut)->args[i]; }
void rt_future_started(val fut) { frame_of(fut)->nargs = 0; }
void rt_future_suspend(val fut, int k, val awaited) {
    kgframe* f = frame_of(fut);
    f->state = k;
    f->awaited = awaited;
}
val rt_future_complete(val fut, val v) {
    frame_of(fut)->state = -1;
    return v;
}
// A value that is not a compiler-built Future (a BlockingPool result) has
// no mobility summary, so it stays on the lane that created it.
static int future_mobile(val fut) {
    val d = rt_deref(fut);
    if (!d || d->kind != K_OPAQUE || strcmp(d->u.op.kind, "Future")) return 0;
    return ((kgframe*)d->u.op.data)->desc->mobile;
}
val rt_future_new(val callee, int n, val* args) {
    val clo = rt_deref(callee);
    if (clo->kind != K_CLOSURE || !clo->u.clo.fn) rt_panic("this Future has no async body to run");
    const kg_async_desc* d = (const kg_async_desc*)(void*)clo->u.clo.fn;
    val env = clo->u.clo.env;
    int ne = env ? (int)env->u.arr.len : 0;
    int total = ne + n;
    kgframe* f = calloc(1, sizeof(kgframe) + (size_t)(d->nslots + total) * sizeof(val));
    if (!f) rt_panic("out of memory");
    f->desc = d;
    f->nargs = total;
    f->args = f->slots + d->nslots;
    int m = 0;
    int trailing = clo->u.clo.async == 2;
    if (!trailing)
        for (int i = 0; i < ne; i++) f->args[m++] = rt_retain(env->u.arr.items[i]);
    for (int i = 0; i < n; i++) f->args[m++] = rt_retain(args[i]);
    if (trailing)
        for (int i = 0; i < ne; i++) f->args[m++] = rt_retain(env->u.arr.items[i]);
    return rt_opaque("Future", f);
}
#ifdef KG_CATCH
#define KG_FAULTING g_faulting
#else
#define KG_FAULTING 0
#endif
// Dropping a parked Future cancels it first, so its cleanups run once.
static int frame_cancel_run(val fut) {
    kgframe* f = frame_of(fut);
    if (f->state <= 0 || f->state >= KG_FRAME_PANICKED || !g_cur_task || KG_FAULTING) return 0;
    f->force = 1;
    if (f->desc->poll(fut) != KG_CANCELLED) rt_fatal("a cancelled Future did not finish as cancelled");
    return 1;
}
#if defined(KG_CATCH) || defined(KIGUMI_CAP_TASK_EXECUTOR)
static char* dup_msg(const char* m) {
    size_t n = strlen(m) + 1;
    char* d = malloc(n);
    if (d) memcpy(d, m, n);
    return d;
}
#endif
#ifdef KG_CATCH
static void join_fault(const char* msg) {
    char* m = dup_msg(msg);
    if (!m) rt_fatal(msg);
    g_fault_count++;
    if (!g_fault_msg) {
        g_fault_msg = m;
        return;
    }
    size_t a = strlen(g_fault_msg), b = strlen(m);
    char* j = malloc(a + b + 9);
    if (!j) rt_fatal(msg);
    memcpy(j, g_fault_msg, a);
    memcpy(j + a, "\npanic: ", 8);
    memcpy(j + a + 8, m, b + 1);
    free(g_fault_msg);
    free(m);
    g_fault_msg = j;
}
// A frame record stays on the stack while it runs so a panic inside resumes it; other records are one-shot and pop first.
#ifdef KIGUMI_CAP_TASK_EXECUTOR
static void task_close_on_panic(void);
#endif
static void task_raise(void) {
#ifdef KIGUMI_CAP_TASK_EXECUTOR
    if (!g_faulting && !g_drop_depth && g_cur_task && !g_cur_task->closing) task_close_on_panic();
#endif
    while (g_unwind != g_unwind_base) {
        kg_unwind* u = g_unwind;
        if (!IS_FRAME_REC(u)) g_unwind = u->prev;
        if (!IS_FRAME_REC(u) || !g_cur_task || g_cur_task->parent) u->run(u);
        if (g_unwind == u) g_unwind = u->prev;
    }
    KG_LONGJMP(*g_task_catch);
}
static void task_fault(const char* msg) {
    g_try_bare = 0;
    join_fault(msg);
    task_raise();
}
static __attribute__((noinline)) int contained(void (*fn)(val), val arg) {
    kg_jmp_buf jb;
    kg_jmp_buf* prev = g_task_catch;
    int sp = g_alloc_sp;
    int depth = g_drop_depth;
    kg_unwind* prev_base = g_unwind_base;
    g_unwind_base = g_unwind;
    if (KG_SETJMP(jb)) {
        g_task_catch = prev;
        kg_stack_restore();
        g_unwind_base = prev_base;
        g_alloc_sp = sp;
        g_drop_depth = depth;
#ifdef KIGUMI_THREADS
        g_flushing = 0;
#endif
        return 0;
    }
    g_task_catch = &jb;
    fn(arg);
    g_task_catch = prev;
    g_unwind_base = prev_base;
    return 1;
}
static void frame_slots_drop(val fut) {
    kgframe* f = frame_of(fut);
    if (f->desc->drop) f->desc->drop(fut);
}
static void frame_release(val fut) {
    rt_release(fut);
}
static void frame_release_awaited(val fut) {
    kgframe* f = frame_of(fut);
    val a = f->awaited;
    f->awaited = NULL;
    if (!g_task_catch || !a) {
        rt_release(a);
        return;
    }
    uint64_t seen = g_fault_count;
    g_drop_depth++;
    contained(frame_release, a);
    g_drop_depth--;
    if (g_fault_count != seen && !g_drop_depth && !g_faulting) task_raise();
}
// Inside a task a drop that panics joins the fault and the rest of the frame still drops; the outermost frame then raises it.
static void frame_abandon(val fut) {
    kgframe* f = frame_of(fut);
    if (frame_cancel_run(fut)) return;
    if (!g_task_catch) {
        val a = f->awaited;
        f->awaited = NULL;
        rt_release(a);
        if (f->state > 0 && f->desc->drop) f->desc->drop(fut);
        f->state = -1;
        return;
    }
    uint64_t seen = g_fault_count;
    g_drop_depth++;
    val a = f->awaited;
    f->awaited = NULL;
    if (a) contained(frame_release, a);
    if (f->state > 0 && f->desc->drop)
        while (!contained(frame_slots_drop, fut)) {
        }
    f->state = -1;
    g_drop_depth--;
    if (g_fault_count != seen && !g_drop_depth && !g_faulting) task_raise();
}
#else
static void frame_release_awaited(val fut) {
    kgframe* f = frame_of(fut);
    val a = f->awaited;
    f->awaited = NULL;
    rt_release(a);
}
static void frame_abandon(val fut) {
    kgframe* f = frame_of(fut);
    if (frame_cancel_run(fut)) return;
    val a = f->awaited;
    f->awaited = NULL;
    rt_release(a);
    if (f->state > 0 && f->desc->drop) f->desc->drop(fut);
    f->state = -1;
}
#endif
static val poll_value(val v);
static val poll_future(val fut) {
    kgframe* f = frame_of(fut);
    if (f->state < 0) rt_panic("a Future that already completed cannot be polled again");
    return f->desc->poll(fut);
}
#ifdef KIGUMI_CAP_TASK_EXECUTOR
static int task_close_kids(kgtask* t);
#endif
// The children close before this body's cleanups run.
static val future_cancel(val fut) {
#ifdef KIGUMI_CAP_TASK_EXECUTOR
    if (!frame_of(fut)->force && g_cur_task && task_close_kids(g_cur_task)) return NULL;
#endif
    frame_release_awaited(fut);
    return KG_CANCELLED;
}
val rt_future_resume(val fut) {
    kgframe* f = frame_of(fut);
    if (f->force || (g_cur_task && KG_ALOAD(g_cur_task->cancel))) return future_cancel(fut);
    val r = poll_value(f->awaited);
    if (!r) return NULL;
    if (r == KG_CANCELLED) return future_cancel(fut);
    val a = f->awaited;
    f->awaited = NULL;
    rt_release(a);
    return r;
}
// A discarded `await` of a Task goes on past its cancellation; the result is dropped here.
val rt_future_settle(val fut) {
    kgframe* f = frame_of(fut);
    if (f->force || (g_cur_task && KG_ALOAD(g_cur_task->cancel))) return future_cancel(fut);
    val r = poll_value(f->awaited);
    if (!r) return NULL;
    if (r != KG_CANCELLED) rt_release(r);
    val a = f->awaited;
    f->awaited = NULL;
    rt_release(a);
    return rt_unit();
}

#ifdef KIGUMI_CAP_TASK_EXECUTOR
#include "rt_cap_task_executor.inc"
#endif
// The scope closes only when the body leaving is the task's own Future.
val rt_task_close(val fut) {
#ifdef KIGUMI_CAP_TASK_EXECUTOR
    if (g_cur_task && g_cur_task->fut && rt_deref(g_cur_task->fut) == fut) task_close_scope(g_cur_task);
#else
    (void)fut;
#endif
    return rt_unit();
}
#if !defined(KIGUMI_FREESTANDING) && defined(KIGUMI_CAP_TASK_BLOCKING_POOL)
#include "rt_cap_task_blocking_pool.inc"
#endif
static val poll_value(val v) {
    val d = rt_deref(v);
#ifdef KIGUMI_CAP_TASK_EXECUTOR
    if (d && d->kind == K_OPAQUE && !strcmp(d->u.op.kind, "Task")) return poll_task((kgtask*)d->u.op.data);
    if (d && d->kind == K_OPAQUE && !strcmp(d->u.op.kind, "Yield")) return poll_yield((int*)d->u.op.data);
#endif
#if !defined(KIGUMI_FREESTANDING) && defined(KIGUMI_CAP_TASK_BLOCKING_POOL)
    if (d && d->kind == K_OPAQUE && !strcmp(d->u.op.kind, "BlockingFuture")) return blocking_future_join((blocking_future*)d->u.op.data);
#endif
    return poll_future(d);
}
// A Shared cell's control block outlives the content for weak handles.
// borrow is a RefCell-style dynamic borrow flag (0 free, -1 exclusive, >0
// shared count), since Shared's receiver is a freely-copyable handle the
// static borrow checker does not track.
typedef struct sctrl {
    val cell, shared;
    int weak, alive, borrow, draining;
} sctrl;
#ifdef KG_CATCH
static void unwind_borrow_release(kg_unwind* u) {
    ((sctrl*)u->p)->borrow--;
}
static void unwind_borrow_free(kg_unwind* u) {
    ((sctrl*)u->p)->borrow = 0;
}
static void unwind_release_out(kg_unwind* u) {
    rt_release(u->v);
}
static void unwind_lend_back(kg_unwind* u) {
    ((val)u->p)->u.arr.items[u->i] = u->v->u.cell.v;
    u->v->u.cell.v = NULL;
    rt_release(u->v);
}
#endif
// Opaque values that own runtime state; Box, UnsafeCell, AllocatorScope, AllocGuard and Shared hand their content to the drain as a child slot.
static void free_opaque(val v) {
    const char* kind = v->u.op.kind;
    void* d = v->u.op.data;
    if (!strcmp(kind, "AllocatorHandle")) arena_close((arena*)d);
    else if (!strcmp(kind, "Uninit")) {
        ((val)d)->u.cell.v = NULL;
        rt_release((val)d);
    } else if (!strcmp(kind, "Weak")) {
        sctrl* c = d;
        if (--c->weak == 0 && !c->alive && !c->draining) free(c);
    } else if (!strcmp(kind, "Future")) {
        kgframe* f = (kgframe*)d;
        frame_abandon(v);
        for (int i = 0; i < f->nargs; i++) rt_release(f->args[i]);
        free(f);
    }
#ifdef KIGUMI_CAP_TASK_EXECUTOR
    else if (!strcmp(kind, "Task")) {
        task_detach((kgtask*)d);
        sched_flush();
    } else if (!strcmp(kind, "Yield")) free(d);
    else if (!strcmp(kind, "Executor")) exec_drop((kgexec*)d);
#endif
#if !defined(KIGUMI_FREESTANDING) && defined(KIGUMI_CAP_TASK_BLOCKING_POOL)
    else if (!strcmp(kind, "BlockingFuture")) blocking_future_free(d);
    else if (!strcmp(kind, "BlockingPool")) free(d);
#endif
}
enum { OPAQUE_PLAIN, OPAQUE_SLOT, OPAQUE_SHARED };
static int opaque_class(val v) {
    const char* kind = v->u.op.kind;
    if (!strcmp(kind, "Shared")) return OPAQUE_SHARED;
    if (!strcmp(kind, "AllocatorScope") || !strcmp(kind, "UnsafeCell") || !strcmp(kind, "Box") || !strcmp(kind, "AllocGuard")) return OPAQUE_SLOT;
    return OPAQUE_PLAIN;
}
#ifdef KG_CATCH
static void free_opaque_cb(val v) {
    free_opaque(v);
}
static void run_drop_hook(val v) {
    val a[1] = {v};
    rt_invoke((adapter_fn)v->u.rec.type->drop, NULL, 1, a);
}
static int has_drop_hook(val v) {
    return v->kind == K_RECORD && v->u.rec.type->kind == 1 && v->u.rec.type->drop;
}
static int child_may_run_drop(val c) {
    return c && c->rc == 1 && c->kind >= K_RECORD && c->kind != K_PTR;
}
static int is_borrow_slot(rt_type* t, int64_t i);
static int free_runs_user_code(val v) {
    if (has_drop_hook(v)) return 1;
    switch (v->kind) {
    case K_RECORD: {
        // A borrow field points at storage the record does not own; that referent may already be
        // gone when this record drops, so it is not a child to inspect.
        rt_type* t = v->u.rec.type;
        for (int64_t i = 0; i < v->u.rec.n; i++) {
            if (!is_borrow_slot(t, i) && child_may_run_drop(v->u.rec.fields[i])) return 1;
        }
        return 0;
    }
    case K_VARIANT:
        for (int64_t i = 0; i < v->u.var.n; i++) {
            if (child_may_run_drop(v->u.var.payload[i])) return 1;
        }
        return 0;
    case K_ARRAY:
        for (int64_t i = 0; i < v->u.arr.len; i++) {
            if (child_may_run_drop(v->u.arr.items[i])) return 1;
        }
        return 0;
    case K_BOX:
        return child_may_run_drop(v->u.box.v);
    case K_CLOSURE:
        return child_may_run_drop(v->u.clo.env);
    case K_CELL:
        return child_may_run_drop(v->u.cell.v);
    }
    return 0;
}
#endif
typedef struct dframe {
    val v;
    int64_t i, pend;
    uint64_t seen;
    unsigned char guarded, outer, opaque;
} dframe;
enum { DSTACK_INL = 16 };
typedef struct dstack {
    dframe* f;
    int64_t n, cap;
    int base;
    dframe inl[DSTACK_INL];
} dstack;
static int is_borrow_slot(rt_type* t, int64_t i) {
    return i < 64 && ((t->borrow_fields >> i) & 1);
}
// The next child slot of d, or NULL when none is left; advance moves the cursor past it.
static val* child_slot(dframe* d, int advance) {
    val v = d->v;
    val* slot = NULL;
    switch (v->kind) {
    case K_RECORD:
        while (d->i >= 0 && is_borrow_slot(v->u.rec.type, d->i)) d->i--;
        if (d->i < 0) return NULL;
        slot = &v->u.rec.fields[d->i];
        if (advance) d->i--;
        return slot;
    case K_VARIANT:
        if (d->i >= v->u.var.n) return NULL;
        slot = &v->u.var.payload[d->i];
        if (advance) d->i++;
        return slot;
    case K_ARRAY:
        if (d->i < 0) return NULL;
        slot = &v->u.arr.items[d->i];
        if (advance) d->i--;
        return slot;
    case K_BOX: slot = &v->u.box.v; break;
    case K_CLOSURE: slot = &v->u.clo.env; break;
    case K_CELL: slot = &v->u.cell.v; break;
    case K_OPAQUE: slot = d->opaque == OPAQUE_SHARED ? &((sctrl*)v->u.op.data)->cell : (val*)&v->u.op.data; break;
    default: return NULL;
    }
    if (d->i) return NULL;
    if (advance) d->i = 1;
    return slot;
}
static void free_storage(val v) {
    switch (v->kind) {
    case K_STR:
    case K_BYTES:
        if (v->owned) free((char*)v->u.s.data);
        break;
    case K_RECORD: afree(v->al, v->u.rec.fields); break;
    case K_VARIANT: afree(v->al, v->u.var.payload); break;
    case K_ARRAY: afree(v->al, v->u.arr.items ? v->u.arr.items - v->u.arr.head : NULL); break;
    }
    LIVE_OBJ(v->kind, -1);
#ifdef KIGUMI_RC_CHECK
    v->kind = 0;
#else
    afree(v->al, v);
#endif
}
#ifdef KG_CATCH
#define DEPTH_NOW() g_drop_depth
#define DEPTH_SUB(n) (g_drop_depth -= (n))
#else
#define DEPTH_NOW() 0
#define DEPTH_SUB(n) ((void)(n))
#endif
static int drain_push(dstack* st, dframe* fr) {
    if (st->n == st->cap) {
        int64_t cap = st->cap * 2;
        dframe* f = st->f == st->inl ? malloc((size_t)cap * sizeof *f) : realloc(st->f, (size_t)cap * sizeof *f);
        if (!f) return 0;
        if (st->f == st->inl) memcpy(f, st->inl, (size_t)st->n * sizeof *f);
        st->f = f;
        st->cap = cap;
    }
    st->f[st->n++] = *fr;
    return 1;
}
static void drain_discard(dstack* st) {
    if (st->f != st->inl) free(st->f);
}
// Frees the storage of a frame whose children are gone; returns whether a joined fault is still to be raised.
static int drain_close(dframe* fr) {
    int raise = 0;
    if (fr->opaque == OPAQUE_SHARED) {
        sctrl* c = fr->v->u.op.data;
        c->draining = 0;
        if (c->weak == 0) free(c);
    }
    free_storage(fr->v);
    DEPTH_SUB(fr->guarded + fr->pend);
#ifdef KG_CATCH
    if (fr->guarded) raise = g_fault_count != fr->seen && !g_drop_depth && !g_faulting;
#endif
    return raise;
}
static void raise_drop_fault(dstack* st) {
#ifdef KG_CATCH
    if (g_task_catch) {
        drain_discard(st);
        task_raise();
    }
#else
    (void)st;
#endif
}
// Starts freeing v, whose last reference is gone: runs its drop hook and, when it owns children, pushes a frame for them.
static void drain_open(dstack* st, val v, int64_t pend) {
    dframe fr = {v, 0, pend, 0, 0, 0, 0};
    int guarded = 0;
#ifdef KG_CATCH
    guarded = (g_task_catch || g_outer_drop) && free_runs_user_code(v);
    if (guarded) {
        fr.guarded = 1;
        fr.outer = g_drop_depth == 0;
        fr.seen = g_fault_count;
        g_drop_depth++;
        if (has_drop_hook(v)) {
            v->rc = -1;
            contained(run_drop_hook, v);
            v->rc = 0;
        }
    }
#endif
    switch (v->kind) {
    case K_RECORD: {
        rt_type* t = v->u.rec.type;
        if (!guarded && t->kind == 1 && t->drop) {
            v->rc = -1;
            val a[1] = {v};
            rt_invoke((adapter_fn)t->drop, NULL, 1, a);
            v->rc = 0;
        }
        fr.i = v->u.rec.n - 1;
        break;
    }
    case K_ARRAY:
        fr.i = v->u.arr.len - 1;
        break;
    case K_VARIANT:
    case K_BOX:
    case K_CLOSURE:
    case K_CELL:
        break;
    case K_OPAQUE:
        fr.opaque = (unsigned char)opaque_class(v);
        if (fr.opaque == OPAQUE_SHARED) {
            ((sctrl*)v->u.op.data)->alive = 0;
            ((sctrl*)v->u.op.data)->draining = 1;
        } else if (fr.opaque == OPAQUE_SLOT) {
            if (!strcmp(v->u.op.kind, "AllocGuard")) g_alloc_sp--;
        } else {
#ifdef KG_CATCH
            if (g_drop_depth > st->base) contained(free_opaque_cb, v);
            else
#endif
                free_opaque(v);
            free_storage(v);
            DEPTH_SUB(pend);
            return;
        }
        break;
    default:
        free_storage(v);
        DEPTH_SUB(pend);
        return;
    }
    if (drain_push(st, &fr)) return;
    val* slot;
    while ((slot = child_slot(&fr, 1))) {
        val c = *slot;
        if (guarded) *slot = NULL;
        if (release_dec(c)) free_obj(c);
    }
    if (drain_close(&fr)) raise_drop_fault(st);
}
// Children are released from an explicit stack, in the order a recursive release would visit them, so any ownership depth drops in constant C stack.
static void free_obj(val root) {
    dstack st;
    st.f = st.inl;
    st.n = 0;
    st.cap = DSTACK_INL;
    st.base = DEPTH_NOW();
    drain_open(&st, root, 0);
    while (st.n > 0) {
        dframe* fr = &st.f[st.n - 1];
        val* slot = child_slot(fr, 1);
        if (!slot) {
            int raise = drain_close(fr);
            st.n--;
            if (raise) raise_drop_fault(&st);
            continue;
        }
        val c = *slot;
        if (fr->guarded) *slot = NULL;
        if (!release_dec(c)) continue;
        int64_t pend = 0;
        if (!child_slot(fr, 0) && fr->opaque != OPAQUE_SHARED && !(fr->guarded && fr->outer)) {
            pend = fr->pend + fr->guarded;
            free_storage(fr->v);
            st.n--;
        }
        drain_open(&st, c, pend);
    }
    drain_discard(&st);
}
static rt_type* map_type(void) {
    static rt_type* t;
    static int looked;
    if (!looked) {
        t = rt_find_type("Map");
        looked = 1;
    }
    return t;
}
int64_t rt_iter_count(val it) {
    it = rt_deref(it);
    if (it->kind == K_ARRAY) return it->u.arr.len;
    if (it->kind == K_BYTES) return it->u.s.len;
    if (it->kind == K_RECORD && it->u.rec.type == map_type()) return it->u.rec.fields[0]->u.arr.len;
#ifdef KIGUMI_INT128
    if (W_IS(it->u.rec.fields[0]->nk)) return w_range_len(it->u.rec.fields[0], it->u.rec.fields[1], it->u.rec.fields[2]->u.b);
#endif
    int64_t lo = it->u.rec.fields[0]->u.i, hi = it->u.rec.fields[1]->u.i;
    int reversed = (it->u.rec.fields[0]->nk & 256) ? lo > hi : (uint64_t)lo > (uint64_t)hi;
    return reversed ? 0 : (int64_t)((uint64_t)hi - (uint64_t)lo + it->u.rec.fields[2]->u.b);
}
static val iter_len(val it) {
    return rt_int(rt_iter_count(it), 64);
}
/* A trivial scalar has no drop hook of its own: sharing it needs no
 * ownership transfer, only a retain. */
static int is_trivial_scalar(val v) {
    switch (v->kind) {
    case K_INT: case K_FLOAT: case K_BOOL: case K_STR: case K_BYTES:
    case K_CHAR: case K_UNIT:
        return 1;
    }
    return 0;
}
/* Takes ownership of *slot for the owned non-Copy case: a
 * trivial scalar (no drop of its own, and a generic loop body compiles
 * once for every T, so this also runs for a Copy instantiation such as
 * Array[T].fold with no `T: Copy` bound) is retained in place instead, so
 * a std loop re-reading the container mid-loop by index (Array.reverse)
 * still sees it. */
static val take_or_retain(val* slot) {
    if (is_trivial_scalar(*slot)) return rt_retain(*slot);
    val v = *slot;
    *slot = rt_unit();
    return v;
}
static val fixed_fill(val v, int64_t n) {
    val a = rt_array(0, NULL);
    for (int64_t i = 0; i < n; i++) rt_push(a, rt_copy(v));
    return a;
}
/* mode: 0 = iter.at (retains in place), 1 = iter.at_ref (borrowed, no
 * retain), 2 = iter.at_move (owned, transfers non-Copy elements). */
val rt_iter_at(val it, int64_t i, int mode) {
    it = rt_deref(it);
    if (it->kind == K_ARRAY) {
        if (mode == 1) return it->u.arr.items[i];
        if (mode == 2) return take_or_retain(&it->u.arr.items[i]);
        return rt_retain(it->u.arr.items[i]);
    }
    if (it->kind == K_BYTES) return rt_int((unsigned char)it->u.s.data[i], 8);
    if (it->kind == K_RECORD && it->u.rec.type == map_type()) {
        val keys = it->u.rec.fields[0], vals = it->u.rec.fields[1];
        rt_type* tup = rt_find_type("Tuple2");
        if (mode == 1) {
            val f[2] = {keys->u.arr.items[i], vals->u.arr.items[i]};
            return rt_record(tup, 2, f);
        }
        if (mode == 2) {
            val f[2] = {take_or_retain(&keys->u.arr.items[i]), take_or_retain(&vals->u.arr.items[i])};
            return rt_record(tup, 2, f);
        }
        val f[2] = {rt_retain(keys->u.arr.items[i]), rt_retain(vals->u.arr.items[i])};
        return rt_record(tup, 2, f);
    }
    val lo = it->u.rec.fields[0];
#ifdef KIGUMI_INT128
    if (W_IS(lo->nk)) return w_box(w_add(w_of(lo), w_new((uint64_t)i, 0)), lo->nk);
#endif
    return rt_int(lo->u.i + i, lo->nk);
}
// An integer element read straight into a register takes no box of its own.
int64_t rt_iter_at_int(val it, int64_t i) {
    it = rt_deref(it);
    if (it->kind == K_ARRAY) return rt_deref(it->u.arr.items[i])->u.i;
    if (it->kind == K_BYTES) return (unsigned char)it->u.s.data[i];
    return it->u.rec.fields[0]->u.i + i;
}
// print takes a String, so its bytes go out as they are and nothing is allocated.
static void print_line(int fd, val v) {
    v = rt_deref(v);
    if (v->kind == K_BOX) v = rt_deref(v->u.box.v);
    if (v->kind == K_STR && v->u.s.len >= 0) {
        write_line(fd, "", 0, v->u.s.data, v->u.s.len);
        return;
    }
    val s = rt_display(v);
    write_line(fd, "", 0, s->u.s.data, s->u.s.len);
    rt_release(s);
}
val rt_builtin(const char* name, int n, val* args) {
    if (!strcmp(name, "print")) {
        print_line(1, args[0]);
        return rt_unit();
    }
    if (!strcmp(name, "eprint")) {
        print_line(2, args[0]);
        fault_arm();
        return rt_unit();
    }
    if (!strcmp(name, "panic")) rt_panic(n > 0 ? rt_cstr(args[0]) : "explicit panic");
    if (!strcmp(name, "host")) return rt_opaque("Host", NULL);
    if (!strcmp(name, "cmp.unordered")) {
        val w = rt_deref(args[0]), a = rt_deref(args[1]), b = rt_deref(args[2]);
        const char* k = w->kind == K_CLOSURE && w->u.clo.key ? w->u.clo.key : "";
        if (!strncmp(k, "std/", 4)) k += 4;
        int fl = !strcmp(k, "prelude.f64.equals") || !strcmp(k, "prelude.f64.compareTo") || !strcmp(k, "prelude.f32.equals") || !strcmp(k, "prelude.f32.compareTo");
        return rt_bool(fl && ((a->kind == K_FLOAT && a->u.f != a->u.f) || (b->kind == K_FLOAT && b->u.f != b->u.f)));
    }
    if (!strcmp(name, "cmp.guard")) return rt_bool(rt_truth(args[0]) && !rt_truth(args[1]));
    if (!strcmp(name, "fixed.fill")) return fixed_fill(args[0], rt_deref(args[1])->u.i);
    if (!strcmp(name, "iter.len")) return iter_len(args[0]);
    if (!strcmp(name, "iter.at")) return rt_iter_at(args[0], rt_deref(args[1])->u.i, 0);
    if (!strcmp(name, "iter.at_ref")) return rt_iter_at(args[0], rt_deref(args[1])->u.i, 1);
    if (!strcmp(name, "iter.at_move")) return rt_iter_at(args[0], rt_deref(args[1])->u.i, 2);
    if (!strcmp(name, "alloc.push")) {
        val scope = rt_deref(args[0]);
        if (g_alloc_sp + 1 >= 64) rt_panic("allocator blocks nested too deeply");
        arena* ar = (arena*)rt_deref((val)scope->u.op.data)->u.op.data;
        g_alloc_stack[++g_alloc_sp] = &ar->a;
        return opaque_in(&((arena*)((val)scope->u.op.data)->u.op.data)->a, "AllocGuard", rt_retain(scope));
    }
    rt_panicf("unknown builtin %s", name);
    return NULL;
}

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
#ifdef KIGUMI_CAP_FFI_CSTRING
#include "rt_cap_ffi_cstring.inc"
#endif
#ifdef KIGUMI_CAP_FFI_PIN
#include "rt_cap_ffi_pin.inc"
#endif
#ifdef KIGUMI_CAP_FFI_RAW
#include "rt_cap_ffi_raw.inc"
#endif
#ifdef KIGUMI_CAP_FFI_HANDLE_TABLE
#include "rt_cap_ffi_handle_table.inc"
#endif
#ifdef KIGUMI_CAP_MEM_SECURE_ERASE
#include "rt_cap_mem_secure_erase.inc"
#endif
#ifdef KIGUMI_CAP_HOST_SHELL
#include "rt_cap_host_shell.inc"
#endif
#ifdef KIGUMI_CAP_HOST_ARGS
#include "rt_cap_host_args.inc"
#endif
#ifdef KIGUMI_CAP_HOST_FILES
#include "rt_cap_host_files.inc"
#endif
#ifdef KIGUMI_CAP_HOST_NET
#include "rt_cap_host_net.inc"
#endif
#ifdef KIGUMI_CAP_HOST_DL
#include "rt_cap_host_dl.inc"
#endif
#ifdef KIGUMI_CAP_HOST_ENTROPY
#include "rt_cap_host_entropy.inc"
#endif
#ifdef KIGUMI_CAP_HOST_SIGNALS
#include "rt_cap_host_signals.inc"
#endif
#ifdef KIGUMI_CAP_HOST_STDIO
#include "rt_cap_host_stdio.inc"
#endif
#include "rt_std_text.inc"
// A constructor keeps its argument as is: an interface value must stay a witness-carrying box inside the cell.
static int op_stores_a0(int op) {
    return op == OP_BOX_TRY_NEW || op == OP_SHARED_TRY_NEW || op == OP_UCELL_NEW;
}
#ifdef KIGUMI_THREADS
static pthread_mutex_t g_ucell_atomic = PTHREAD_MUTEX_INITIALIZER;
#define UCELL_LOCK() (kg_stack_check_locked(), pthread_mutex_lock(&g_ucell_atomic))
#define UCELL_UNLOCK() pthread_mutex_unlock(&g_ucell_atomic)
#else
#define UCELL_LOCK() ((void)0)
#define UCELL_UNLOCK() ((void)0)
#endif
// Cell content is a boxed scalar that may be shared, so atomics take one lock and store a fresh box; the named order is not used.
static val ucell_scalar(val v) {
    switch (v->kind) {
    case K_INT: return rt_int(v->u.i, v->nk);
    case K_BOOL: return rt_bool(v->u.b);
    case K_PTR: return rt_ptr(v->u.p);
    default: return rt_retain(v);
    }
}
static int ucell_same(val a, val b) {
    switch (a->kind) {
    case K_INT: return a->u.i == b->u.i;
    case K_BOOL: return a->u.b == b->u.b;
    default: return a->u.p == b->u.p;
    }
}
static val ucell_atomic(char op, val cell, val x, val y) {
    val c = (val)cell->u.op.data;
    UCELL_LOCK();
    val old = c->u.cell.v, res = ucell_scalar(old), next = NULL;
    switch (op) {
    case 'l': break;
    case 's':
        next = ucell_scalar(x);
        rt_release(res);
        res = rt_unit();
        break;
    case 'x': next = ucell_scalar(x); break;
    case 'c':
        if (ucell_same(old, x)) next = ucell_scalar(y);
        break;
    default: {
        uint64_t o = (uint64_t)old->u.i, v = (uint64_t)x->u.i;
        uint64_t r = op == '+' ? o + v : op == '-' ? o - v : op == '&' ? (o & v) : op == '|' ? (o | v) : (o ^ v);
        next = rt_int(truncate_to((int64_t)r, old->nk), old->nk);
    }
    }
    if (next) {
        c->u.cell.v = next;
        rt_release(old);
    }
    UCELL_UNLOCK();
    return res;
}
static val uninit_filled(val u) {
    val v = ((val)u->u.op.data)->u.cell.v;
    if (!v) rt_panic("Uninit slot is not initialized");
    return v;
}
// The lease is a private C-layout copy of the scalar, written back into the
// cell as a fresh object: the old one may be a shared immortal literal.
static int scalar_lease_bytes(val v) {
    switch (v->kind) {
    case K_INT: return (v->nk & 255) <= 64 ? (v->nk & 255) / 8 : 0;
    case K_FLOAT: return (v->nk & 255) == 32 ? 4 : 8;
    case K_BOOL: return 1;
    case K_CHAR: return 4;
    default: return 0;
    }
}
static void scalar_lease_load(val v, void* buf) {
    switch (scalar_lease_bytes(v)) {
    case 1: *(uint8_t*)buf = v->kind == K_BOOL ? (uint8_t)v->u.b : (uint8_t)v->u.i; break;
    case 2: *(uint16_t*)buf = (uint16_t)v->u.i; break;
    case 4:
        if (v->kind == K_FLOAT) *(float*)buf = (float)v->u.f;
        else if (v->kind == K_CHAR) *(uint32_t*)buf = v->u.c;
        else *(uint32_t*)buf = (uint32_t)v->u.i;
        break;
    default:
        if (v->kind == K_FLOAT) *(double*)buf = v->u.f;
        else *(int64_t*)buf = v->u.i;
    }
}
static val scalar_lease_store(val v, const void* buf) {
    int signd = v->nk & 256;
    switch (v->kind) {
    case K_BOOL: return rt_bool(*(const uint8_t*)buf != 0);
    case K_CHAR: return rt_char(*(const uint32_t*)buf);
    case K_FLOAT: return rt_float(scalar_lease_bytes(v) == 4 ? (double)*(const float*)buf : *(const double*)buf, v->nk);
    default: break;
    }
    switch (scalar_lease_bytes(v)) {
    case 1: return rt_int(signd ? (int64_t) * (const int8_t*)buf : (int64_t) * (const uint8_t*)buf, v->nk);
    case 2: return rt_int(signd ? (int64_t) * (const int16_t*)buf : (int64_t) * (const uint16_t*)buf, v->nk);
    case 4: return rt_int(signd ? (int64_t) * (const int32_t*)buf : (int64_t) * (const uint32_t*)buf, v->nk);
    default: return rt_int(*(const int64_t*)buf, v->nk);
    }
}
static val with_address(val arg, val value, val f) {
    if (!scalar_lease_bytes(value)) {
        val ptr[1] = {rt_ptr((void*)&value->u)};
        return rt_callv(f, 1, ptr);
    }
    uint64_t buf[1] = {0};
    scalar_lease_load(value, buf);
    uint64_t before = buf[0];
    val p[1] = {rt_ptr(buf)};
    val r = rt_callv(f, 1, p);
    while (arg->kind == K_CELL && arg->u.cell.v->kind == K_CELL) arg = arg->u.cell.v;
    if (buf[0] != before && arg->kind == K_CELL) rt_cellset(arg, scalar_lease_store(value, buf));
    return r;
}
// rt_std_id never compares key strings: llgen resolves a call's std key to
// an enum std_op at compile time (rt_std_op_of does the same resolution
// for rt_std's few dynamic callers).
val rt_std_id(int op, int n, val* a);
// A try op for a caller holding Result[T, AllocError] inline: the Ok payload, or NULL for the refusal.
val rt_d_try(int op, int n, val* a) {
    g_try_bare = 1;
    val r = rt_std_id(op, n, a);
    int bare = !g_try_bare;
    g_try_bare = 0;
    if (bare || !r || r->kind != K_VARIANT) return r;
    if (r->u.var.v == V_Err) {
        rt_release(r);
        return NULL;
    }
    val p = rt_retain(r->u.var.payload[0]);
    rt_release(r);
    return p;
}
// The same for an Ok payload that is an Option: 0 is the refusal, 1 Ok(None), 2 Ok(Some(*out)).
int rt_d_try_opt(int op, int n, val* a, val* out) {
    val r = rt_d_try(op, n, a);
    *out = NULL;
    if (!r) return 0;
    if (r->kind == K_VARIANT && r->u.var.v == V_None) return 1;
    if (r->kind == K_VARIANT && r->u.var.v == V_Some) {
        *out = rt_retain(r->u.var.payload[0]);
        rt_release(r);
        return 2;
    }
    *out = r;
    return 2;
}
#include "rt_std_dispatch.inc"
// rt_std is rt_callv's path for a std function value (rt_std_closure /
// rt_std_bind carry the key, resolved fresh on every call); a static call
// site instead calls rt_std_id directly at its own compile-time-known op.
#ifdef KIGUMI_STD_DYNAMIC
val rt_std(const char* key, int n, val* a) {
    int op = rt_std_op_of(key);
    if (op >= 0) return rt_std_id(op, n, a);
    val a0 = n > 0 ? rt_deref(a[0]) : NULL, a1 = n > 1 ? rt_deref(a[1]) : NULL, a2 = n > 2 ? rt_deref(a[2]) : NULL;
    if (a0 && a0->kind == K_BOX) a0 = a0->u.box.v;
    val r = rt_sys_std(key, n, a, a0, a1, a2);
    if (r) return r;
    rt_panicf("no runtime for %s", key);
    return NULL;
}
#endif
// A std hash reached through a function value needs no box when the caller keeps the result in a register.
int64_t rt_callv_i64_m(val f, int n, val* args, uint32_t lent, uint32_t given) {
    val r = rt_callv_m(f, n, args, lent, given);
    int64_t x = rt_int_val(r);
    rt_release(r);
    return x;
}

int64_t rt_callv_i64(val f, int n, val* args) {
#ifdef KIGUMI_STD_DYNAMIC
    val g = rt_deref(f);
    if (g->kind == K_CLOSURE && g->u.clo.key && !g->u.clo.async && n == 1 && g->u.clo.env->u.arr.len == 0 &&
        rt_std_op_of(g->u.clo.key) == OP_HASH) {
        val a0 = rt_deref(args[0]);
        if (a0->kind == K_BOX) a0 = a0->u.box.v;
        int64_t h = (int64_t)rt_hash_of(a0);
        if (!(g->u.clo.lent & 1)) rt_release(args[0]);
        return h;
    }
#endif
    val r = rt_callv(f, n, args);
    int64_t x = rt_int_val(r);
    rt_release(r);
    return x;
}
