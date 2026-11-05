val rt_retain(val v) {
    CHK(v);
#ifdef KIGUMI_THREADS
    if (v && atomic_load_explicit(&v->rc, memory_order_relaxed) > 0) atomic_fetch_add_explicit(&v->rc, 1, memory_order_relaxed);
#else
    if (v && v->rc > 0) v->rc++;
#endif
    return v;
}
static void free_obj(val v);
val rt_future_new(val callee, int n, val* args);
static int release_dec(val v) {
    CHK(v);
    if (!v) return 0;
#ifdef KIGUMI_THREADS
    if (atomic_load_explicit(&v->rc, memory_order_relaxed) <= 0) return 0;
#ifdef __SANITIZE_THREAD__
    return atomic_fetch_sub_explicit(&v->rc, 1, memory_order_acq_rel) == 1;
#else
    if (atomic_fetch_sub_explicit(&v->rc, 1, memory_order_release) != 1) return 0;
    atomic_thread_fence(memory_order_acquire);
    return 1;
#endif
#else
    if (v->rc <= 0) return 0;
    return --v->rc == 0;
#endif
}
void rt_release(val v) {
    if (release_dec(v)) free_obj(v);
}
#ifdef KIGUMI_LEAK_CHECK
// Test-only: how many allocations are still live at a fatal exit.
static void report_live(void) {
    char buf[32];
    int n = sizeof buf;
    buf[--n] = '\n';
    long long v = (long long)g_general.live;
    do {
        buf[--n] = (char)('0' + v % 10);
        v /= 10;
    } while (v);
    rt_sys_write(2, "live: ", 6);
    rt_sys_write(2, buf + n, (size_t)(sizeof buf - n));
}
#endif
#ifdef KIGUMI_THREADS
static pthread_mutex_t g_out = PTHREAD_MUTEX_INITIALIZER;
#define OUT_LOCK() (kg_stack_check_locked(), pthread_mutex_lock(&g_out))
#define OUT_UNLOCK() pthread_mutex_unlock(&g_out)
#else
#define OUT_LOCK() ((void)0)
#define OUT_UNLOCK() ((void)0)
#endif
// A line is one locked write sequence, so lines from parallel tasks never interleave.
static void write_line(int fd, const char* pre, size_t pn, const char* p, size_t n) {
    char buf[1024];
    OUT_LOCK();
    if (pn + n < sizeof buf) {
        memcpy(buf, pre, pn);
        memcpy(buf + pn, p, n);
        buf[pn + n] = '\n';
        rt_sys_write(fd, buf, pn + n + 1);
    } else {
        if (pn) rt_sys_write(fd, pre, pn);
        rt_sys_write(fd, p, n);
        rt_sys_write(fd, "\n", 1);
    }
    OUT_UNLOCK();
}
static void rt_fatal(const char* msg) {
    write_line(2, "panic: ", 7, msg, strlen(msg));
#ifdef KIGUMI_LEAK_CHECK
    report_live();
#endif
    rt_sys_exit(2);
}
void rt_panic(const char* msg) {
#ifdef KG_CATCH
    if (g_task_catch) task_fault(msg);
#endif
    rt_fatal(msg);
}
