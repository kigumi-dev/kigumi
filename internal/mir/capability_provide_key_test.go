package mir

import (
	"reflect"
	"testing"
)

func TestProvisionKeyEqualImpliesSameVerdicts(t *testing.T) {
	oses := []string{"linux", "darwin", "freebsd", "netbsd", "openbsd", "windows", "wasi", "freestanding", "uefi"}
	var all []Platform
	for _, os := range oses {
		for _, bare := range []bool{false, true} {
			for _, noLibc := range []bool{false, true} {
				for _, alloc := range []string{"", "none"} {
					for _, rec := range []bool{false, true} {
						for _, prof := range []string{"", "isr"} {
							all = append(all, Platform{OS: os, Bare: bare, NoLibc: noLibc, Allocator: alloc, RecoverableOnly: rec, ThreadProfile: prof})
						}
					}
				}
			}
		}
	}
	table := func(p Platform) []Provision {
		var out []Provision
		for _, c := range Vocabulary() {
			out = append(out, p.Provides(c))
		}
		return out
	}
	byKey := map[string]Platform{}
	shared := 0
	for _, p := range all {
		k := p.ProvisionKey()
		first, ok := byKey[k]
		if !ok {
			byKey[k] = p
			continue
		}
		shared++
		if !reflect.DeepEqual(table(first), table(p)) {
			t.Fatalf("%+v and %+v share key %q but differ", first, p, k)
		}
	}
	if shared == 0 {
		t.Fatal("no two platforms shared a key")
	}
}
