// write, exit and close are weak so hosted std or an embedder can
// override them; every other host primitive is answered by std's Kigumi bodies.
#include "rt.h"

#if defined(__wasi__)
#include <unistd.h>
// wasi-libc ships getentropy but no getrandom, which std/entropy calls.
__attribute__((weak)) ssize_t getrandom(void* buf, size_t n, unsigned flags) {
    (void)flags;
    size_t done = 0;
    while (done < n) {
        size_t chunk = n - done < 256 ? n - done : 256;
        if (getentropy((char*)buf + done, chunk) != 0) return done ? (ssize_t)done : -1;
        done += chunk;
    }
    return (ssize_t)n;
}
#endif

__attribute__((weak)) void rt_sys_write(int fd, const char* p, size_t n) {
    (void)fd;
    (void)p;
    (void)n;
}
__attribute__((weak)) void rt_sys_exit(int code) {
    (void)code;
    for (;;) __builtin_trap();
}
__attribute__((weak)) int rt_sys_close(int fd) {
    (void)fd;
    return -1;
}
__attribute__((weak)) void rt_sys_stack_guard(void) {}
__attribute__((weak)) void rt_sys_stack_force(void) {}
val rt_sys_std(const char* k, int n, val* a, val a0, val a1, val a2) {
    (void)k;
    (void)n;
    (void)a;
    (void)a0;
    (void)a1;
    (void)a2;
    return NULL;
}
