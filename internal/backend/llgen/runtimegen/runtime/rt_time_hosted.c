// std/time reads the clock through the target's own <time.h>: the clock ids,
// the time64 symbol redirection (musl's __clock_gettime64/__nanosleep_time64,
// NetBSD's __clock_gettime50/__nanosleep50) and wasi-libc's pointer clockid_t
// are the C library's business, not something Kigumi can guess from a target
// name.
#include <errno.h>
#include <stddef.h>
#include <time.h>

// The record std/platform/clock.kg models. FreeBSD, NetBSD and OpenBSD on x86
// still have a 32-bit time_t and an 8-byte timespec; refuse the call instead
// of writing past the 16-byte record.
static int rt_time_layout_ok(void) {
    return sizeof(struct timespec) == 16 && offsetof(struct timespec, tv_nsec) == 8;
}

// 0 selects CLOCK_REALTIME and any other value CLOCK_MONOTONIC. Returns 0, or
// the errno that stopped the call.
int rt_time_clock(int which, struct timespec* out) {
    if (!rt_time_layout_ok()) return EINVAL;
    clockid_t id = which == 0 ? CLOCK_REALTIME : CLOCK_MONOTONIC;
    if (clock_gettime(id, out) != 0) return errno;
    return 0;
}

// Returns 0, or the errno that stopped the call.
int rt_time_sleep(const struct timespec* req, struct timespec* rem) {
    if (!rt_time_layout_ok()) return EINVAL;
    if (nanosleep(req, rem) != 0) return errno;
    return 0;
}
