static val cap_cstring_new(val a0) {
    // data[len] is always NUL, so the first NUL sits at len unless one is embedded.
    if (strchr(a0->u.s.data, 0) != a0->u.s.data + a0->u.s.len) return rt_err_msg("text contains a NUL byte");
    val h = rt_int((int64_t)(intptr_t)rt_cstr(a0), 64 | 256);
    return rt_ok(rt_make_record("CString", 1, &h));
}
static val cap_cstring_ptr(val a0) {
    return rt_ptr((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
}
static val cap_cstring_drop(val a0) {
    free((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
    return rt_unit();
}
