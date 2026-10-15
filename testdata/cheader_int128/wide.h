typedef struct Wide {
    unsigned char tag;
    unsigned __int128 big;
    __int128 neg;
    int last;
} Wide;

unsigned long cheader128_sizeof(void);
unsigned long cheader128_alignof(void);
unsigned long cheader128_off_big(void);
unsigned long cheader128_off_neg(void);
unsigned long cheader128_off_last(void);
Wide cheader128_make(unsigned long long hi, unsigned long long lo, long long sign, int last);
unsigned long long cheader128_hi(Wide w);
unsigned long long cheader128_lo(Wide w);
Wide cheader128_bump(Wide w);
