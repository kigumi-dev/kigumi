static val cap_host_stdin(void) {
    return rt_opaque("Stdin", NULL);
}
static val cap_host_stdout(void) {
    return rt_opaque("Stdout", NULL);
}
static val cap_host_stderr(void) {
    return rt_opaque("Stderr", NULL);
}
