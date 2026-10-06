package mir

import "testing"

func TestCapabilityMacro(t *testing.T) {
	cases := map[Capability]string{
		CapHostShell:      "KIGUMI_CAP_HOST_SHELL",
		CapFFIHandleTable: "KIGUMI_CAP_FFI_HANDLE_TABLE",
		CapTaskBlocking:   "KIGUMI_CAP_TASK_BLOCKING_POOL",
		CapTaskExecutor:   "KIGUMI_CAP_TASK_EXECUTOR",
		CapSecureErase:    "KIGUMI_CAP_MEM_SECURE_ERASE",
	}
	for c, want := range cases {
		if got := c.Macro(); got != want {
			t.Errorf("%s.Macro() = %s, want %s", c, got, want)
		}
	}
}

func TestCapabilityMacros(t *testing.T) {
	id, ok := StdOpId(0), false
	for i, name := range StdOpNames {
		if name == "TASK_SPAWN" {
			id, ok = StdOpId(i), true
			break
		}
	}
	if !ok {
		t.Fatal("TASK_SPAWN not found in StdOpNames")
	}
	got := CapabilityMacros([]StdOpId{id})
	want := []string{"KIGUMI_CAP_TASK_EXECUTOR"}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("CapabilityMacros(TASK_SPAWN) = %v, want %v", got, want)
	}
}

func TestNewCapabilitiesAreNotHostOnly(t *testing.T) {
	for _, c := range []Capability{CapFFIRaw, CapTaskExecutor, CapSecureErase} {
		if c.HostOnly() {
			t.Errorf("%s.HostOnly() = true, want false: it has an OS-free rt_core.c implementation", c)
		}
	}
}
