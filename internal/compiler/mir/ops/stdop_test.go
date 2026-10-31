package ops

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
		{"alloc.Shared.tryNew", KindPrimitive},
		{"alloc.Box.tryNew", KindPrimitive},
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

func TestPanickingFormsResolveToNoOp(t *testing.T) {
	for _, key := range []string{
		"prelude.Bytes.slice", "prelude.Bytes.zeros", "prelude.Bytes.fill", "prelude.Bytes.fromArray", "prelude.Bytes.concat",
		"prelude.String.sliceBytes", "array.Array.clone", "alloc.Shared.new", "alloc.Shared.get", "alloc.Shared.downgrade",
		"alloc.Box.new",
	} {
		if _, ok := StdOpFor(key); ok {
			t.Errorf("%s resolves to an op; it is a Kigumi body over its try form", key)
		}
	}
}
