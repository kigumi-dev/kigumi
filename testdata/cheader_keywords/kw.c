#include "kw.h"
int params(int type, int pub, int defer, int test, int comptime, int var, int fn, int export) {
    return type + 2 * pub + 3 * defer + 4 * test + 5 * comptime + 6 * var + 7 * fn + 8 * export;
}
int match(int in) { return in + 100; }
int fail(int move) { return move * 2; }
int mut_add(mut a, mut b) { return a + b; }
int fn_t_sum(fn_t p) { return p.x; }
