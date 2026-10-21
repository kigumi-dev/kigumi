package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/sem"
)

// checkCRange panics unless the 64-bit usize or isize value w fits ct, the
// narrower size_t or ptrdiff_t of a 32-bit target.
func (e *emitter) checkCRange(sb *strings.Builder, w string, t sem.TypeID, ct string) {
	ext, name, msg := "zext", "size_t", "usize value does not fit the %s of this target"
	if t == sem.TyIsize {
		ext, name = "sext", "ptrdiff_t"
		msg = "isize value does not fit the %s of this target"
	}
	narrow, wide, bad := e.newTmp(), e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = trunc i64 %s to %s\n", narrow, w, ct)
	fmt.Fprintf(sb, "  %s = %s %s %s to i64\n", wide, ext, ct, narrow)
	fmt.Fprintf(sb, "  %s = icmp ne i64 %s, %s\n", bad, wide, w)
	e.overflowTrap(sb, bad, fmt.Sprintf(msg, name))
}
