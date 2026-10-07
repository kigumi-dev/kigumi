package mir

import (
	"strings"
	"testing"
)

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

func TestCapabilitiesCloseOverNeeds(t *testing.T) {
	id, ok := StdOpIdOf("TASK_THREAD_POOL_EXECUTOR")
	if !ok {
		t.Fatal("TASK_THREAD_POOL_EXECUTOR not found in StdOpNames")
	}
	got := strings.Join(Capabilities([]StdOpId{id}), ",")
	if got != "task.executor,task.threads" {
		t.Errorf("Capabilities(TASK_THREAD_POOL_EXECUTOR) = %s, want task.executor,task.threads", got)
	}
}

func TestVocabularyIsClosed(t *testing.T) {
	inVocabulary := map[Capability]bool{}
	for _, c := range Vocabulary() {
		if inVocabulary[c] {
			t.Errorf("%s listed twice in the vocabulary", c)
		}
		inVocabulary[c] = true
	}
	reached := map[Capability]bool{}
	for op, c := range opCapability {
		if _, ok := StdOpIdOf(op); !ok {
			t.Errorf("opCapability names %s, which is not a std op", op)
		}
		if !inVocabulary[c] {
			t.Errorf("op %s maps to %s, which is not in the vocabulary", op, c)
		}
		reached[c] = true
	}
	for c := range inVocabulary {
		if !reached[c] {
			t.Errorf("%s is in the vocabulary but no op draws on it", c)
		}
		for _, need := range c.Needs() {
			if !inVocabulary[need] {
				t.Errorf("%s needs %s, which is not in the vocabulary", c, need)
			}
		}
	}
}
