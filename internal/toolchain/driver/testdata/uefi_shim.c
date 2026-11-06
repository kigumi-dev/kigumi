typedef unsigned char u8; typedef unsigned int u32; typedef unsigned long long u64; typedef unsigned long usize; typedef long i64;
static inline void outb(unsigned short p, u8 v) { __asm__ volatile("outb %0, %1" : : "a"(v), "Nd"(p)); }
void serial_putc(u8 c) { outb(0x3f8, c); }
void serial_write(const char *s, usize n) { for (usize i = 0; i < n; i++) serial_putc((u8)s[i]); }
void qemu_exit(u8 code) { outb(0x501, code); for (;;) __asm__ volatile("hlt"); }
void rt_sys_write(int fd, const char *p, usize n) { (void)fd; serial_write(p, n); }
void rt_sys_exit(int code) { serial_write("SYS-EXIT\n", 9); qemu_exit((u8)code); }
struct efi_table_header { u64 sig; u32 rev; u32 hdrsize; u32 crc; u32 reserved; };
struct efi_boot_services { struct efi_table_header hdr; void *a,*b,*c,*d,*e; u64 (*allocate_pool)(u32, usize, void **); u64 (*free_pool)(void *); };
struct efi_system_table { struct efi_table_header hdr; void *a,*b,*c,*d,*e,*f,*g,*h,*i; struct efi_boot_services *boot; };
static struct efi_boot_services *g_bs;
struct hdr { usize size; };
void *malloc(usize n) { void *p = 0; if (!g_bs || g_bs->allocate_pool(0, n + sizeof(struct hdr), &p) != 0) return 0; ((struct hdr*)p)->size = n; return (char*)p + sizeof(struct hdr); }
void free(void *p) { if (p && g_bs) g_bs->free_pool((char*)p - sizeof(struct hdr)); }
void *calloc(usize n, usize s) { usize t = n*s; void *p = malloc(t); if (p) for (usize i=0;i<t;i++) ((u8*)p)[i]=0; return p; }
void *realloc(void *p, usize n) { if (!p) return malloc(n); usize o=((struct hdr*)((char*)p-sizeof(struct hdr)))->size; void *q=malloc(n); if(!q) return 0; usize m=o<n?o:n; for(usize i=0;i<m;i++) ((u8*)q)[i]=((u8*)p)[i]; free(p); return q; }
usize strlen(const char *s){usize n=0;while(s[n])n++;return n;}
int strcmp(const char*a,const char*b){while(*a&&*a==*b){a++;b++;}return (u8)*a-(u8)*b;}
int strncmp(const char*a,const char*b,usize n){for(usize i=0;i<n;i++){if(a[i]!=b[i])return (u8)a[i]-(u8)b[i];if(!a[i])break;}return 0;}
char *strrchr(const char*s,int c){const char*r=0;do{if(*s==(char)c)r=s;}while(*s++);return(char*)r;}
int memcmp(const void*a,const void*b,usize n){const u8*x=a,*y=b;for(usize i=0;i<n;i++)if(x[i]!=y[i])return x[i]-y[i];return 0;}
void *memcpy(void*d,const void*s,usize n){u8*x=d;const u8*y=s;for(usize i=0;i<n;i++)x[i]=y[i];return d;}
void *memmove(void*d,const void*s,usize n){u8*x=d;const u8*y=s;if(x<y)for(usize i=0;i<n;i++)x[i]=y[i];else for(usize i=n;i>0;i--)x[i-1]=y[i-1];return d;}
void *memset(void*d,int c,usize n){u8*x=d;for(usize i=0;i<n;i++)x[i]=(u8)c;return d;}
double trunc(double x){return(double)(i64)x;}
int kigumi_main(int, void*);
void efi_main(void *image, struct efi_system_table *st) { (void)image; g_bs = st->boot; serial_write("EFI-BOOT\n", 9); int rc = kigumi_main(0, 0); serial_write("EFI-RET\n", 8); qemu_exit((u8)rc); }
double floor(double x) {
    union { double d; unsigned long long u; } v = { x };
    int e = (int)((v.u >> 52) & 0x7ff) - 1023;
    if (e >= 52) return x;
    if (e < 0) return x < 0 ? -1.0 : 0.0;
    unsigned long long fracMask = (1ULL << (52 - e)) - 1;
    if (v.u & fracMask) {
        if (x < 0) v.u += 1ULL << (52 - e);
        v.u &= ~fracMask;
    }
    return v.d;
}
double ceil(double x) { return -floor(-x); }
double round(double x) {
    double t = trunc(x), f = x - t;
    if (f >= 0.5) t += 1.0;
    else if (f <= -0.5) t -= 1.0;
    return t;
}
