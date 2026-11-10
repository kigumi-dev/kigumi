typedef struct Pair { int type; int match; } Pair;
int params(int type, int pub, int defer, int test, int comptime, int var, int fn, int export);
int match(int in);
int fail(int move);
#define is 4

// C allows Kigumi's keywords as identifiers; type names have to be renamed
// too, not just parameters and fields.
struct fn { int x; };
typedef struct fn fn_t;
typedef int mut;
typedef int type;
int mut_add(mut a, mut b);
int fn_t_sum(fn_t p);
