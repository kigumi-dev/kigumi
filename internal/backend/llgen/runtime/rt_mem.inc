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
