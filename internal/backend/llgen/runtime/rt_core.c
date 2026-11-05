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

#include "rt_value.inc"
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
#include "rt_call.inc"
void rt_drop(val v) {
    rt_release(v);
}
#include "rt_async.inc"
#include "rt_drop.inc"
#include "rt_iter.inc"
#include "rt_builtin.inc"
#include "rt_utf8.inc"
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
#include "rt_mem.inc"
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
