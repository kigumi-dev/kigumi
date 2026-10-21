static int task_terminal(kgtask* t) { return t->state >= T_COMPLETED; }
static void q_push(kgqueue* k, kgtask* t) {
    if (k->head + k->len == k->cap) {
        for (int i = 0; i < k->len; i++) k->q[i] = k->q[k->head + i];
        k->head = 0;
        if (k->len == k->cap) {
            k->cap = k->cap ? k->cap * 2 : 8;
            k->q = realloc(k->q, (size_t)k->cap * sizeof(kgtask*));
            if (!k->q) rt_fatal("out of memory");
        }
    }
    k->q[k->head + k->len++] = t;
}
static kgtask* q_pop(kgqueue* k) {
    if (k->len == 0) return NULL;
    kgtask* t = k->q[k->head++];
    if (--k->len == 0) k->head = 0;
    return t;
}
// Work stealing never reaches a home-lane queue: a pinned task polls only
// on the lane that owns it.
static void exec_push(kgexec* e, kgtask* t) {
    if (t->queued) return;
    t->queued = 1;
    t->state = T_RUNNABLE;
    q_push(t->lane < 0 ? &e->pool : &e->lane[t->lane], t);
#ifdef KIGUMI_THREADS
    pthread_cond_broadcast(&e->cv);
#endif
}
static kgtask* exec_take(kgexec* e, int lane) {
    kgtask* t = q_pop(&e->lane[lane]);
    if (!t) t = q_pop(&e->pool);
    if (t) t->queued = 0;
    return t;
}
static void task_unref(kgtask* t) {
    if (KG_DEC(t->refs) > 1) return;
    sched_release(t->fut);
    sched_release(t->result);
    free(t->kids);
    free(t->fault);
    free(t);
}
static void task_end(kgtask* t) {
    val f = t->fut;
    t->fut = NULL;
    sched_release(f);
    val r = t->result;
    t->result = NULL;
    sched_release(r);
}
// task_wake coalesces: it only marks the task runnable, never runs its code.
static void task_wake(kgtask* t) {
    if (task_terminal(t)) return;
    t->notified = 1;
    if (t->state == T_SUSPENDED) exec_push(t->ex, t);
}
static void task_request_cancel(kgtask* t);
static void task_cancel_kids(kgtask* t) {
    for (int i = 0; i < t->nkids; i++)
        if (t->kids[i]) task_request_cancel(t->kids[i]);
}
static void task_request_cancel(kgtask* t) {
    if (task_terminal(t) || KG_ALOAD(t->cancel)) return;
    KG_ASTORE(t->cancel, 1);
    task_cancel_kids(t);
    task_wake(t);
}
// The last kid to end wakes a task that is waiting on them.
static int task_close_kids(kgtask* t) {
    SCHED_LOCK();
    KG_ASTORE(t->cancel, 1);
    t->closing = 1;
    task_cancel_kids(t);
    int open = t->open_kids > 0;
    t->wait_kids = open;
    SCHED_UNLOCK();
    sched_flush();
    return open;
}
static void task_inherit_fault(kgtask* t) {
    if (t->ending == T_FAULTED) return;
    for (int i = 0; i < t->nkids; i++)
        if (t->kids[i] && t->kids[i]->state == T_FAULTED) {
            t->ending = T_FAULTED;
            t->fault = t->kids[i]->fault ? dup_msg(t->kids[i]->fault) : NULL;
            return;
        }
}
// A panic nobody awaits cancels the parent, and with it every sibling.
static void task_notify_scope(kgtask* t) {
    kgtask* p = t->parent;
    if (!p) return;
    p->open_kids--;
    if (t->state == T_FAULTED && !t->waiter) task_request_cancel(p);
#ifdef KIGUMI_THREADS
    if (p->closing) pthread_cond_broadcast(&p->ex->cv);
#endif
}
// An awaited child leaves its parent's scope, so its frame goes now instead of at the parent's end.
// Its slot becomes a NULL hole so the other kids keep their order without a copy.
static void task_forget(kgtask* t) {
    kgtask* p = t->parent;
    if (!p || t->kid_at >= p->nkids || p->kids[t->kid_at] != t) return;
    p->kids[t->kid_at] = NULL;
    task_end(t);
    task_unref(t);
    if (++p->gone_kids * 2 <= p->nkids) return;
    int n = 0;
    for (int i = 0; i < p->nkids; i++)
        if (p->kids[i]) {
            p->kids[i]->kid_at = n;
            p->kids[n++] = p->kids[i];
        }
    p->nkids = n;
    p->gone_kids = 0;
}
static void task_finish_drain(kgtask* t) {
    if (t->open_kids > 0) {
        t->state = T_SUSPENDED;
        return;
    }
    task_inherit_fault(t);
    t->state = t->ending;
    t->draining = 0;
    task_notify_scope(t);
    t->ex->live--;
#ifdef KIGUMI_THREADS
    if (exec_finished(t->ex)) pthread_cond_broadcast(&t->ex->cv);
#endif
    for (int i = 0; i < t->nkids; i++)
        if (t->kids[i]) {
            task_end(t->kids[i]);
            task_unref(t->kids[i]);
        }
    t->nkids = 0;
    if (t->pex && t->pex != t->ex) {
        t->pex->external--;
#ifdef KIGUMI_THREADS
        pthread_cond_broadcast(&t->pex->cv);
#endif
    }
    if (t->waiter) {
        kgtask* w = t->waiter;
        t->waiter = NULL;
        task_wake(w);
    }
    if (t->parent && (t->parent->draining || t->parent->wait_kids)) task_wake(t->parent);
}
// task_begin_finish cancels still-running kids; the terminal transition waits for all of them.
static void task_begin_finish(kgtask* t, int end) {
    t->ending = end;
    t->draining = 1;
    task_cancel_kids(t);
    task_finish_drain(t);
}
#ifdef KG_CATCH
static __attribute__((noinline)) val poll_body(kgtask* t, int* faulted) {
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
        *faulted = 1;
        return NULL;
    }
    g_task_catch = &jb;
    val r = poll_value(t->fut);
    g_task_catch = prev;
    g_unwind_base = prev_base;
    return r;
}
// Inner to outer; a drop that panics only costs the slot it was releasing.
static void frames_sweep(val root) {
    int n = 0;
    for (val v = root;;) {
        val d = deref(v);
        if (!d || d->kind != K_OPAQUE || strcmp(d->u.op.kind, "Future")) break;
        n++;
        v = ((kgframe*)d->u.op.data)->awaited;
    }
    val* chain = malloc((size_t)n * sizeof(val));
    if (!chain) rt_fatal("out of memory");
    val v = root;
    for (int i = 0; i < n; i++) {
        chain[i] = v;
        kgframe* f = frame_of(v);
        if (f->state >= 0) f->state = KG_FRAME_PANICKED;
        val next = f->awaited;
        if (i + 1 < n) f->awaited = NULL;
        v = next;
    }
    for (int i = n - 1; i >= 0; i--) {
        kgframe* f = frame_of(chain[i]);
        if (f->state > 0) {
            while (!contained(frame_slots_drop, chain[i])) {
            }
            f->state = -1;
        }
        contained(frame_release, chain[i]);
    }
    free(chain);
}
// Only the awaited chain was running; the root keeps its frames, as a fatal panic does.
static void task_unwind(kgtask* t) {
    if (!t->parent) {
        t->fault = g_fault_msg;
        g_fault_msg = NULL;
        return;
    }
    val f = t->fut;
    t->fut = NULL;
    g_faulting++;
    frames_sweep(f);
    g_faulting--;
    t->fault = g_fault_msg;
    g_fault_msg = NULL;
}
#else
static val poll_body(kgtask* t, int* faulted) {
    (void)faulted;
    return poll_value(t->fut);
}
static void task_unwind(kgtask* t) {
    (void)t;
}
#endif
static void task_run(kgtask* t, int lane) {
    SCHED_LOCK();
    if (task_terminal(t)) {
        SCHED_UNLOCK();
        return;
    }
    if (t->draining) {
        task_finish_drain(t);
        SCHED_UNLOCK();
        sched_flush();
        return;
    }
    t->state = T_RUNNING;
    t->notified = 0;
    SCHED_UNLOCK();
    sched_flush();
    kgtask* prev = g_cur_task;
    int prevlane = g_cur_lane;
    g_cur_task = t;
    g_cur_lane = lane;
    int faulted = 0;
    val r = poll_body(t, &faulted);
    if (faulted) task_unwind(t);
    g_cur_task = prev;
    g_cur_lane = prevlane;
    SCHED_LOCK();
    if (faulted) {
        task_begin_finish(t, T_FAULTED);
    } else if (!r) {
        t->state = T_SUSPENDED;
        if (t->notified) exec_push(t->ex, t);
    } else if (r == KG_CANCELLED) {
        task_begin_finish(t, T_CANCELLED);
    } else {
        t->result = r;
        task_begin_finish(t, T_COMPLETED);
    }
    SCHED_UNLOCK();
    sched_flush();
}
// Registers the waiter before reading target state, so a racing completion is still seen.
static val poll_task(kgtask* t) {
    SCHED_LOCK();
    if (!task_terminal(t)) {
        t->waiter = g_cur_task;
        SCHED_UNLOCK();
        return NULL;
    }
    t->waiter = NULL;
    int st = t->state;
    val r = t->result;
    t->result = NULL;
    const char* fault = t->fault;
    task_forget(t);
    SCHED_UNLOCK();
    sched_flush();
    if (st == T_CANCELLED) return KG_CANCELLED;
    if (st == T_FAULTED) rt_panic(fault ? fault : "the awaited task did not finish");
    return r;
}
#ifdef KIGUMI_THREADS
typedef struct kgworker {
    kgexec* e;
    int lane;
} kgworker;
static void* worker_main(void* p) {
    kgworker* w = (kgworker*)p;
    rt_sys_stack_guard();
    g_cur_lane = w->lane;
    SCHED_LOCK();
    for (;;) {
        kgtask* t = exec_take(w->e, w->lane);
        if (!t) {
            if (exec_finished(w->e)) break;
            pthread_cond_wait(&w->e->cv, &g_sched);
            continue;
        }
        w->e->running++;
        SCHED_UNLOCK();
        sched_flush();
        task_run(t, w->lane);
        SCHED_LOCK();
        w->e->running--;
        pthread_cond_broadcast(&w->e->cv);
    }
    SCHED_UNLOCK();
    sched_flush();
    return NULL;
}
static kgexec** g_orphans;
static int g_norphans, g_caporphans;
static kgexec** g_pools;
static int g_npools, g_cappools;
static void pool_register(kgexec* e) {
    SCHED_LOCK();
    if (g_npools == g_cappools) {
        g_cappools = g_cappools ? g_cappools * 2 : 4;
        g_pools = realloc(g_pools, (size_t)g_cappools * sizeof(kgexec*));
        if (!g_pools) rt_fatal("out of memory");
    }
    g_pools[g_npools++] = e;
    SCHED_UNLOCK();
}
static void pool_forget(kgexec* e) {
    SCHED_LOCK();
    for (int i = 0; i < g_npools; i++)
        if (g_pools[i] == e) {
            g_pools[i] = g_pools[--g_npools];
            break;
        }
    SCHED_UNLOCK();
}
static void exec_join(kgexec* e) {
    if (e->nworkers) pool_forget(e);
    for (int i = 0; i < e->nworkers; i++) pthread_join(e->threads[i], NULL);
    free(e->threads);
    free(e->wk);
    e->threads = NULL;
    e->wk = NULL;
    e->nworkers = 0;
}
#endif
static kgexec* exec_new(int workers) {
    kgexec* e = calloc(1, sizeof *e);
    if (!e) rt_panic("out of memory");
    e->nlanes = workers + 1;
    e->lane = calloc((size_t)e->nlanes, sizeof(kgqueue));
    if (!e->lane) rt_panic("out of memory");
#ifdef KIGUMI_THREADS
    pthread_cond_init(&e->cv, NULL);
#endif
    return e;
}
static void exec_free(kgexec* e) {
#ifdef KIGUMI_THREADS
    pthread_cond_destroy(&e->cv);
#endif
    for (int i = 0; i < e->nlanes; i++) free(e->lane[i].q);
    free(e->lane);
    free(e->pool.q);
    free(e);
}
// exec_drop runs from the Executor's drop, on any lane; joining a pool that
// still owes a task would hold that lane until another one finishes.
static void exec_drop(kgexec* e) {
#ifdef KIGUMI_THREADS
    if (e->nworkers) {
        SCHED_LOCK();
        e->dropped = 1;
        pthread_cond_broadcast(&e->cv);
        int idle = exec_finished(e) && !e->running;
        if (!idle) {
            if (g_norphans == g_caporphans) {
                g_caporphans = g_caporphans ? g_caporphans * 2 : 4;
                kgexec** o = realloc(g_orphans, (size_t)g_caporphans * sizeof(kgexec*));
                if (!o) rt_fatal("out of memory");
                g_orphans = o;
            }
            g_orphans[g_norphans++] = e;
        }
        SCHED_UNLOCK();
        if (!idle) return;
        exec_join(e);
    }
#endif
    exec_free(e);
}
// Runs once the task tree is terminal, so every worker only has to leave its loop.
static void exec_reap(void) {
#ifdef KIGUMI_THREADS
    SCHED_LOCK();
    kgexec** o = g_orphans;
    int n = g_norphans;
    g_orphans = NULL;
    g_norphans = g_caporphans = 0;
    SCHED_UNLOCK();
    for (int i = 0; i < n; i++) {
        exec_join(o[i]);
        exec_free(o[i]);
    }
    free(o);
#endif
}
static void exec_drain(void) {
#ifdef KIGUMI_THREADS
    exec_reap();
    SCHED_LOCK();
    int n = g_npools;
    kgexec** held = n ? malloc((size_t)n * sizeof(kgexec*)) : NULL;
    if (held) memcpy(held, g_pools, (size_t)n * sizeof(kgexec*));
    for (int i = 0; held && i < n; i++) {
        held[i]->dropped = 1;
        pthread_cond_broadcast(&held[i]->cv);
    }
    SCHED_UNLOCK();
    for (int i = 0; held && i < n; i++) exec_join(held[i]);
    free(held);
#endif
}
val rt_executor_new(void) { return opaque("Executor", exec_new(0)); }
val rt_thread_pool_new(int64_t workers) {
#ifdef KIGUMI_THREADS
    if (workers < 1) rt_panic("a thread pool executor needs at least one worker");
    if (workers > 256) rt_panic("a thread pool executor is bounded to 256 workers");
    kgexec* e = exec_new((int)workers);
    e->threads = calloc((size_t)workers, sizeof(pthread_t));
    e->wk = calloc((size_t)workers, sizeof(kgworker));
    if (!e->threads || !e->wk) rt_panic("out of memory");
    e->nworkers = (int)workers;
    pool_register(e);
    for (int i = 0; i < e->nworkers; i++) {
        e->wk[i].e = e;
        e->wk[i].lane = i + 1;
        if (pthread_create(&e->threads[i], NULL, worker_main, &e->wk[i])) rt_panic("could not start an executor worker thread");
    }
    return opaque("Executor", e);
#else
    (void)workers;
    rt_panic("this build has no thread support; use `task.currentThreadExecutor()`");
    return NULL;
#endif
}
val rt_run_to_completion(val exv, val fut) {
    if (g_cur_task) rt_panic("`Executor.runToCompletion` cannot run inside a task; `await` the future instead");
    kgexec* e = (kgexec*)deref(exv)->u.op.data;
    SCHED_LOCK();
    if (e->driving) {
        SCHED_UNLOCK();
        rt_panic("this executor is already running a task tree");
    }
    e->driving = 1;
    kgtask* root = calloc(1, sizeof *root);
    if (!root) rt_panic("out of memory");
    root->fut = fut;
    root->ex = e;
    KG_ASTORE(root->refs, 1);
    e->live++;
    exec_push(e, root);
    while (!task_terminal(root)) {
        kgtask* t = exec_take(e, 0);
        if (!t) {
#ifdef KIGUMI_THREADS
            if (e->running || e->external) {
                pthread_cond_wait(&e->cv, &g_sched);
                continue;
            }
#endif
            SCHED_UNLOCK();
            rt_panic("every task on this executor is waiting for something that cannot happen");
        }
        e->running++;
        SCHED_UNLOCK();
        sched_flush();
        task_run(t, 0);
        SCHED_LOCK();
        e->running--;
    }
    e->driving = 0;
    SCHED_UNLOCK();
    sched_flush();
    if (root->state == T_CANCELLED || root->state == T_FAULTED) exec_drain();
    if (root->state == T_CANCELLED) rt_panic("the root task was cancelled");
    if (root->state == T_FAULTED) rt_panic(root->fault ? root->fault : "the root task did not finish");
    val r = root->result;
    root->result = NULL;
    task_end(root);
    task_unref(root);
    sched_flush();
    exec_reap();
    return r;
}
// Admission is one linearization point: a child joins its parent's kid list
// and its executor's queue under the same lock.
static val task_start(kgexec* e, val fut, int mobile) {
    kgtask* t = calloc(1, sizeof *t);
    if (!t) rt_panic("out of memory");
    t->fut = fut;
    t->ex = e;
    KG_ASTORE(t->refs, 2);
    SCHED_LOCK();
    kgtask* p = g_cur_task;
    t->parent = p;
    t->pex = p->ex;
    KG_ASTORE(t->cancel, KG_ALOAD(p->cancel));
    t->lane = -1;
#ifdef KIGUMI_THREADS
    if (!e->nworkers || !mobile) t->lane = e == p->ex && e->nworkers ? g_cur_lane : 0;
#else
    (void)mobile;
    t->lane = 0;
#endif
    if (p->nkids == p->capkids) {
        p->capkids = p->capkids ? p->capkids * 2 : 4;
        p->kids = realloc(p->kids, (size_t)p->capkids * sizeof(kgtask*));
        if (!p->kids) rt_fatal("out of memory");
    }
    t->kid_at = p->nkids;
    p->kids[p->nkids++] = t;
    p->open_kids++;
    if (e != p->ex) p->ex->external++;
    e->live++;
    exec_push(e, t);
    SCHED_UNLOCK();
    sched_flush();
    return opaque("Task", t);
}
static void task_check_open(void) {
    if (g_cur_task->closing) rt_panic("a task cannot spawn a child once its scope is closing; spawn it before the task returns, fails or is cancelled");
}
val rt_task_spawn(val fut) {
    if (!g_cur_task) rt_panic("`task.spawn` needs a running task; start one with `Executor.runToCompletion`");
    task_check_open();
    return task_start(g_cur_task->ex, fut, future_mobile(fut));
}
val rt_task_handoff(val exv, val fut) {
    if (!g_cur_task) rt_panic("`Executor.handoff` needs a running task; start one with `Executor.runToCompletion`");
    task_check_open();
    kgexec* e = (kgexec*)deref(exv)->u.op.data;
    if (!future_mobile(fut)) rt_panic("this future is bound to its own executor and cannot be handed to another one");
    SCHED_LOCK();
#ifdef KIGUMI_THREADS
    int agent = e->nworkers || e->driving;
#else
    int agent = e->driving;
#endif
    SCHED_UNLOCK();
    if (e != g_cur_task->ex && !agent) rt_panic("this executor has no worker and is not running a task tree, so it cannot take a handed-off task");
    return task_start(e, fut, 1);
}
void rt_task_cancel(val tv) {
    SCHED_LOCK();
    task_request_cancel((kgtask*)deref(tv)->u.op.data);
    SCHED_UNLOCK();
    sched_flush();
}
int rt_cancel_requested(void) { return g_cur_task && KG_ALOAD(g_cur_task->cancel); }
// Only descendants run here: they are all cancelled, so none waits on a task outside the closing scope.
static int task_descends(kgtask* d, kgtask* t, unsigned long mark) {
    for (kgtask* p = d->parent; p; p = p->parent)
        if (p == t || p->close_mark == mark) {
            for (kgtask* q = d; q != p; q = q->parent) q->close_mark = mark;
            return 1;
        }
    return 0;
}
static unsigned long g_close_gen;
static kgtask* q_take_desc(kgqueue* k, kgtask* t, unsigned long mark) {
    for (int i = 0; i < k->len; i++) {
        kgtask* d = k->q[k->head + i];
        if (!task_descends(d, t, mark)) continue;
        if (i == 0)
            k->head++;
        else
            memmove(&k->q[k->head + i], &k->q[k->head + i + 1], (size_t)(k->len - i - 1) * sizeof(kgtask*));
        if (--k->len == 0) k->head = 0;
        d->queued = 0;
        return d;
    }
    return NULL;
}
// Deeper scopes than this join their children after their own cleanups, as the VM does.
#define KG_CLOSE_NESTING 64
static KG_TLS int g_close_depth;
// Cancels the children still running and drives them to a terminal state before t's own cleanups run.
static void task_close_scope(kgtask* t) {
    SCHED_LOCK();
    if (t->closing) {
        SCHED_UNLOCK();
        return;
    }
    t->closing = 1;
    task_cancel_kids(t);
    if (g_close_depth >= KG_CLOSE_NESTING) {
        SCHED_UNLOCK();
        sched_flush();
        return;
    }
    g_close_depth++;
    unsigned long mark = ++g_close_gen;
    for (;;) {
        if (!t->open_kids) break;
        kgexec* e = t->ex;
        kgtask* d = q_take_desc(&e->lane[g_cur_lane], t, mark);
        if (!d) d = q_take_desc(&e->pool, t, mark);
        if (!d) {
#ifdef KIGUMI_THREADS
            if (e->nworkers || e->external) {
                pthread_cond_wait(&e->cv, &g_sched);
                continue;
            }
#endif
            g_close_depth--;
            SCHED_UNLOCK();
            sched_flush();
            rt_panic("every task on this executor is waiting for something that cannot happen");
        }
        e->running++;
        SCHED_UNLOCK();
        sched_flush();
        task_run(d, g_cur_lane);
        SCHED_LOCK();
        e->running--;
#ifdef KIGUMI_THREADS
        pthread_cond_broadcast(&e->cv);
#endif
    }
    g_close_depth--;
    SCHED_UNLOCK();
    sched_flush();
}
#ifdef KG_CATCH
// A stack overflow leaves no room to run the children, so they close after the unwinding instead.
static void task_close_on_panic(void) {
    if (g_fault_msg && !strncmp(g_fault_msg, "stack overflow", 14)) return;
    char* saved = g_fault_msg;
    g_fault_msg = NULL;
    task_close_scope(g_cur_task);
    g_fault_msg = saved;
}
#endif
static val poll_yield(int* st) {
    if (!*st && g_cur_task) {
        *st = 1;
        SCHED_LOCK();
        g_cur_task->notified = 1;
        SCHED_UNLOCK();
        return NULL;
    }
    *st = 1;
    return rt_unit();
}
val rt_task_yield_now(void) {
    int* st = calloc(1, sizeof *st);
    if (!st) rt_panic("out of memory");
    return opaque("Yield", st);
}
