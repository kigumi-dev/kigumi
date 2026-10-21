package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/sem"
)

// A libc call the runtime makes is a name export(C) must not redefine, unless it is one of the rt_sys_* points an embedder overrides.
func TestExportRejectedLibcNamesCoverRuntimeCalls(t *testing.T) {
	libc := sem.LibcSymbols()
	for name := range llgen.FreestandingSymbols() {
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, "rt_") {
			continue
		}
		if !libc[name] {
			t.Errorf("%s is a libc call of the runtime but export(C) may define it", name)
		}
	}
	if libc["rt_sys_write"] {
		t.Error("rt_sys_write is an override point and must stay exportable")
	}
}
