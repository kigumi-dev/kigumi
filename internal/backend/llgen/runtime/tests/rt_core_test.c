// Unit tests for the C runtime's contracts, compiled against rt_core.c itself so the
// static helpers are reachable. Run by TestRuntimeCUnitContracts in the driver package,
// once as a plain build and once with KIGUMI_FAIL_ALLOC_AT=1 for the allocator refusal.
#include "rt_core.c"

#include <stdio.h>
#include <string.h>

static int failures;

#define CHECK(cond)                                                       \
    do {                                                                  \
        if (!(cond)) {                                                    \
            fprintf(stderr, "FAIL %d: %s\n", __LINE__, #cond);            \
            failures++;                                                   \
        }                                                                 \
    } while (0)

// The RC checker owns leak accounting when it is on; it may keep bookkeeping objects that the
// exact live counter sees, so only a plain build asserts the counter returns to its baseline.
#ifdef KIGUMI_RC_CHECK
#define CHECK_LIVE(live) ((void)(live))
#else
#define CHECK_LIVE(live) CHECK(g_general.live == (live))
#endif

// A data record with a drop hook: releasing it must run the hook exactly once.
static int g_dropped;
static val drop_hook(val env, val args) {
    (void)env;
    (void)args;
    g_dropped++;
    return rt_unit();
}

static rt_type dropped_type = {.name = "DropProbe", .kind = 1, .drop = (void*)drop_hook};

// Values own their children: releasing the array releases the boxed int and the string,
// and a copy is a new array whose elements are themselves copied.
static void test_value_and_copy(void) {
    int64_t live = g_general.live;
    val item = rt_array(1, (val[]){rt_int(1LL << 40, 0)});
    val outer = rt_array(1, (val[]){item});
    CHECK(outer->kind == K_ARRAY && outer->u.arr.len == 1);
    CHECK(outer->u.arr.items[0] == item);

    val copied = rt_copy(outer);
    CHECK(copied != outer);
    CHECK(copied->u.arr.len == 1);
    CHECK(copied->u.arr.items[0] != item);
    CHECK(copied->u.arr.items[0]->u.arr.items[0]->u.i == (1LL << 40));

    rt_release(copied);
    rt_release(outer);
    CHECK_LIVE(live);
}

static void test_drop_hook(void) {
    int64_t live = g_general.live;
    val record = rt_record(&dropped_type, 0, NULL);
    CHECK(g_dropped == 0);
    rt_release(record);
    CHECK(g_dropped == 1);
    CHECK_LIVE(live);
}

static void boom(val unused) {
    (void)unused;
    rt_panic("boom");
}

// A contained panic leaves no allocation or backward reference behind.
static void test_unwind(void) {
    int64_t live = g_general.live;
    CHECK(contained(boom, NULL) == 0);
    CHECK_LIVE(live);
}

static void allocate_int(val unused) {
    (void)unused;
    rt_release(rt_int(1LL << 40, 0));
}

// Runs first in the allocfail process: the fault tick must turn the first value
// allocation into the out-of-memory panic, which contained turns into a failure report.
static void test_alloc_failure(void) {
    int64_t live = g_general.live;
    CHECK(contained(allocate_int, NULL) == 0);
    CHECK_LIVE(live);
}

int main(int argc, char** argv) {
    if (argc > 1 && strcmp(argv[1], "allocfail") == 0) {
        test_alloc_failure();
    } else {
        test_value_and_copy();
        test_drop_hook();
        test_unwind();
    }
    if (failures) {
        fprintf(stderr, "%d failure(s)\n", failures);
        return 1;
    }
    printf("ok\n");
    return 0;
}
