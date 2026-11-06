// Shared by rt_core.c and the rt_sys_*.c layer: the boxed value model and
// the core helpers a system layer builds on.
#ifndef KIGUMI_RT_H
#define KIGUMI_RT_H
#include <stddef.h>
#include <stdint.h>
#ifdef KIGUMI_CAP_TASK_THREADS
#define KIGUMI_THREADS
#endif
#ifdef KIGUMI_THREADS
#include <stdatomic.h>
// Under the thread profile a value's last reference can be dropped on a
// worker other than the one that made it, so rc is atomic.
#define KG_RC _Atomic int
#else
#define KG_RC int
#endif
typedef struct rt_type rt_type;
typedef struct rt_vdesc rt_vdesc;
enum { K_INT = 1,
       K_FLOAT,
       K_BOOL,
       K_STR,
       K_BYTES,
       K_CHAR,
       K_UNIT,
       K_RECORD,
       K_VARIANT,
       K_ARRAY,
       K_CLOSURE,
       K_BOX,
       K_CELL,
       K_OPAQUE,
       K_PTR };

typedef struct obj* val;
typedef val (*adapter_fn)(val env, val args);
struct obj {
    int kind;
    int nk;
    KG_RC rc;
    int owned;
    struct rt_allocator* al;
    union rt_value {
        int64_t i;
#ifdef KIGUMI_INT128
        // 128-bit integers: lo aliases i.
        struct {
            int64_t lo, hi;
        } w;
#endif
        double f;
        int b;
        uint32_t c;
        struct {
            const char* data;
            int64_t len;
        } s;
        struct {
            rt_type* type;
            int64_t n;
            val* fields;
        } rec;
        struct {
            rt_vdesc* v;
            int64_t n;
            val* payload;
        } var;
        struct {
            val* items;
            int64_t len, cap, head;
        } arr;
        struct {
            adapter_fn fn;
            val env;
            const char* key;
            uint32_t lent;
            int async;
        } clo;
        struct {
            rt_type* dyn;
            val v;
            // vt holds one adapter per requirement of the interface the
            // value was boxed as.
            void** vt;
            // src is the `&mut` cell an `&mut Interface` view lends self through, else NULL.
            val src;
        } box;
        struct {
            val v;
        } cell;
        struct {
            const char* kind;
            void* data;
        } op;
        void* p;
    } u;
};
struct rt_type {
    const char* name;
    int kind;
    int nfields;
    const char** fields;
    int nvariants;
    rt_vdesc** variants;
    void* drop;
    // borrow_fields bits the fields holding a borrow (`&'a T`);
    // rt_release/rt_copy skip them, since the record never owns them.
    uint64_t borrow_fields;
    // private_fields marks fields a secret-bearing type must not expose
    // through debug, which prints <private> instead of the value.
    uint64_t private_fields;
};
// nullary is this variant's payload-free value: immortal, filled once by
// rt_init_nullary, so constructing it allocates nothing.
struct rt_vdesc {
    const char* name;
    rt_type* adt;
    int index;
    int npayload;
    struct obj nullary;
};
// The emitted %rt_variant global reserves RT_OBJ_SLOT bytes for nullary at
// this offset; llgen (emit_data.go) must agree with both.
#define RT_OBJ_SLOT 64
_Static_assert(sizeof(struct obj) <= RT_OBJ_SLOT, "struct obj outgrew the emitted variant slot");
_Static_assert(offsetof(rt_vdesc, nullary) == 2 * sizeof(void*) + 2 * sizeof(int), "rt_vdesc prefix does not match the emitted %rt_variant");

typedef struct {
    char* p;
    size_t len, cap;
} buf;

// ---- system layer (rt_sys_weak.c; hosted std overrides the weak symbols) ----
void rt_sys_write(int fd, const char* p, size_t n);
void rt_sys_exit(int code);
int rt_sys_close(int fd);
// A random word for keyed hashing, or 0 when the platform has none.
int64_t rt_sys_seed(void);
// Called once per OS thread that runs Kigumi code; a hosted POSIX target installs its stack-overflow guard here.
void rt_sys_stack_guard(void);
void rt_sys_stack_force(void);
val rt_sys_std(const char* k, int n, val* a, val a0, val a1, val a2);

// ---- core helpers ----
void rt_panic(const char* msg);
void rt_panicf(const char* fmt, const char* a);
val rt_retain(val v);
void rt_release(val v);
val rt_deref(val v);
val rt_unit(void);
val rt_int(int64_t i, int nk);
val rt_float(double f, int nk);
val rt_bool(int b);
val rt_str(const char* data, int64_t len);
val rt_bytes(const char* data, int64_t len);
val rt_str_take(char* data, int64_t len);
val strdup_val(const char* s, int64_t len);
val bytes_val(const char* s, int64_t len);
val rt_from_cstr(const char* s);
val rt_ptr(void* p);
val rt_array(int64_t n, val* items);
void rt_push(val a, val x);
val rt_record(rt_type* t, int64_t n, val* fields);
val rt_nullary(rt_vdesc* d);
void rt_const_variant(void* slot, val* payload, rt_vdesc* d, int64_t n);
void rt_const_record(void* slot, val* fields, rt_type* t, int64_t n);
void rt_const_num(void* slot, int kind, int nk, int64_t i, double f);
void rt_const_text(void* slot, int kind, const char* data, int64_t len);
void rt_init_nullary(int ntypes, rt_type** types);
void rt_fnitem_init(void* slot, adapter_fn fn, int async);
void rt_stditem_init(void* slot, const char* key, uint32_t lent);
void rt_fnitem_lent(void* slot, int lent);
val rt_closure_lent(val v, int lent);
val rt_callv(val f, int n, val* args);
val rt_callv_m(val f, int n, val* args, uint32_t lent, uint32_t given);
val rt_callv_lent(val f, int n, val* args);
int64_t rt_callv_i64_m(val f, int n, val* args, uint32_t lent, uint32_t given);
void rt_cell_init(void* slot, val x);
val rt_cell_take(val c);
val rt_copy(val v);
val rt_copy_mut(val v);
void rt_setindex_copy(val base, val idx, val src, const char* site);
void rt_setfield_copy(val r, int64_t i, val src);
void rt_secure_zero(volatile unsigned char* p, int64_t n);
val rt_opaque(const char* kind, void* data);
val rt_some(val v);
val rt_none(void);
val rt_ok(val v);
val rt_err_val(val e);
val rt_err_msg(const char* msg);
val rt_make_record(const char* tname, int n, val* fields);
val rt_display(val v);
void display_into(buf* b, val v, int canon);
const char* rt_cstr(val s);
void rt_bput(buf* b, const char* s, size_t n);
void rt_bputs(buf* b, const char* s);
int rt_utf8_valid(const char* s, int64_t n);
rt_type* rt_find_type(const char* name);
int rt_equal(val a, val b);

// ---- rt_float.c: shortest round-trip formatting, correctly rounded parsing ----
void rt_format_float(double x, int f32, char* out, size_t n);
int rt_parse_f64(const char* s, const char** end, double* out, int* erange);
int rt_parse_f32(const char* s, const char** end, float* out, int* erange);
#endif
