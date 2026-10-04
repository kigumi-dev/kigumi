package driver

import (
	"fmt"
	"strings"
)

// nativeOnlyRefusal reports whether why is C ABI/asm/cfnptr/ffi/dl: `kigumi build` is the fix, not --engine interp.
func nativeOnlyRefusal(why string) bool {
	switch why {
	case "callc", "asm", "cfnptr", "primitive os.Host.dl":
		return true
	}
	return strings.HasPrefix(why, "foreign function ") ||
		strings.HasPrefix(why, "primitive ffi.") ||
		strings.HasPrefix(why, "primitive dl.")
}

// vmRefusalError turns vm.Supports/SupportsTests' reason into the CLI
// error: native-only points at `kigumi build`, anything else at --engine interp.
func vmRefusalError(why string) error {
	if nativeOnlyRefusal(why) {
		return fmt.Errorf("%s is not supported by the VM: native-only, run `kigumi build` instead", why)
	}
	return fmt.Errorf("%s is not supported by the VM (run with --engine interp)", why)
}
