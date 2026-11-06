package driver

import (
	"fmt"
	"strings"

	"kigumi/internal/backend/vm"
)

// nativeOnlyRefusal reports whether why is C ABI/asm/ffi/dl: `kigumi build` is the fix.
func nativeOnlyRefusal(why string) bool {
	switch why {
	case "asm", "primitive os.Host.dl":
		return true
	}
	return strings.HasPrefix(why, "foreign function ") ||
		strings.HasPrefix(why, "primitive ffi.") ||
		strings.HasPrefix(why, "primitive dl.") || strings.HasPrefix(why, "primitive ffi/dl.")
}

// vmRefusalError turns vm.Supports/SupportsTests' reason into the CLI
// error: native-only points at `kigumi build`, anything else just reports the gap.
func vmRefusalError(why string) error {
	if nativeOnlyRefusal(why) {
		return fmt.Errorf("%s is not supported by the VM: native-only, run `kigumi build` instead", why)
	}
	return fmt.Errorf("%s is not supported by the VM", why)
}

// VMRefusal is the error RunWith would return for a program the VM cannot run, or nil.
func VMRefusal(a *Analysis) error {
	if ok, why := vm.Supports(a.Prog); !ok {
		return vmRefusalError(why)
	}
	return nil
}
