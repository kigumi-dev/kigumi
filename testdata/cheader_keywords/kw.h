typedef struct Pair { int type; int match; } Pair;
int params(int type, int pub, int defer, int test, int comptime, int var, int fn, int export);
int match(int in);
int fail(int move);
#define is 4
