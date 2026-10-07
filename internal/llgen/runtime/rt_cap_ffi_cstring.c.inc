static val cap_cstring_new(val a0) {
    // Every String/Bytes buffer already carries a guaranteed trailing
    // NUL at data[len] (see strdup_val); strchr for the first NUL byte
    // reaches that one unless an earlier, embedded NUL is hit first.
    if (strchr(a0->u.s.data, 0) != a0->u.s.data + a0->u.s.len) return err_msg("text contains a NUL byte");
    val h = rt_int((int64_t)(intptr_t)rt_cstr(a0), 64 | 256);
    return ok(make_record("CString", 1, &h));
}
static val cap_cstring_ptr(val a0) {
    return rt_ptr((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
}
static val cap_cstring_drop(val a0) {
    free((void*)(intptr_t)a0->u.rec.fields[0]->u.i);
    return rt_unit();
}
