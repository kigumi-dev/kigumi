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
