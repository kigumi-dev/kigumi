static int record_partial(val root) {
    if (root->kind != K_RECORD && root->kind != K_VARIANT) return 0;
    dstack st;
    st.f = st.inl;
    st.n = 0;
    st.cap = DSTACK_INL;
    dframe fr = {.v = root, .i = root->kind == K_RECORD ? root->u.rec.n - 1 : 0};
    drain_push(&st, &fr);
    while (st.n > 0) {
        val* slot = child_slot(&st.f[st.n - 1], 1);
        if (!slot) {
            st.n--;
            continue;
        }
        val v = *slot;
        if (!v) {
            drain_discard(&st);
            return 1;
        }
        if (v->kind == K_RECORD || v->kind == K_VARIANT) {
            dframe child = {.v = v, .i = v->kind == K_RECORD ? v->u.rec.n - 1 : 0};
            if (!drain_push(&st, &child)) rt_fatal("out of memory");
        }
    }
    drain_discard(&st);
    return 0;
}

// Children are released from an explicit stack, in the order a recursive release would visit them, so any ownership depth drops in constant C stack.
static void free_obj_mode(val root, int partial) {
    dstack st;
    st.f = st.inl;
    st.n = 0;
    st.cap = DSTACK_INL;
    st.base = DEPTH_NOW();
    drain_open(&st, root, 0, partial && record_partial(root));
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
        drain_open(&st, c, pend, partial && record_partial(c));
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
