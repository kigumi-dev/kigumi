// Children are released from an explicit stack, in the order a recursive release would visit them, so any ownership depth drops in constant C stack.
static void free_obj_mode(val root, int skip_hook) {
    dstack st;
    st.f = st.inl;
    st.n = 0;
    st.cap = DSTACK_INL;
    st.base = DEPTH_NOW();
    drain_open(&st, root, 0, skip_hook);
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
        drain_open(&st, c, pend, 0);
    }
    drain_discard(&st);
}

static void free_obj(val root) { free_obj_mode(root, 0); }

void rt_drop_partial(val v) {
    if (v != NULL && release_dec(v)) free_obj_mode(v, 1);
}

int rt_release_outer_partial(val v) {
#ifdef KG_CATCH
    if (g_task_catch) {
        rt_drop_partial(v);
        return 0;
    }
    uint64_t seen = g_fault_count;
    g_outer_drop++;
    rt_drop_partial(v);
    g_outer_drop--;
    return g_fault_count != seen;
#else
    rt_drop_partial(v);
    return 0;
#endif
}

void rt_drop_partial_outer(val v) {
    if (rt_release_outer_partial(v)) rt_outer_fatal();
}
