#include <stddef.h>

struct Wide {
    unsigned char tag;
    unsigned __int128 big;
    __int128 neg;
    int last;
};

unsigned long cheader128_sizeof(void) { return sizeof(struct Wide); }
unsigned long cheader128_alignof(void) { return _Alignof(struct Wide); }
unsigned long cheader128_off_big(void) { return offsetof(struct Wide, big); }
unsigned long cheader128_off_neg(void) { return offsetof(struct Wide, neg); }
unsigned long cheader128_off_last(void) { return offsetof(struct Wide, last); }

struct Wide cheader128_make(unsigned long long hi, unsigned long long lo, long long sign, int last) {
    struct Wide w;
    w.tag = 7;
    w.big = ((unsigned __int128)hi << 64) | lo;
    w.neg = (__int128)sign * (__int128)w.big;
    w.last = last;
    return w;
}

unsigned long long cheader128_hi(struct Wide w) { return (unsigned long long)(w.big >> 64); }
unsigned long long cheader128_lo(struct Wide w) { return (unsigned long long)w.big; }

struct Wide cheader128_bump(struct Wide w) {
    w.tag += 1;
    w.big += 1;
    w.neg -= 1;
    w.last += 1;
    return w;
}

__int128 cheader128_add(__int128 a, unsigned __int128 b) { return a + (__int128)b; }

unsigned __int128 cheader128_shl(unsigned __int128 a, int n) { return a << n; }

int cheader128_sign(__int128 a) { return a < 0 ? -1 : (a > 0); }

extern __int128 kg_echo128(__int128 a, unsigned __int128 b);

__int128 cheader128_echo_call(__int128 a, unsigned __int128 b) { return kg_echo128(a, b); }
