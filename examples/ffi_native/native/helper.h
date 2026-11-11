#ifndef KIGUMI_NATIVE_HELPER_H
#define KIGUMI_NATIVE_HELPER_H

#include <stdint.h>

/* A plain C ABI record: passed and returned by value below. */
typedef struct Vec2 {
    double x;
    double y;
} Vec2;

/* State that lives in C's data segment for the whole process. */
int32_t counter_next(void);
int32_t counter_peek(void);

/* Records by value in both directions, plus a pointer-written record. */
Vec2 vec2_scale(Vec2 v, double k);
double vec2_length(Vec2 v);
void vec2_shift(Vec2 *v, double dx, double dy);
int32_t vec2_sizeof(void);

/* malloc/free pair: the guest copies the text out and then frees it. */
char *greeting_for(const char *name);
void greeting_free(char *text);

/* A guest callback: walk calls visit(0 .. n-1) and sums the results. */
typedef int32_t (*visit_fn)(int32_t);
int32_t walk(visit_fn visit, int32_t n);

/* C calls these exported Kigumi functions back across the boundary. */
int32_t call_named_score(int32_t x);
int64_t call_report(void *userdata);
int64_t call_naked_triple(int64_t x);

#endif
