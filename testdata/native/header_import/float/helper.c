#include <stddef.h>

struct Single {
    unsigned char tag;
    float a;
    double d;
    float b;
    float c;
};

unsigned long cheaderf_sizeof(void) { return sizeof(struct Single); }
unsigned long cheaderf_alignof(void) { return _Alignof(struct Single); }
unsigned long cheaderf_off_a(void) { return offsetof(struct Single, a); }
unsigned long cheaderf_off_b(void) { return offsetof(struct Single, b); }
unsigned long cheaderf_off_c(void) { return offsetof(struct Single, c); }

struct Single cheaderf_make(float a, double d, float b) {
    struct Single s;
    s.tag = 3;
    s.a = a;
    s.d = d;
    s.b = b;
    s.c = a + b;
    return s;
}

float cheaderf_sum(struct Single s) { return s.a + s.b + s.c + (float)s.d; }

struct Single cheaderf_scale(struct Single s, float k) {
    s.tag += 1;
    s.a *= k;
    s.d *= k;
    s.b *= k;
    s.c *= k;
    return s;
}

float cheaderf_half(float x) { return x / 2; }
