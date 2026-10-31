// p is volatile, so the standard forbids eliding these stores as dead
// even though nothing reads p afterward.
void rt_secure_zero(volatile unsigned char* p, int64_t n) {
    for (int64_t i = 0; i < n; i++) p[i] = 0;
}
static val cap_secure_erase(val a0) {
    int64_t n = a0->u.s.len;
    if (n <= 0) return rt_unit();
    if (a0->owned) {
        rt_secure_zero((volatile unsigned char*)a0->u.s.data, n);
        return rt_unit();
    }
    // a literal/foreign Bytes has no writable backing of its own; give
    // self a fresh owned one instead of writing through static memory.
    char* d = kg_calloc_n(n);
    if (!d) rt_panic("out of memory");
    a0->u.s.data = d;
    a0->owned = 1;
    return rt_unit();
}
static val cap_secure_erase_raw(val a0, val a1) {
    int64_t n = a1->u.i;
    if (n > 0) rt_secure_zero((volatile unsigned char*)a0->u.p, n);
    return rt_unit();
}
static val cap_secure_erase_try(val a0) {
    int64_t n = a0->u.s.len;
    if (n <= 0) return try_ok(rt_unit());
    if (a0->owned) {
        rt_secure_zero((volatile unsigned char*)a0->u.s.data, n);
        return try_ok(rt_unit());
    }
    char* d = fault_tick() ? NULL : kg_calloc_n(n);
    if (!d) return alloc_error();
    a0->u.s.data = d;
    a0->owned = 1;
    return try_ok(rt_unit());
}
