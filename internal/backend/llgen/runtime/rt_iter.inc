static rt_type* map_type(void) {
    static rt_type* t;
    static int looked;
    if (!looked) {
        t = rt_find_type("Map");
        looked = 1;
    }
    return t;
}
int64_t rt_iter_count(val it) {
    it = rt_deref(it);
    if (it->kind == K_ARRAY) return it->u.arr.len;
    if (it->kind == K_BYTES) return it->u.s.len;
    if (it->kind == K_RECORD && it->u.rec.type == map_type()) return it->u.rec.fields[0]->u.arr.len;
#ifdef KIGUMI_INT128
    if (W_IS(it->u.rec.fields[0]->nk)) return w_range_len(it->u.rec.fields[0], it->u.rec.fields[1], it->u.rec.fields[2]->u.b);
#endif
    int64_t lo = it->u.rec.fields[0]->u.i, hi = it->u.rec.fields[1]->u.i;
    int reversed = (it->u.rec.fields[0]->nk & 256) ? lo > hi : (uint64_t)lo > (uint64_t)hi;
    return reversed ? 0 : (int64_t)((uint64_t)hi - (uint64_t)lo + it->u.rec.fields[2]->u.b);
}
static val iter_len(val it) {
    return rt_int(rt_iter_count(it), 64);
}
/* A trivial scalar has no drop hook of its own: sharing it needs no
 * ownership transfer, only a retain. */
static int is_trivial_scalar(val v) {
    switch (v->kind) {
    case K_INT: case K_FLOAT: case K_BOOL: case K_STR: case K_BYTES:
    case K_CHAR: case K_UNIT:
        return 1;
    }
    return 0;
}
/* Takes ownership of *slot for the owned non-Copy case: a
 * trivial scalar (no drop of its own, and a generic loop body compiles
 * once for every T, so this also runs for a Copy instantiation such as
 * Array[T].fold with no `T: Copy` bound) is retained in place instead, so
 * a std loop re-reading the container mid-loop by index (Array.reverse)
 * still sees it. */
static val take_or_retain(val* slot) {
    if (is_trivial_scalar(*slot)) return rt_retain(*slot);
    val v = *slot;
    *slot = rt_unit();
    return v;
}
static val fixed_fill(val v, int64_t n) {
    val a = rt_array(0, NULL);
    for (int64_t i = 0; i < n; i++) rt_push(a, rt_copy(v));
    return a;
}
/* mode: 0 = iter.at (retains in place), 1 = iter.at_ref (borrowed, no
 * retain), 2 = iter.at_move (owned, transfers non-Copy elements). */
val rt_iter_at(val it, int64_t i, int mode) {
    it = rt_deref(it);
    if (it->kind == K_ARRAY) {
        if (mode == 1) return it->u.arr.items[i];
        if (mode == 2) return take_or_retain(&it->u.arr.items[i]);
        return rt_retain(it->u.arr.items[i]);
    }
    if (it->kind == K_BYTES) return rt_int((unsigned char)it->u.s.data[i], 8);
    if (it->kind == K_RECORD && it->u.rec.type == map_type()) {
        val keys = it->u.rec.fields[0], vals = it->u.rec.fields[1];
        rt_type* tup = rt_find_type("Tuple2");
        if (mode == 1) {
            val f[2] = {keys->u.arr.items[i], vals->u.arr.items[i]};
            return rt_record(tup, 2, f);
        }
        if (mode == 2) {
            val f[2] = {take_or_retain(&keys->u.arr.items[i]), take_or_retain(&vals->u.arr.items[i])};
            return rt_record(tup, 2, f);
        }
        val f[2] = {rt_retain(keys->u.arr.items[i]), rt_retain(vals->u.arr.items[i])};
        return rt_record(tup, 2, f);
    }
    val lo = it->u.rec.fields[0];
#ifdef KIGUMI_INT128
    if (W_IS(lo->nk)) return w_box(w_add(w_of(lo), w_new((uint64_t)i, 0)), lo->nk);
#endif
    return rt_int(lo->u.i + i, lo->nk);
}
// An integer element read straight into a register takes no box of its own.
int64_t rt_iter_at_int(val it, int64_t i) {
    it = rt_deref(it);
    if (it->kind == K_ARRAY) return rt_deref(it->u.arr.items[i])->u.i;
    if (it->kind == K_BYTES) return (unsigned char)it->u.s.data[i];
    return it->u.rec.fields[0]->u.i + i;
}
// print takes a String, so its bytes go out as they are and nothing is allocated.
static void print_line(int fd, val v) {
    v = rt_deref(v);
    if (v->kind == K_BOX) v = rt_deref(v->u.box.v);
    if (v->kind == K_STR && v->u.s.len >= 0) {
        write_line(fd, "", 0, v->u.s.data, v->u.s.len);
        return;
    }
    val s = rt_display(v);
    write_line(fd, "", 0, s->u.s.data, s->u.s.len);
    rt_release(s);
}
