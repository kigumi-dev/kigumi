// Package zigtool resolves the zig binary a native build or header import
// should run, and checks its reported version against a module's pin.
package zigtool

import (
	"fmt"
	"os"
)

// Resolve finds the zig binary: explicit (a caller-supplied path, such as
// $KIGUMI_ZIG threaded in by internal/cli) first, then the lookup.
func Resolve(look Lookup, explicit string) (string, error) {
	if explicit == "" {
		return look.find("zig")
	}
	if st, err := os.Stat(explicit); err != nil {
		return "", fmt.Errorf("KIGUMI_ZIG=%s: %w", explicit, err)
	} else if st.IsDir() {
		return "", fmt.Errorf("KIGUMI_ZIG=%s is a directory, not the zig binary", explicit)
	}
	return explicit, nil
}
