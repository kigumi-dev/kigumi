// Hosted-only: turns delivery of a watched OS signal into a flag std/os can
// poll. The handler touches nothing but this array, so it never re-enters
// the refcounted runtime (rt_core.c) from signal context.
#include <signal.h>
#include <string.h>

#define KG_NSIG 64

static volatile sig_atomic_t kg_sig_flag[KG_NSIG];
// What each watched signal's disposition was before the first watch replaced
// it; the matching unwatch (a dropped std/os.Watch guard) puts it back, so a
// watch scope ending restores SIGINT/SIGTERM's default termination.
static struct sigaction kg_sig_saved[KG_NSIG];
static int kg_sig_watched[KG_NSIG];
static int kg_sig_depth[KG_NSIG];

static void kg_signal_handler(int sig) {
    if (sig >= 0 && sig < KG_NSIG) kg_sig_flag[sig] = 1;
}

// which is std/os.Signal's discriminant, not a raw signal number, so the
// mapping to each platform's SIG* macro lives here instead of in Kigumi.
static int kg_signal_number(int which) {
    switch (which) {
        case 0: return SIGINT;
        case 1: return SIGTERM;
        case 2: return SIGWINCH;
        default: return -1;
    }
}

// Nested watches share one installed disposition and restore it when the last
// guard is dropped, so an inner scope ending never un-watches an outer one.
int rt_sys_signal_watch(int which) {
    int sig = kg_signal_number(which);
    if (sig < 0 || sig >= KG_NSIG) return -1;
    if (!kg_sig_watched[sig]) {
        struct sigaction sa;
        memset(&sa, 0, sizeof sa);
        sa.sa_handler = kg_signal_handler;
        sigemptyset(&sa.sa_mask);
        sa.sa_flags = SA_RESTART;
        if (sigaction(sig, &sa, &kg_sig_saved[sig]) != 0) return -1;
        kg_sig_watched[sig] = 1;
    }
    kg_sig_depth[sig]++;
    return 0;
}

int rt_sys_signal_unwatch(int which) {
    int sig = kg_signal_number(which);
    if (sig < 0 || sig >= KG_NSIG) return -1;
    if (!kg_sig_watched[sig] || kg_sig_depth[sig] <= 0) return 0;
    if (--kg_sig_depth[sig] > 0) return 0;
    if (sigaction(sig, &kg_sig_saved[sig], NULL) != 0) return -1;
    kg_sig_watched[sig] = 0;
    kg_sig_flag[sig] = 0;
    return 0;
}

int rt_sys_signal_pending(int which) {
    int sig = kg_signal_number(which);
    if (sig < 0 || sig >= KG_NSIG) return 0;
    if (kg_sig_flag[sig]) {
        kg_sig_flag[sig] = 0;
        return 1;
    }
    return 0;
}
