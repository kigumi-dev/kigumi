static val cap_pin_new(val a) {
    val v = rt_retain(rt_deref(a));
    val h = rt_int((int64_t)(intptr_t)v, 64 | 256);
    return rt_make_record("Pin", 1, &h);
}
static val cap_pin_ptr(val a0) {
    return rt_ptr((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
}
static val cap_pin_with(val a0, val a1) {
    val arg[1] = {(val)a0->u.p};
    return rt_callv_lent(a1, 1, arg);
}
static val cap_pin_drop(val a0) {
    // The zero guard makes a repeated drop a no-op.
    int64_t* hf = &a0->u.rec.fields[0]->u.i;
    if (*hf) {
        rt_release((val)(intptr_t)*hf);
        *hf = 0;
    }
    return rt_unit();
}
static val cap_pin_release(val a0) {
    // release lends self, so the caller's drop still runs; the retain keeps the pointer alive through it.
    rt_retain((val)(intptr_t)a0->u.rec.fields[0]->u.i);
    return rt_ptr((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
}
static val cap_pin_reclaim(val a0) {
    val h = rt_int((int64_t)(intptr_t)a0->u.p, 64 | 256);
    return rt_make_record("Pin", 1, &h);
}
