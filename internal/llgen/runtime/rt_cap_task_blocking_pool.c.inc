// blocking_pool is a BlockingPool: a root-owned, bounded count of BlockingFuture results admitted but not taken.
// run executes work on the calling thread, not a separate worker, so in_flight here does not need to be atomic.
typedef struct blocking_pool {
    int capacity, in_flight;
} blocking_pool;
// blocking_future is one admitted BlockingPool.run result; pool is
// retained so the pool outlives every future it issued.
typedef struct blocking_future {
    val pool, result;
    int taken;
} blocking_future;
static val blocking_capacity_exhausted(void) {
    rt_vdesc* d = find_variant(find_type("BlockingError"), "CapacityExhausted");
    if (!d) return err_msg("blocking pool capacity exhausted");
    return err_val(rt_variant(d, 0, NULL));
}
static val blocking_future_join(blocking_future* f) {
    f->taken = 1;
    ((blocking_pool*)f->pool->u.op.data)->in_flight--;
    return f->result;
}
static val cap_blocking_pool_new(val a0) {
    blocking_pool* p = calloc(1, sizeof *p);
    if (!p) rt_panic("out of memory");
    p->capacity = (int)a0->u.i;
    return opaque("BlockingPool", p);
}
static val cap_blocking_pool_run(val a0, val a1) {
    blocking_pool* p = (blocking_pool*)a0->u.op.data;
    if (p->in_flight >= p->capacity) return blocking_capacity_exhausted();
    val r = rt_callv(a1, 0, NULL);
    blocking_future* f = calloc(1, sizeof *f);
    if (!f) {
        rt_release(r);
        rt_panic("out of memory");
    }
    p->in_flight++;
    f->pool = rt_retain(a0);
    f->result = r;
    return ok(opaque("BlockingFuture", f));
}
