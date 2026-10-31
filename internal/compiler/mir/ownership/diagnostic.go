package ownership

import "kigumi/internal/compiler/sem"

// DiagnosticCode is the catalog number for a reason the sem catalog names, so `kigumi explain` can find it.
func DiagnosticCode(name string) string {
	if c, ok := sem.CodeByName(name); ok {
		return c.Num
	}
	return name
}
