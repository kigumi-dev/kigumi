package version

import (
	"fmt"
	"strconv"
	"strings"
)

// AtLeast reports whether have is at least want, comparing dotted numeric
// toolchain versions component by component. A manifest's `kigumi` field is
// such a minimum; a non-numeric component is reported instead of compared as
// 0. internal/toolchain/module enforces the same rule for `Module { kigumi }`.
func AtLeast(have, want string) (bool, error) {
	h, w := strings.Split(have, "."), strings.Split(want, ".")
	for i := 0; i < len(h) || i < len(w); i++ {
		var a, b int
		var err error
		if i < len(h) {
			if a, err = strconv.Atoi(h[i]); err != nil {
				return false, fmt.Errorf("version %q: component %q is not a number", have, h[i])
			}
		}
		if i < len(w) {
			if b, err = strconv.Atoi(w[i]); err != nil {
				return false, fmt.Errorf("version %q: component %q is not a number", want, w[i])
			}
		}
		if a != b {
			return a > b, nil
		}
	}
	return true, nil
}
