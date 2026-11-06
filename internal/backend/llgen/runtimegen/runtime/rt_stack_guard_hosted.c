// Hosted POSIX only: a guard-page fault becomes the runtime's "stack overflow" panic, and the hash seed comes from the OS.
// Other targets keep the weak defaults in rt_sys_weak.c.
#define _GNU_SOURCE
#include <pthread.h>
#include <signal.h>
#include <stdint.h>
#include <sys/mman.h>
#include <sys/resource.h>
#include <unistd.h>
#ifdef __linux__
#include <sys/syscall.h>
// A `-include` header may pull in pthread.h before _GNU_SOURCE is set, hiding this prototype.
int pthread_getattr_np(pthread_t, pthread_attr_t*);
#endif

#include "rt.h"

#if defined(__linux__) || defined(__APPLE__) || defined(__FreeBSD__)
#include <sys/random.h>
int64_t rt_sys_seed(void) {
    int64_t seed = 0;
    return getentropy(&seed, sizeof seed) == 0 ? seed : 0;
}
#endif

#if defined(__linux__) || defined(__APPLE__)

#define KG_ALT_SIZE (256 * 1024)
#define KG_GUARD_WINDOW (2 * 1024 * 1024)

#ifdef KIGUMI_THREADS
#define KG_TLS _Thread_local
#else
#define KG_TLS
#endif
extern KG_TLS uintptr_t rt_stack_floor;
extern KG_TLS uintptr_t rt_stack_saved;
extern KG_TLS int rt_stack_forced;
extern KG_TLS int rt_in_libc;

static _Thread_local void* t_alt;
static pthread_key_t g_alt_key;
static pthread_once_t g_once = PTHREAD_ONCE_INIT;
static int g_installed;

static void on_fault(int sig, siginfo_t* si, void* ctx) {
    (void)ctx;
    uintptr_t addr = (uintptr_t)si->si_addr;
    int forced = rt_stack_forced;
    rt_stack_forced = 0;
    if (rt_stack_floor && (forced || (addr + KG_GUARD_WINDOW >= rt_stack_floor && addr < rt_stack_floor + KG_GUARD_WINDOW))) {
        // The interrupted malloc still holds its lock, so a panic that allocates would deadlock.
        if (rt_in_libc) {
            static const char msg[] = "panic: stack overflow\n";
            rt_sys_write(2, msg, sizeof msg - 1);
            _exit(2);
        }
        // The unwind and the panic message run on the little stack left, so the check must not fire again.
        rt_stack_saved = rt_stack_floor;
        rt_stack_floor = 0;
        rt_panic("stack overflow");
    }
    signal(sig, SIG_DFL);
}

static void free_alt(void* alt) {
    stack_t off = {.ss_flags = SS_DISABLE};
    sigaltstack(&off, NULL);
    munmap(alt, KG_ALT_SIZE);
}

static void install_handler(void) {
    pthread_key_create(&g_alt_key, free_alt);
    struct sigaction sa = {0};
    sa.sa_sigaction = on_fault;
    sigemptyset(&sa.sa_mask);
    sa.sa_flags = SA_SIGINFO | SA_ONSTACK | SA_NODEFER;
    int sigs[] = {SIGSEGV, SIGBUS};
    for (int i = 0; i < 2; i++) {
        struct sigaction cur;
        // A sanitizer or an embedder that already owns the signal keeps it.
        if (sigaction(sigs[i], NULL, &cur) == 0 && cur.sa_handler == SIG_DFL && sigaction(sigs[i], &sa, NULL) == 0) g_installed = 1;
    }
}

// musl reports a made-up size for the main thread, so its limit comes from RLIMIT_STACK.
#ifdef __linux__
static uintptr_t main_stack_low(void) {
    struct rlimit rl;
    if (getrlimit(RLIMIT_STACK, &rl) != 0 || rl.rlim_cur == RLIM_INFINITY) return 0;
    return (uintptr_t)__builtin_frame_address(0) - (uintptr_t)rl.rlim_cur;
}
#endif

static uintptr_t stack_low(void) {
#ifdef __APPLE__
    pthread_t self = pthread_self();
    return (uintptr_t)pthread_get_stackaddr_np(self) - pthread_get_stacksize_np(self);
#else
    if (syscall(SYS_gettid) == getpid()) return main_stack_low();
    pthread_attr_t attr;
    void* addr = NULL;
    size_t size = 0;
    if (pthread_getattr_np(pthread_self(), &attr) != 0) return 0;
    pthread_attr_getstack(&attr, &addr, &size);
    pthread_attr_destroy(&attr);
    return (uintptr_t)addr;
#endif
}

void rt_sys_stack_force(void) { raise(SIGSEGV); }

void rt_sys_stack_guard(void) {
    stack_t cur;
    if (t_alt || (sigaltstack(NULL, &cur) == 0 && !(cur.ss_flags & SS_DISABLE))) return;
    void* alt = mmap(NULL, KG_ALT_SIZE, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    if (alt == MAP_FAILED) return;
    stack_t ss = {.ss_sp = alt, .ss_size = KG_ALT_SIZE};
    if (sigaltstack(&ss, NULL) != 0) {
        munmap(alt, KG_ALT_SIZE);
        return;
    }
    t_alt = alt;
    pthread_once(&g_once, install_handler);
    pthread_setspecific(g_alt_key, alt);
    if (g_installed) rt_stack_floor = stack_low();
}

#else

void rt_sys_stack_guard(void) {}
void rt_sys_stack_force(void) {}

#endif
