#ifdef KG_CATCH
// Mask words and then slots follow the header.
typedef struct kg_uframe {
    struct kg_unwind* prev;
    void (*run)(struct kg_unwind*);
    int64_t n;
} kg_uframe;
static void unwind_slots(val* slots, uint64_t* mask, int64_t n) {
    for (int64_t k = n - 1; k >= 0; k--) {
        uint64_t* w = &mask[k / 64];
        uint64_t bit = (uint64_t)1 << (k % 64);
        if (!(*w & bit)) continue;
        *w &= ~bit;
        val v = slots[k];
        slots[k] = NULL;
        // Bit 0 marks an inline Option or Result: the next slot holds the helper that drops it in place.
        if ((uintptr_t)v & 1) {
            ((void (*)(void*))slots[k + 1])((void*)((uintptr_t)v & ~(uintptr_t)1));
            continue;
        }
        rt_release(v);
    }
}
// A release that panics leaves the other set bits for the nested fault to run.
static void unwind_frame(kg_unwind* u) {
    kg_uframe* f = (kg_uframe*)u;
    uint64_t* mask = (uint64_t*)(f + 1);
    unwind_slots((val*)(mask + (f->n + 63) / 64), mask, f->n);
}
static void unwind_frame_slots(kg_unwind* u) {
    unwind_slots((val*)u->p, u->m, u->i);
}
// The arguments rt_invoke still owes a release; u->i counts the ones left, so a release that panics resumes with the rest.
static void unwind_args(kg_unwind* u) {
    val* args = (val*)u->p;
    while (u->i > 0) rt_release(args[--u->i]);
}
// The same, for an argument region the call also owns; freed once every release has run.
static void unwind_args_region(kg_unwind* u) {
    unwind_args(u);
    gen_free(&g_general, u->p);
}
// The arguments of a std function value, released in order; u->i is the next one, so a release that panics resumes with the rest.
typedef struct {
    val* args;
    int64_t n;
    uint64_t lent;
    val* heap;
} kg_stdargs;
static void unwind_std_args(kg_unwind* u) {
    kg_stdargs* s = (kg_stdargs*)u->p;
    if (s->heap) gen_free(&g_general, s->heap);
    s->heap = NULL;
    while (u->i < s->n) {
        int64_t k = u->i++;
        if (k >= 64 || !((s->lent >> k) & 1)) rt_release(s->args[k]);
    }
}
#define IS_FRAME_REC(u) \
    ((u)->run == unwind_frame || (u)->run == unwind_frame_slots || (u)->run == unwind_args || (u)->run == unwind_args_region || (u)->run == unwind_std_args)
void rt_unwind_enter(void* rec, int64_t n) {
    kg_uframe* f = (kg_uframe*)rec;
    f->prev = g_unwind;
    f->run = unwind_frame;
    f->n = n;
    g_unwind = (kg_unwind*)f;
}
void rt_unwind_enter_slots(void* rec, void* slots, uint64_t* mask, int64_t n) {
    kg_unwind* u = (kg_unwind*)rec;
    u->prev = g_unwind;
    u->run = unwind_frame_slots;
    u->p = slots;
    u->v = NULL;
    u->i = n;
    u->m = mask;
    g_unwind = u;
}
// The record is a kg_unwind followed by the kg_stdargs it points at.
void rt_unwind_enter_std(void* rec, val* args, int64_t n, int64_t lent) {
    kg_unwind* u = (kg_unwind*)rec;
    kg_stdargs* s = (kg_stdargs*)(u + 1);
    s->args = args;
    s->n = n;
    s->lent = (uint64_t)lent;
    s->heap = NULL;
    u->prev = g_unwind;
    u->run = unwind_std_args;
    u->p = s;
    u->v = NULL;
    u->i = 0;
    u->m = NULL;
    g_unwind = u;
}
void rt_unwind_leave(void* rec) {
    g_unwind = ((kg_unwind*)rec)->prev;
}
// Nothing catches a panic outside a task, so the caller sweeps its own frame after a faulting release.
int rt_release_outer(val v) {
    if (g_task_catch) {
        rt_release(v);
        return 0;
    }
    uint64_t seen = g_fault_count;
    g_outer_drop++;
    rt_release(v);
    g_outer_drop--;
    return g_fault_count != seen;
}
void rt_sweep_release(val v) {
    g_outer_drop++;
    rt_release(v);
    g_outer_drop--;
}
void rt_outer_fatal(void) {
    rt_fatal(g_fault_msg);
}
void rt_drop_outer(val v) {
    if (rt_release_outer(v)) rt_outer_fatal();
}
#else
int rt_release_outer(val v) {
    rt_release(v);
    return 0;
}
void rt_sweep_release(val v) {
    rt_release(v);
}
void rt_outer_fatal(void) {
    rt_fatal("panic");
}
void rt_drop_outer(val v) {
    rt_release(v);
}
void rt_unwind_enter(void* rec, int64_t n) {
    (void)rec;
    (void)n;
}
void rt_unwind_enter_slots(void* rec, void* slots, uint64_t* mask, int64_t n) {
    (void)rec;
    (void)slots;
    (void)mask;
    (void)n;
}
void rt_unwind_enter_std(void* rec, val* args, int64_t n, int64_t lent) {
    (void)rec;
    (void)args;
    (void)n;
    (void)lent;
}
void rt_unwind_leave(void* rec) {
    (void)rec;
}
#endif
// rt_panicf's every caller passes a fixed message with exactly one "%s"; this
// avoids a full printf just for that one substitution.
void rt_panicf(const char* fmt, const char* a) {
    char buf[512];
    size_t n = 0;
    for (const char* f = fmt; *f && n + 1 < sizeof buf; f++) {
        if (f[0] == '%' && f[1] == 's') {
            for (const char* s = a; *s && n + 1 < sizeof buf; s++) buf[n++] = *s;
            f++;
            continue;
        }
        buf[n++] = *f;
    }
    buf[n] = 0;
    rt_panic(buf);
}

