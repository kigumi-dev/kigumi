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
static rt_type borrow_field_type = {.name = "BorrowField", .kind = 1, .borrow_fields = 1};
static val g_borrow_child;

static void drop_borrow_record(val unused) {
    (void)unused;
    val fields[1] = {g_borrow_child};
    rt_release(rt_record(&borrow_field_type, 1, fields));
}

// The field is a borrow: the record never owns its referent, so a drop under a catch (which scans
// children for user code) must not read a referent that is already gone.
static void test_borrow_field_drop(void) {
    int64_t live = g_general.live;
    g_borrow_child = rt_int(1LL << 40, 0);
    rt_release(g_borrow_child);
    CHECK(contained(drop_borrow_record, NULL) == 1);
    CHECK_LIVE(live);
}

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

static rt_vdesc probe_variant = {.name = "Probe"};

static void build_record(val field) {
    rt_release(rt_record(&dropped_type, 1, (val[]){field}));
}
static void build_record_borrowed(val field) {
    rt_release(rt_record_borrowed(&dropped_type, 1, (val[]){field}));
}
static void build_array(val unused) {
    (void)unused;
    rt_release(rt_array(1, (val[]){NULL}));
}
static void build_variant(val unused) {
    (void)unused;
    rt_release(rt_variant(&probe_variant, 1, (val[]){NULL}));
}

// Fault tick on the second allocation, the element storage: the header is discarded and an owning
// record releases its field; counts end level.
static void test_constructor_storage_failure(void) {
    void (*builders[])(val) = {build_record, build_array, build_variant, build_record_borrowed};
    for (int k = 0; k < 4; k++) {
        val field = k == 0 || k == 3 ? rt_int(1LL << 40, 0) : NULL;
        g_fault_n = 0;
        int64_t live = g_general.live - (field ? 1 : 0);
#ifdef KIGUMI_RC_CHECK
        long objs = g_live_objs - (field ? 1 : 0);
#endif
        CHECK(contained(builders[k], field) == 0);
        if (k == 3) rt_release(field);
        CHECK_LIVE(live);
#ifdef KIGUMI_RC_CHECK
        CHECK(g_live_objs == objs);
#endif
    }
}

int main(int argc, char** argv) {
    if (argc > 1 && strcmp(argv[1], "allocfail") == 0) {
        test_alloc_failure();
    } else if (argc > 1 && strcmp(argv[1], "recordfail") == 0) {
        test_constructor_storage_failure();
    } else {
        test_value_and_copy();
        test_drop_hook();
        test_borrow_field_drop();
        test_unwind();
    }
    if (failures) {
        fprintf(stderr, "%d failure(s)\n", failures);
        return 1;
    }
    printf("ok\n");
    return 0;
}
