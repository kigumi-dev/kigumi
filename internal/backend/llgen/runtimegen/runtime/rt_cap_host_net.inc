static val cap_host_net(void) {
    return rt_opaque("Net", NULL);
}
// Bodiless, since an accelerator on a bodied drop would not replace the implicit drop at a resource's last reference.
static val cap_net_conn_drop(val a0) {
    rt_sys_close((int)a0->u.rec.fields[0]->u.i);
    return rt_unit();
}
