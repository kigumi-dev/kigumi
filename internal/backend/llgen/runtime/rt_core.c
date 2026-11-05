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

#include "rt_alloc.inc"
#include "rt_rc.inc"
#include "rt_unwind.inc"
#include "rt_value.inc"
#include "rt_display.inc"
#include "rt_number.inc"

#include "rt_adt.inc"
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
