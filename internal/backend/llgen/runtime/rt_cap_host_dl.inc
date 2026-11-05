static val cap_host_dl(void) {
    return opaque("Loader", NULL);
}
static val cap_dl_symbol_call(void) {
    rt_panic("dl.Symbol.call is lowered by the compiler");
    return NULL;
}
