#include "helper.h"

#include <math.h>
#include <stdlib.h>
#include <string.h>

/* Defined by Kigumi's export(C) block. They are declared here rather than
 * in helper.h so the header import does not also expose them as extern
 * declarations of the same C symbols. */
int32_t kg_visit(int32_t x);
int32_t kg_native_score(int32_t x);
int64_t kg_report(void *userdata);
int64_t kg_naked_triple(int64_t x);

static int32_t counter;

int32_t counter_next(void) {
    counter += 1;
    return counter;
}

int32_t counter_peek(void) {
    return counter;
}

Vec2 vec2_scale(Vec2 v, double k) {
    Vec2 out = { v.x * k, v.y * k };
    return out;
}

double vec2_length(Vec2 v) {
    return sqrt(v.x * v.x + v.y * v.y);
}

void vec2_shift(Vec2 *v, double dx, double dy) {
    v->x += dx;
    v->y += dy;
}

int32_t vec2_sizeof(void) {
    return (int32_t)sizeof(Vec2);
}

char *greeting_for(const char *name) {
    size_t len = strlen(name) + 8;
    char *out = malloc(len);
    if (out == NULL) {
        return NULL;
    }
    strcpy(out, "hello, ");
    strcat(out, name);
    return out;
}

void greeting_free(char *text) {
    free(text);
}

int32_t walk(visit_fn visit, int32_t n) {
    int32_t sum = 0;
    for (int32_t i = 0; i < n; i += 1) {
        sum += visit(i);
    }
    return sum;
}

int32_t call_named_score(int32_t x) {
    return kg_native_score(x) + 1;
}

int64_t call_report(void *userdata) {
    return kg_report(userdata);
}

int64_t call_naked_triple(int64_t x) {
    return kg_naked_triple(x);
}
