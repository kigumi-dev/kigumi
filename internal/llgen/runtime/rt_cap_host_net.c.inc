static val cap_host_net(void) {
    return opaque("Net", NULL);
}
static val cap_net_http(void) {
    return opaque("Http", NULL);
}
// Bodiless like Conn.drop/Listener.drop themselves (see net_hosted.kg):
// an accelerator on a *bodied* drop only replaces normal calls to it,
// not the implicit drop on a resource's last reference going, which
// always runs the real (here nonexistent) lowered body instead.
static val cap_net_conn_drop(val a0) {
    rt_sys_close((int)a0->u.rec.fields[0]->u.i);
    return rt_unit();
}
