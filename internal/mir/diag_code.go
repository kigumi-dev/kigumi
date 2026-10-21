package mir

import "kigumi/internal/sem"

// diagCode is the catalog number for a reason the sem catalog names, so `kigumi explain` can find it.
func diagCode(name string) string {
	if c, ok := sem.CodeByName(name); ok {
		return c.Num
	}
	return name
}
