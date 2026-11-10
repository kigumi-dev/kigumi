package runtimegen

import (
	"strings"
	"testing"
)

// testOps is a two-op registry whose bodies live in testBodies.
var testOps = []Op{
	{ID: 0, Name: "A", Keys: []string{"a.key"}},
	{ID: 1, Name: "B", Keys: []string{"b.key"}},
}

const testBodies = `@section prelude
@op A
 return a;
@op B
 return b;
`

func testVM() map[string]VMImpl {
	return map[string]VMImpl{
		"A": {Kind: VMStd, Symbol: "std"},
		"B": {Kind: VMHost, Symbol: "registerHost"},
	}
}

func TestRuntimeManifestRenders(t *testing.T) {
	ops := []Op{
		{ID: 0, Name: "A", Keys: []string{"a.key"}, Cap: "host.shell", CapMacros: []string{"KIGUMI_CAP_HOST_SHELL"}},
		{ID: 1, Name: "B", Retired: true},
		{ID: 2, Name: "C", Provider: true, Keys: []string{"c.key"}, Suffixes: []string{"c"}},
	}
	bodies := "@section prelude\n@cap KIGUMI_CAP_HOST_SHELL host.shell\n@arm A\n return a;\n@stub A\n@endcap\n@op C\n return c;\n"
	vm := map[string]VMImpl{
		"A": {Kind: VMHost, Symbol: "registerShell"},
		"C": {Kind: VMGap, Reason: "the VM cannot reach it"},
	}
	got, err := runtimeManifest(ops, vm, bodies)
	if err != nil {
		t.Fatal(err)
	}
	want := manifestHeader +
		"op 0 A primitive cap=host.shell c=rt_std_dispatch.inc:prelude guard=KIGUMI_CAP_HOST_SHELL vm=host:registerShell keys=a.key\n" +
		"op 1 B retired\n" +
		"op 2 C provider cap=- c=rt_std_dispatch.inc:prelude guard=- vm=gap:- reason=\"the VM cannot reach it\" keys=c.key suffixes=c\n"
	if got != want {
		t.Fatalf("manifest =\n%s\nwant\n%s", got, want)
	}
}

func TestRuntimeManifestRejects(t *testing.T) {
	ops := func() []Op { return append([]Op(nil), testOps...) }
	cases := []struct {
		name   string
		ops    []Op
		vm     map[string]VMImpl
		bodies string
		want   string
	}{
		{"missing body", ops(), testVM(), "@op A\n return a;\n", "no body for live op(s) B"},
		{"retired body", append(ops(), Op{ID: 2, Name: "C", Retired: true}), testVM(), testBodies + "@op C\n return c;\n", "op C is not a live registry op"},
		{"no std key", []Op{{ID: 0, Name: "A"}}, map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}}, "@op A\n return a;\n", "has no key, suffix or prefix"},
		{"missing VM", ops(), map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}}, testBodies, "op B has no VM implementation"},
		{"unknown VM", ops(), map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}, "B": {Kind: VMStd, Symbol: "std"}, "Z": {Kind: VMStd, Symbol: "std"}}, testBodies, "VM op table names Z"},
		{"unknown VM kind", ops(), map[string]VMImpl{"A": {Kind: "elsewhere", Symbol: "std"}, "B": {Kind: VMStd, Symbol: "std"}}, testBodies, `"elsewhere" is not a VM implementation kind`},
		{"gap without reason", ops(), map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}, "B": {Kind: VMGap}}, testBodies, "needs the reason"},
		{"gap with symbol", ops(), map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}, "B": {Kind: VMGap, Symbol: "std", Reason: "x"}}, testBodies, "names no function"},
		{"host without symbol", ops(), map[string]VMImpl{"A": {Kind: VMHost}, "B": {Kind: VMStd, Symbol: "std"}}, testBodies, "needs the VM function"},
		{"reason on an implementation", ops(), map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std", Reason: "x"}, "B": {Kind: VMStd, Symbol: "std"}}, testBodies, "only a gap carries a reason"},
		{"capability unguarded", []Op{{ID: 0, Name: "A", Keys: []string{"a.key"}, Cap: "host.shell", CapMacros: []string{"KIGUMI_CAP_HOST_SHELL"}}}, map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}}, "@op A\n return a;\n", "guarded by none of KIGUMI_CAP_HOST_SHELL"},
		{"guard without capability", []Op{{ID: 0, Name: "A", Keys: []string{"a.key"}}}, map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}}, "@cap KIGUMI_CAP_HOST_SHELL host.shell\n@arm A\n return a;\n@stub A\n@endcap\n", "but the registry gives it no capability"},
		{"inline guard without capability", []Op{{ID: 0, Name: "A", Keys: []string{"a.key"}}}, map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}}, "@op A\n\n#if defined(KIGUMI_CAP_HOST_SHELL)\n return a;\n#endif\n", "but the registry gives it no capability"},
		{"declared capability mismatch", []Op{{ID: 0, Name: "A", Keys: []string{"a.key"}, Cap: "ffi.raw", CapMacros: []string{"KIGUMI_CAP_FFI_RAW"}}}, map[string]VMImpl{"A": {Kind: VMStd, Symbol: "std"}}, "@cap KIGUMI_CAP_HOST_SHELL host.shell\n@arm A\n return a;\n@stub A\n@endcap\n", "guarded by none of KIGUMI_CAP_FFI_RAW"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := runtimeManifest(c.ops, c.vm, c.bodies)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error mentioning %q", err, c.want)
			}
		})
	}
}

// The capability a body is guarded by may be one the op's capability needs, as
// task.threads needs task.executor.
func TestRuntimeManifestAcceptsNeededCapabilityGuard(t *testing.T) {
	ops := []Op{{ID: 0, Name: "A", Keys: []string{"a.key"}, Cap: "task.threads", CapMacros: []string{"KIGUMI_CAP_TASK_THREADS", "KIGUMI_CAP_TASK_EXECUTOR"}}}
	vm := map[string]VMImpl{"A": {Kind: VMHost, Symbol: "registerTask"}}
	bodies := "@cap KIGUMI_CAP_TASK_EXECUTOR task.executor\n@arm A\n return a;\n@stub A\n@endcap\n"
	got, err := runtimeManifest(ops, vm, bodies)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "c=rt_std_dispatch.inc:- guard=KIGUMI_CAP_TASK_EXECUTOR vm=host:registerTask") {
		t.Fatalf("manifest does not name the needed capability guard:\n%s", got)
	}
}

// OpBodies records where a body sits and which macro guards it, including a
// body guarding itself outside a @cap group.
func TestOpBodies(t *testing.T) {
	ops := []Op{
		{ID: 0, Name: "A", Keys: []string{"a.key"}, Cap: "host.shell", CapMacros: []string{"KIGUMI_CAP_HOST_SHELL"}},
		{ID: 1, Name: "B", Keys: []string{"b.key"}, Cap: "ffi.raw", CapMacros: []string{"KIGUMI_CAP_FFI_RAW"}},
	}
	bodies := "@section prelude\n@cap KIGUMI_CAP_HOST_SHELL host.shell\n@arm A\n return a;\n@stub A\n@endcap\n@op B\n\n#if defined(KIGUMI_CAP_FFI_RAW)\n return b;\n#endif\n"
	index, err := opBodies(ops, bodies)
	if err != nil {
		t.Fatal(err)
	}
	a, b := index["A"], index["B"]
	if a.Section != "prelude" || a.GuardCap != "host.shell" || a.GuardMacro != "KIGUMI_CAP_HOST_SHELL" || len(a.Macros) != 0 {
		t.Fatalf("A body = %+v", a)
	}
	if b.GuardMacro != "" || len(b.Macros) != 1 || b.Macros[0] != "KIGUMI_CAP_FFI_RAW" {
		t.Fatalf("B body = %+v", b)
	}
}
