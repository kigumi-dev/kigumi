typedef struct Single {
    unsigned char tag;
    float a;
    double d;
    float b;
    float c;
} Single;

unsigned long cheaderf_sizeof(void);
unsigned long cheaderf_alignof(void);
unsigned long cheaderf_off_a(void);
unsigned long cheaderf_off_b(void);
unsigned long cheaderf_off_c(void);
Single cheaderf_make(float a, double d, float b);
float cheaderf_sum(Single s);
Single cheaderf_scale(Single s, float k);
float cheaderf_half(float x);
