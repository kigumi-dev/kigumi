// Stand-ins for <stdlib.h>/<string.h>/<math.h>, absent on a real
// freestanding triple; freestanding_symbols.txt pins the symbols left undefined here.
#ifndef KIGUMI_RT_FREESTANDING_H
#define KIGUMI_RT_FREESTANDING_H
#include <stddef.h>

// Avoids an extra undefined symbol for math.h's type-generic isinf macro.
#define isinf(x) __builtin_isinf(x)

void* malloc(size_t n);
void* calloc(size_t n, size_t sz);
void* realloc(void* p, size_t n);
void free(void* p);
void* memcpy(void* dst, const void* src, size_t n);
void* memset(void* dst, int c, size_t n);
int memcmp(const void* a, const void* b, size_t n);
size_t strlen(const char* s);
int strcmp(const char* a, const char* b);
int strncmp(const char* a, const char* b, size_t n);
char* strchr(const char* s, int c);
char* strrchr(const char* s, int c);
char* strstr(const char* h, const char* n);
double pow(double a, double b);
double sqrt(double a);
double floor(double a);
double ceil(double a);
double round(double a);
double trunc(double a);
#endif
