// Declares core helpers rt.h doesn't expose, for rt_cap_*.c.inc to call.
#ifndef KIGUMI_RT_INTERNAL_H
#define KIGUMI_RT_INTERNAL_H
#include "rt.h"

typedef struct rt_allocator rt_allocator;
static void* amalloc(rt_allocator* a, size_t n);
static void afree(rt_allocator* a, void* p);
static void* arealloc(rt_allocator* a, void* p, size_t oldn, size_t newn);
static rt_allocator* cur_alloc(void);
static void rt_fatal(const char* msg);
static val alloc_in(rt_allocator* a, int kind);
static val opaque_in(rt_allocator* a, const char* kind, void* data);
static rt_vdesc* find_variant(rt_type* t, const char* name);
static val try_ok(val v);
static val alloc_error(void);
static int borrow_field(val r, int64_t i);
#endif
