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
