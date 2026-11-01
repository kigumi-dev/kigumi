package driver_test

import (
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/policy"
)

func TestLinkIdentityViolationsReleaseOnly(t *testing.T) {
	m := &driver.Module{Name: "app", Links: []module.Link{{Library: "kigumi_test_definitely_missing_xyz"}}}
	vs := driver.LinkIdentityViolations(m, buildconfig.Target{})
	if len(vs) != 1 || vs[0].ID != "release-native-link-identity-unknown" {
		t.Fatalf("want one release-native-link-identity-unknown violation, got %+v", vs)
	}
	dev := policy.Evaluate(m.Violations, policy.Development(), vs...)
	if len(dev.Warnings) != 1 || len(dev.Errors) != 0 {
		t.Errorf("development should warn, not fail: %+v", dev)
	}
	rel := policy.Evaluate(m.Violations, policy.Release(), vs...)
	if len(rel.Errors) != 1 {
		t.Errorf("release should refuse an unidentified Link: %+v", rel)
	}

	resolvable := &driver.Module{Name: "app", Links: []module.Link{{Library: "m"}}}
	if vs := driver.LinkIdentityViolations(resolvable, buildconfig.Target{}); len(vs) != 0 {
		t.Errorf("libm should resolve on this host, got %+v", vs)
	}
}
