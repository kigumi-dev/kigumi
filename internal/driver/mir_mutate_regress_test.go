package driver_test

import (
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/mir"
)

var fixedMutants = []struct{ spec, rule string }{
	{"adt:30425", "cellops"},
	{"adt:31983", "cellops"},
	{"direct_fixed_array:32036", "calls"},
	{"iface_static:32569", "calls"},
	{"direct_aggregate:32561", "stdop"},
	{"mut_self_dispatch:30604", ""},
	{"iface_static:33439", "variants"},
	{"generic_direct_methods:30709", "droplent"},
	{"generics:31239", ""},
	{"direct_fixed_array:32402", "structure"},
	{"direct_closure:61097", "types"},
	{"direct_closure:62170", "types"},
	{"iface_static:62698", "refs"},
	{"direct_adt:61220", "structure"},
	{"mut_self_dispatch:100790", "cellops"},
	{"generics:100118", "refs"},
	{"async:201221", "types"},
	{"async_method:201504", ""},
	{"iface_static:202060", "stdop"},
	{"conform_generic_method_dispatch:301232", "types"},
	{"async_method:302837", "defs"},
	{"mut_self_dispatch:500236", "calls"},
	{"iface_static:500458", "shape"},
	{"direct_closure:600538", "droplent"},
	{"mut_self_dispatch:703770", "cellops"},
	{"generics:800939", "calls"},
	{"generics:900073", "refs"},
	{"direct_closure:905554", "shape"},
	{"generic_direct_methods:906132", "defs"},
	{"generic_direct_methods:1000622", "refs"},
	{"direct_closure:1004262", "aggregates"},
	{"mut_self_dispatch:2005186", "calls"},
	{"mut_self_dispatch:3000216", "cellops"},
	{"direct_closure:2002554", "entities"},
	{"iface_static:3005068", "aggregates"},
	{"generics:4001769", "aggregates"},
	{"adt:4001965", "refs"},
	{"adt:5004915", "refs"},
	{"mut_self_dispatch:4004503", "shape"},
	{"iface_static:4005517", "calls"},
	{"mut_self_dispatch:6004020", "cellops"},
	{"generics:8006032", "refs"},
	{"async_method:6000571", ""},
	{"async_method:20000768", "entry"},
}

func TestFixedMutants(t *testing.T) {
	for _, c := range fixedMutants {
		t.Run(c.spec, func(t *testing.T) {
			name, seedText, _ := strings.Cut(c.spec, ":")
			seed, _ := strconv.ParseInt(seedText, 10, 64)
			base := analyzeFixture(t, fixtureFile(name))
			p, what := mutantOf(base, newMutator(base), seed)
			err := p.Verify()
			if c.rule != "" {
				if ve, ok := err.(*mir.Error); !ok || ve.Rule != c.rule {
					t.Fatalf("%s: got %v, want rule %s", what, err, c.rule)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: rejected: %v", what, err)
			}
			if f := emitMutant(p); f != "" {
				t.Fatalf("%s: %s", what, f)
			}
			if _, f := runMutant(p); f != "" {
				t.Fatalf("%s: %s", what, f)
			}
		})
	}
}
