package hosttool

import (
	"errors"
	"testing"
)

func fakeLookup(found map[string]string) Lookup {
	return func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", errors.New(name + " not found")
	}
}

func TestNoCCompilerEnvHidesToolchain(t *testing.T) {
	t.Setenv("KIGUMI_TEST_NO_CC", "")
	look := fakeLookup(map[string]string{"zig": "/opt/zig", "clang": "/usr/bin/clang"})
	if got := DiscoverToolchain(look); !got.Zig || !got.Clang {
		t.Fatalf("DiscoverToolchain = %+v, want both tools before the gate", got)
	}
	t.Setenv("KIGUMI_TEST_NO_CC", "1")
	if got := DiscoverToolchain(look); got.Zig || got.Clang {
		t.Fatalf("DiscoverToolchain = %+v, want none under KIGUMI_TEST_NO_CC", got)
	}
	if name, _, ok := CCompiler(look); ok {
		t.Fatalf("CCompiler named %q under KIGUMI_TEST_NO_CC", name)
	}
}
