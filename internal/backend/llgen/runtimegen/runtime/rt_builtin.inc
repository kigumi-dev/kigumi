val rt_builtin(const char* name, int n, val* args) {
    if (!strcmp(name, "print")) {
        print_line(1, args[0]);
        return rt_unit();
    }
    if (!strcmp(name, "eprint")) {
        print_line(2, args[0]);
        fault_arm();
        return rt_unit();
    }
    if (!strcmp(name, "panic")) rt_panic(n > 0 ? rt_cstr(args[0]) : "explicit panic");
    if (!strcmp(name, "host")) return rt_opaque("Host", NULL);
    if (!strcmp(name, "cmp.unordered")) {
        val w = rt_deref(args[0]), a = rt_deref(args[1]), b = rt_deref(args[2]);
        const char* k = w->kind == K_CLOSURE && w->u.clo.key ? w->u.clo.key : "";
        if (!strncmp(k, "std/", 4)) k += 4;
        int fl = !strcmp(k, "prelude.f64.equals") || !strcmp(k, "prelude.f64.compareTo") || !strcmp(k, "prelude.f32.equals") || !strcmp(k, "prelude.f32.compareTo");
        return rt_bool(fl && ((a->kind == K_FLOAT && a->u.f != a->u.f) || (b->kind == K_FLOAT && b->u.f != b->u.f)));
    }
    if (!strcmp(name, "cmp.guard")) return rt_bool(rt_truth(args[0]) && !rt_truth(args[1]));
    if (!strcmp(name, "fixed.fill")) return fixed_fill(args[0], rt_deref(args[1])->u.i);
    if (!strcmp(name, "iter.len")) return iter_len(args[0]);
    if (!strcmp(name, "iter.at")) return rt_iter_at(args[0], rt_deref(args[1])->u.i, 0);
    if (!strcmp(name, "iter.at_ref")) return rt_iter_at(args[0], rt_deref(args[1])->u.i, 1);
    if (!strcmp(name, "iter.at_move")) return rt_iter_at(args[0], rt_deref(args[1])->u.i, 2);
    if (!strcmp(name, "alloc.push")) {
        val scope = rt_deref(args[0]);
        if (g_alloc_sp + 1 >= 64) rt_panic("allocator blocks nested too deeply");
        arena* ar = (arena*)rt_deref((val)scope->u.op.data)->u.op.data;
        if (ar->a.closed) rt_panic("arena is closed");
        g_alloc_stack[++g_alloc_sp] = &ar->a;
        return opaque_in(&((arena*)((val)scope->u.op.data)->u.op.data)->a, "AllocGuard", rt_retain(scope));
    }
    rt_panicf("unknown builtin %s", name);
    return NULL;
}
