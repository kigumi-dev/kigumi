static val cap_host_args(void) {
    return rt_opaque("Args", NULL);
}
static val cap_args_get(val a1) {
    int64_t i = a1->u.i;
    if (i < 0 || i >= g_argc) return rt_none();
    int64_t len = (int64_t)strlen(g_argv[i]);
    if (!rt_utf8_valid(g_argv[i], len)) return rt_none();
    return rt_some(rt_str(g_argv[i], len));
}
static val cap_args_get_bytes(val a1) {
    int64_t i = a1->u.i;
    if (i < 0 || i >= g_argc) return rt_none();
    return rt_some(bytes_val(g_argv[i], (int64_t)strlen(g_argv[i])));
}
static val cap_args_len(void) {
    return rt_int(g_argc, 64);
}
