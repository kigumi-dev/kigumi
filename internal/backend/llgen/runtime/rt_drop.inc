// Children are released from an explicit stack, in the order a recursive release would visit them, so any ownership depth drops in constant C stack.
static void free_obj(val root) {
    dstack st;
    st.f = st.inl;
    st.n = 0;
    st.cap = DSTACK_INL;
    st.base = DEPTH_NOW();
    drain_open(&st, root, 0);
    while (st.n > 0) {
        dframe* fr = &st.f[st.n - 1];
        val* slot = child_slot(fr, 1);
        if (!slot) {
            int raise = drain_close(fr);
            st.n--;
            if (raise) raise_drop_fault(&st);
            continue;
        }
        val c = *slot;
        if (fr->guarded) *slot = NULL;
        if (!release_dec(c)) continue;
        int64_t pend = 0;
        if (!child_slot(fr, 0) && fr->opaque != OPAQUE_SHARED && !(fr->guarded && fr->outer)) {
            pend = fr->pend + fr->guarded;
            free_storage(fr->v);
            st.n--;
        }
        drain_open(&st, c, pend);
    }
    drain_discard(&st);
}
