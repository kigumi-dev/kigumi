package vm

import (
	"reflect"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

// TestHostForBuildKeysStayDistinct checks hostFor doesn't let hostByOp conflate build.* keys, which all share one BUILD_ONLY id.
func TestHostForBuildKeysStayDistinct(t *testing.T) {
	m := &Machine{host: map[string]hostFn{}}
	m.registerAll()

	id, ok := opdef.StdOpFor("build.Builder.file")
	if !ok {
		t.Fatal("build.Builder.file: not in the registry")
	}
	prov := opdef.ProviderOpId(id) + 1

	fileFn, ok := m.hostFor(prov, "build.Builder.file")
	if !ok {
		t.Fatal("build.Builder.file: no host function")
	}
	execFn, ok := m.hostFor(prov, "build.Builder.executable")
	if !ok {
		t.Fatal("build.Builder.executable: no host function")
	}
	if reflect.ValueOf(fileFn).Pointer() == reflect.ValueOf(execFn).Pointer() {
		t.Error("build.Builder.file and build.Builder.executable share BUILD_ONLY's id but must not share a host function")
	}
	if _, indexed := m.hostByOp[prov]; indexed {
		t.Error("hostByOp must not index BUILD_ONLY: every build.* key collides on it")
	}
}
