#!/bin/sh
# gen --value N --out FILE: writes a C function returning N.
printf '#include <stdint.h>\nint64_t build_budget(void) { return %s; }\n' "$2" > "$4"
