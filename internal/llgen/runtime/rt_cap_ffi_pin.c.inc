static val cap_pin_new(val a) {
    val v = rt_retain(deref(a));
    val h = rt_int((int64_t)(intptr_t)v, 64 | 256);
    return make_record("Pin", 1, &h);
}
static val cap_pin_ptr(val a0) {
    return rt_ptr((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
}
static val cap_pin_with(val a0, val a1) {
    val arg[1] = {(val)a0->u.p};
    return rt_callv(a1, 1, arg);
}
static val cap_pin_drop(val a0) {
    // mir now consumes self for a type's own drop (isOwnDrop in
    // internal/mir/plan_args.go), so scope exit no longer re-enters
    // here; the zero-guard just stays as a defensive no-op on repeat.
    int64_t* hf = &a0->u.rec.fields[0]->u.i;
    if (*hf) {
        rt_release((val)(intptr_t)*hf);
        *hf = 0;
    }
    return rt_unit();
}
static val cap_pin_release(val a0) {
    // release() (unlike drop) still lends self, so the caller's
    // scope-exit drop runs after this; retain here keeps the
    // extracted pointer alive through that drop.
    rt_retain((val)(intptr_t)a0->u.rec.fields[0]->u.i);
    return rt_ptr((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
}
static val cap_pin_reclaim(val a0) {
    val h = rt_int((int64_t)(intptr_t)a0->u.p, 64 | 256);
    return make_record("Pin", 1, &h);
}
