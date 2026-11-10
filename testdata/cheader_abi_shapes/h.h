struct Node { struct Node *next; int v; };
typedef struct A A;
typedef struct B B;
struct A { B *b; int x; };
struct B { A *a; int y; };

#pragma pack(push, 1)
typedef struct P1 { char c; int x; } P1;
#pragma pack(pop)
typedef struct __attribute__((packed)) P2 { char c; int x; } P2;
typedef struct __attribute__((aligned(16))) A16 { char c; } A16;

typedef int (*FnPtr)(int, ...);
typedef int (*FnPtr2)(const char *s, void *p);

int node_sum(struct Node *n);
int use_a(A *a);
int call_fn(FnPtr2 f);
