#include <stdint.h>

#define PAREN_MIN (-5 - 1)
#define SHIFTED (1u << 31)
#define I32_FLOOR (-2147483647 - 1)
#define I16_FLOOR (-32767-1)
#define BIG 100000000000
#define MASK (0xff00 | 0x00ff)
#define MIXED ((3 + 4) * 2)
#define GREETING "hello, " "world"
#define NEG_UNSIGNED (-1u)
#define OVERFLOWS (2147483647 + 1)
#define DIV_ZERO (1 / 0)
#define WRAP_ADD (0xffffffff + 1)
#define WRAP_NOT (~0xffffffff)
#define WRAP_SHL (0x80000000 << 1)
#define NEG_HEX (-0x80000000)
#define OCT_WRAP (037777777777 + 1)
#define RESET "\x1b[0m"
#define NUL_END "nul\0end"
#define DOLLAR "cost: ${x}"
#define BAD_BYTE "\xff"
