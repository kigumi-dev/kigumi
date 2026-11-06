// blocking_pool is a BlockingPool: a root-owned, bounded count of BlockingFuture results admitted but not taken.
// in_flight is atomic: a future may be taken or dropped on any lane.
typedef struct blocking_pool {
    int capacity;
    KG_AINT in_flight;
} blocking_pool;
#if defined(KIGUMI_THREADS) && defined(KIGUMI_CAP_TASK_EXECUTOR)
#define KG_BLOCKING_THREADS 1
// blocking_job is one closure running on a thread of its own; every field but thread belongs to the scheduler lock.
typedef struct blocking_job {
    pthread_t thread;
    val work, result;
    char* fault;
    int done;
    kgtask* waiter;
    // wait_ex holds an external count of the awaiting task's executor while the job is pending, so its driver does not call the tree stuck.
    kgexec* wait_ex;
} blocking_job;
#endif
// blocking_future is one admitted BlockingPool.run result; pool is
// retained so the pool outlives every future it issued.
typedef struct blocking_future {
    val pool, result;
    int taken;
#ifdef KG_BLOCKING_THREADS
    blocking_job* job;
#endif
} blocking_future;
static int blocking_reserve(blocking_pool* p) {
#ifdef KIGUMI_THREADS
    int cur = KG_ALOAD(p->in_flight);
    while (cur < p->capacity) {
        if (atomic_compare_exchange_weak_explicit(&p->in_flight, &cur, cur + 1,
                memory_order_acq_rel, memory_order_acquire)) return 1;
    }
    return 0;
#else
    if (p->in_flight >= p->capacity) return 0;
    p->in_flight++;
    return 1;
#endif
}
static val blocking_capacity_exhausted(void) {
    rt_vdesc* d = find_variant(rt_find_type("BlockingError"), "CapacityExhausted");
    if (!d) return rt_err_msg("blocking pool capacity exhausted");
    return rt_err_val(rt_variant(d, 0, NULL));
}
#ifdef KG_BLOCKING_THREADS
static KG_TLS val g_job_result;
static void blocking_job_call(val work) {
    g_job_result = rt_callv(work, 0, NULL);
}
static void* blocking_job_main(void* p) {
    blocking_job* j = p;
    rt_sys_stack_guard();
    g_job_result = NULL;
#ifdef KG_CATCH
    int ok = contained(blocking_job_call, j->work);
#else
    int ok = 1;
    blocking_job_call(j->work);
#endif
    char* fault = NULL;
    if (!ok) {
        fault = g_fault_msg;
        g_fault_msg = NULL;
    }
    val result = g_job_result;
    rt_release(j->work);
    j->work = NULL;
    SCHED_LOCK();
    j->result = result;
    j->fault = fault;
    j->done = 1;
    if (j->wait_ex) {
        j->wait_ex->external--;
        pthread_cond_broadcast(&j->wait_ex->cv);
        j->wait_ex = NULL;
    }
    if (j->waiter) {
        kgtask* w = j->waiter;
        j->waiter = NULL;
        task_wake(w);
    }
    SCHED_UNLOCK();
    sched_flush();
    return NULL;
}
static blocking_job* blocking_job_start(val work) {
    blocking_job* j = calloc(1, sizeof *j);
    if (!j) rt_panic("out of memory");
    j->work = rt_retain(work);
    if (pthread_create(&j->thread, NULL, blocking_job_main, j)) {
        rt_release(j->work);
        free(j);
        rt_panic("could not start a blocking worker thread");
    }
    return j;
}
// Returns NULL while the closure still runs, with the awaiting task registered to be woken.
static val blocking_job_poll(blocking_future* f) {
    blocking_job* j = f->job;
    SCHED_LOCK();
    if (!j->done) {
        if (!j->wait_ex && g_cur_task) {
            j->wait_ex = g_cur_task->ex;
            j->wait_ex->external++;
        }
        j->waiter = g_cur_task;
        SCHED_UNLOCK();
        return NULL;
    }
    SCHED_UNLOCK();
    pthread_join(j->thread, NULL);
    f->job = NULL;
    val r = j->result;
    char* fault = j->fault;
    free(j);
    if (fault) {
        static KG_TLS char msg[512];
        strncpy(msg, fault, sizeof msg - 1);
        free(fault);
        rt_panic(msg);
    }
    return r;
}
// Blocking work cannot be interrupted, and its captures must not be freed under it, so dropping the future waits for it.
static void blocking_job_discard(blocking_future* f) {
    blocking_job* j = f->job;
    f->job = NULL;
    pthread_join(j->thread, NULL);
    rt_release(j->result);
    if (j->fault) {
        char* fault = j->fault;
        j->fault = NULL;
        rt_panic(fault);
    }
    free(j);
}
#endif
static val blocking_future_join(blocking_future* f) {
#ifdef KG_BLOCKING_THREADS
    if (f->job) {
        val r = blocking_job_poll(f);
        if (!r) return NULL;
        f->result = r;
    }
#endif
    f->taken = 1;
    KG_ADD(((blocking_pool*)f->pool->u.op.data)->in_flight, -1);
    return f->result;
}
static void blocking_future_free(blocking_future* f) {
    if (!f->taken) {
        KG_ADD(((blocking_pool*)f->pool->u.op.data)->in_flight, -1);
#ifdef KG_BLOCKING_THREADS
        if (f->job) blocking_job_discard(f);
#endif
        rt_release(f->result);
    }
    rt_release(f->pool);
    free(f);
}
static val cap_blocking_pool_new(val a0) {
    blocking_pool* p = calloc(1, sizeof *p);
    if (!p) rt_panic("out of memory");
    p->capacity = (int)a0->u.i;
    return rt_opaque("BlockingPool", p);
}
// Under a thread-pool executor the closure runs on a thread of its own; under a single-thread executor, or with none, it runs here on the caller.
static val cap_blocking_pool_run(val a0, val a1) {
    blocking_pool* p = (blocking_pool*)a0->u.op.data;
    blocking_future* f = calloc(1, sizeof *f);
    if (!f) rt_panic("out of memory");
    if (!blocking_reserve(p)) {
        free(f);
        return blocking_capacity_exhausted();
    }
#ifdef KG_BLOCKING_THREADS
    if (g_cur_task && g_cur_task->ex->nworkers > 0) {
        f->job = blocking_job_start(a1);
    } else
#endif
    {
        f->result = rt_callv(a1, 0, NULL);
    }
    f->pool = rt_retain(a0);
    return rt_ok(rt_opaque("BlockingFuture", f));
}
