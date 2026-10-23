package mir

import "testing"

func TestStdOpRegistryConsistent(t *testing.T) {
	names := map[string]bool{}
	for _, n := range StdOpNames {
		if names[n] {
			t.Errorf("StdOpNames: %q appears twice", n)
		}
		names[n] = true
	}
	for key, name := range stdOpKeys {
		if !names[name] {
			t.Errorf("stdOpKeys[%q] = %q: not in StdOpNames", key, name)
		}
	}
	for method, name := range stdOpSuffixes {
		if !names[name] {
			t.Errorf("stdOpSuffixes[%q] = %q: not in StdOpNames", method, name)
		}
	}
	for name := range providerOps {
		if !names[name] {
			t.Errorf("providerOps: %q not in StdOpNames", name)
		}
	}
}

func TestResolveStdOpKind(t *testing.T) {
	cases := []struct {
		key  string
		kind OpKind
	}{
		{"prelude.String.len", KindPrimitive},
		{"array.Array.push", KindPrimitive},
		{"math.sqrt", KindPrimitive},
		{"alloc.Arena.create", KindPrimitive},
		{"alloc.Shared.new", KindPrimitive},
		{"alloc.Box.new", KindPrimitive},
		{"alloc.Box.into", KindPrimitive},
		{"crypto/subtle.constantTimeEq", KindPrimitive},
		{"os.Host.files", KindProvider},
		{"net.Net.http", KindProvider},
		{"task.Executor.runToCompletion", KindProvider},
		{"ffi.Pin.new", KindProvider},
		{"dl.Symbol.call", KindProvider},
		{"build.Builder.request", KindProvider},
	}
	for _, c := range cases {
		id, ok := StdOpFor(c.key)
		if !ok {
			t.Errorf("%s: not found in the registry", c.key)
			continue
		}
		got := KindPrimitive
		if providerOps[StdOpNames[id]] {
			got = KindProvider
		}
		if got != c.kind {
			t.Errorf("%s: kind = %v, want %v", c.key, got, c.kind)
		}
	}
}

// The per-op tables are built from the registry at package init, in dependency order, so each one is populated before anything reads it.
func TestRegistryTablesArePopulated(t *testing.T) {
	run, _ := StdOpIdOf("BLOCKING_POOL_RUN")
	if refs := nativeRecordTypesForOp[run]; len(refs) != 1 || refs[0].name != "BlockingError" {
		t.Errorf("BLOCKING_POOL_RUN builds %v by name, want BlockingError", refs)
	}
	if !RuntimeNamedTypes()["Entry"] || !RuntimeNamedTypes()["StringBuilder"] {
		t.Error("the types the try primitives build by name are missing from RuntimeNamedTypes")
	}
	elems := 0
	for _, isElem := range elemOps {
		if isElem {
			elems++
		}
	}
	if len(directResultOps) != 20 || len(hoCallbackArg) != 12 || elems < 3 {
		t.Errorf("tables hold %d direct ops, %d callbacks, %d element ops", len(directResultOps), len(hoCallbackArg), elems)
	}
}
