static int task_terminal(kgtask* t) { return t->state >= T_COMPLETED; }
static void q_push(kgqueue* k, kgtask* t) {
    if (k->head + k->len == k->cap) {
        for (int i = 0; i < k->len; i++) k->q[i] = k->q[k->head + i];
        k->head = 0;
        if (k->len == k->cap) {
            k->cap = k->cap ? k->cap * 2 : 8;
            k->q = realloc(k->q, (size_t)k->cap * sizeof(kgtask*));
            if (!k->q) rt_panic("out of memory");
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
static void task_request_cancel(kgtask* t) {
    if (task_terminal(t) || KG_ALOAD(t->cancel)) return;
    KG_ASTORE(t->cancel, 1);
    for (int i = 0; i < t->nkids; i++) task_request_cancel(t->kids[i]);
    task_wake(t);
}
static void task_finish_drain(kgtask* t) {
    for (int i = 0; i < t->nkids; i++)
        if (!task_terminal(t->kids[i])) {
            t->state = T_SUSPENDED;
            return;
        }
    t->state = t->ending;
    t->draining = 0;
    t->ex->live--;
#ifdef KIGUMI_THREADS
    if (exec_finished(t->ex)) pthread_cond_broadcast(&t->ex->cv);
#endif
    for (int i = 0; i < t->nkids; i++) {
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
    if (t->parent && t->parent->draining) task_wake(t->parent);
}
// task_begin_finish cancels still-running kids; the terminal transition waits for all of them.
static void task_begin_finish(kgtask* t, int end) {
    t->ending = end;
    t->draining = 1;
    for (int i = 0; i < t->nkids; i++) task_request_cancel(t->kids[i]);
    task_finish_drain(t);
}
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
    val r = poll_value(t->fut);
    g_cur_task = prev;
    g_cur_lane = prevlane;
    SCHED_LOCK();
    if (!r) {
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
    SCHED_UNLOCK();
    if (st == T_CANCELLED) return KG_CANCELLED;
    if (st == T_FAULTED) rt_panic("the awaited task did not finish");
    return r;
}
#ifdef KIGUMI_THREADS
typedef struct kgworker {
    kgexec* e;
    int lane;
} kgworker;
static void* worker_main(void* p) {
    kgworker* w = (kgworker*)p;
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
static void exec_join(kgexec* e) {
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
                if (!o) rt_panic("out of memory");
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
    if (root->state == T_CANCELLED) rt_panic("the root task was cancelled");
    if (root->state == T_FAULTED) rt_panic("the root task did not finish");
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
        if (!p->kids) rt_panic("out of memory");
    }
    p->kids[p->nkids++] = t;
    if (e != p->ex) p->ex->external++;
    e->live++;
    exec_push(e, t);
    SCHED_UNLOCK();
    sched_flush();
    return opaque("Task", t);
}
val rt_task_spawn(val fut) {
    if (!g_cur_task) rt_panic("`task.spawn` needs a running task; start one with `Executor.runToCompletion`");
    return task_start(g_cur_task->ex, fut, future_mobile(fut));
}
val rt_task_handoff(val exv, val fut) {
    if (!g_cur_task) rt_panic("`Executor.handoff` needs a running task; start one with `Executor.runToCompletion`");
    kgexec* e = (kgexec*)deref(exv)->u.op.data;
    if (!future_mobile(fut)) rt_panic("this future is bound to its own executor and cannot be handed to another one");
#ifdef KIGUMI_THREADS
    int agent = e->nworkers || e->driving;
#else
    int agent = e->driving;
#endif
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
