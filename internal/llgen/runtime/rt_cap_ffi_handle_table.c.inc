// handle_slot/handle_table back std/ffi's HandleTable[T]: like Pin, a slot
// holds a boxed value, but also a generation a stale or forged RawHandle must match.
typedef struct {
    int64_t generation;
    int occupied;
    val value;
} handle_slot;
typedef struct {
    handle_slot* slots;
    int64_t len, cap;
} handle_table;
// Table identity is this registry's permanent 1-based index, never reused;
// the heap pointer itself can be, once freed, handed back by a later calloc.
static handle_table** g_handle_tables;
static int64_t g_handle_tables_len, g_handle_tables_cap;
// The registry array is the one piece of HandleTable state a second worker
// can grow under a reader, so it has its own lock.
static int64_t handle_table_register(handle_table* t) {
#ifdef KIGUMI_THREADS
    static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
    pthread_mutex_lock(&mu);
#endif
    if (g_handle_tables_len == g_handle_tables_cap) {
        int64_t newcap = g_handle_tables_cap ? g_handle_tables_cap * 2 : 4;
        handle_table** ns = realloc(g_handle_tables, (size_t)newcap * sizeof *ns);
        if (!ns) rt_fatal("out of memory");
        g_handle_tables = ns;
        g_handle_tables_cap = newcap;
    }
    g_handle_tables[g_handle_tables_len] = t;
    int64_t id = ++g_handle_tables_len;
#ifdef KIGUMI_THREADS
    pthread_mutex_unlock(&mu);
#endif
    return id;
}

static int64_t handle_pack(int64_t generation, int64_t index) {
    return (generation << 32) | (index & 0xFFFFFFFF);
}
static void handle_unpack(int64_t raw, int64_t* generation, int64_t* index) {
    *generation = raw >> 32;
    *index = raw & 0xFFFFFFFF;
}
// Only an unsafe reclaim of a forged/stale RawHandle can fail this check;
// a live FfiHandle otherwise always names a live slot.
static int64_t handle_find(val h, handle_table** out) {
    handle_table* t = g_handle_tables[h->u.rec.fields[0]->u.i - 1];
    int64_t generation, index;
    handle_unpack(h->u.rec.fields[1]->u.i, &generation, &index);
    if (index < 0 || index >= t->len || !t->slots[index].occupied || t->slots[index].generation != generation) {
        rt_panic("this handle does not address a live slot of this table");
    }
    *out = t;
    return index;
}
static val handle_free(handle_table* t, int64_t index) {
    val v = t->slots[index].value;
    t->slots[index].occupied = 0;
    t->slots[index].value = NULL;
    t->slots[index].generation++;
    return v;
}
static val handle_fault(const char* variant) {
    rt_vdesc* d = find_variant(find_type("HandleFault"), variant);
    if (!d) return err_msg("handle fault");
    return err_val(rt_variant(d, 0, NULL));
}
static val cap_handle_table_new(void) {
    handle_table* t = calloc(1, sizeof *t);
    if (!t) rt_panic("out of memory");
    val h = rt_int(handle_table_register(t), 64 | 256);
    return make_record("HandleTable", 1, &h);
}
static val cap_handle_table_insert(val a0, val a1) {
    int64_t ti = a0->u.rec.fields[0]->u.i;
    handle_table* t = g_handle_tables[ti - 1];
    int64_t index = -1;
    for (int64_t i = 0; i < t->len; i++) {
        if (!t->slots[i].occupied) {
            index = i;
            break;
        }
    }
    if (index < 0) {
        if (t->len == t->cap) {
            int64_t newcap = t->cap ? t->cap * 2 : 4;
            handle_slot* ns = realloc(t->slots, (size_t)newcap * sizeof *ns);
            if (!ns) rt_panic("out of memory");
            for (int64_t i = t->cap; i < newcap; i++) ns[i] = (handle_slot){0};
            t->slots = ns;
            t->cap = newcap;
        }
        index = t->len++;
    }
    t->slots[index].occupied = 1;
    t->slots[index].value = rt_retain(a1);
    val f[2] = {rt_int(ti, 64 | 256), rt_int(handle_pack(t->slots[index].generation, index), 64 | 256)};
    return make_record("FfiHandle", 2, f);
}
static val cap_handle_table_with(val a1, val a2) {
    handle_table* t;
    int64_t index = handle_find(a1, &t);
    val arg[1] = {t->slots[index].value};
    return rt_callv(a2, 1, arg);
}
static val cap_handle_table_remove(val a1) {
    handle_table* t;
    int64_t index = handle_find(a1, &t);
    val v = handle_free(t, index);
    // h is only lent, so the caller's scope-exit FfiHandle.drop still
    // runs; replace the id (never mutate it: literals are immortal).
    rt_setfield(a1, 0, rt_int(0, 64 | 256));
    return v;
}
static val cap_handle_table_reclaim(val a0, val a1) {
    int64_t self_tbl = a0->u.rec.fields[0]->u.i;
    int64_t raw_tbl = a1->u.rec.fields[0]->u.i, raw_slot = a1->u.rec.fields[1]->u.i;
    if (raw_tbl != self_tbl) return handle_fault("WrongTable");
    handle_table* t = g_handle_tables[self_tbl - 1];
    int64_t generation, index;
    handle_unpack(raw_slot, &generation, &index);
    if (index < 0 || index >= t->len) return handle_fault("OutOfRange");
    if (!t->slots[index].occupied || t->slots[index].generation != generation) return handle_fault("Stale");
    val f[2] = {rt_int(self_tbl, 64 | 256), rt_int(raw_slot, 64 | 256)};
    return ok(make_record("FfiHandle", 2, f));
}
static val cap_handle_table_close(val a0) {
    handle_table* t = g_handle_tables[a0->u.rec.fields[0]->u.i - 1];
    int64_t live = 0;
    for (int64_t i = 0; i < t->len; i++) {
        if (t->slots[i].occupied) live++;
    }
    if (live > 0) {
        val n = rt_int(live, 64 | 256);
        return err_val(make_record("HandleTableBusy", 1, &n));
    }
    return ok(rt_unit());
}
static val cap_handle_table_drop(val a0) {
    int64_t ti = a0->u.rec.fields[0]->u.i;
    handle_table* t = g_handle_tables[ti - 1];
    for (int64_t i = 0; i < t->len; i++) {
        if (t->slots[i].occupied) rt_panic("dropped a HandleTable with a handle still outstanding; call close() first");
    }
    free(t->slots);
    free(t);
    g_handle_tables[ti - 1] = NULL;
    return rt_unit();
}
static val cap_handle_raw(val a0) {
    val f[2] = {rt_int(a0->u.rec.fields[0]->u.i, 64 | 256), rt_int(a0->u.rec.fields[1]->u.i, 64 | 256)};
    return make_record("RawHandle", 2, f);
}
static val cap_handle_drop(val a0) {
    if (a0->u.rec.fields[0]->u.i == 0) return rt_unit();
    handle_table* t;
    int64_t index = handle_find(a0, &t);
    rt_release(handle_free(t, index));
    return rt_unit();
}
