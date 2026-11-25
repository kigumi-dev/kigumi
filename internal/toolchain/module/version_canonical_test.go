package module_test

import (
	"errors"
	"testing"

	"kigumi/internal/toolchain/module"
)

func mustRevisionCanonical(t *testing.T, v string) module.CanonicalVersion {
	t.Helper()
	cv, err := module.ParseCanonicalVersion(v)
	if err != nil {
		t.Fatalf("ParseCanonicalVersion(%q): %v", v, err)
	}
	return cv
}

func revisionCode(t *testing.T, err error) module.RevisionErrorCode {
	t.Helper()
	var re *module.RevisionError
	if !errors.As(err, &re) {
		t.Fatalf("error %v is not a *RevisionError", err)
	}
	return re.Code
}

func TestParseCanonicalVersionValid(t *testing.T) {
	for _, v := range []string{
		"v0.0.0",
		"v1.2.3",
		"v10.20.30",
		"v1.3.0-rc.1",
		"v1.0.0-alpha",
		"v1.0.0-alpha.1",
		"v1.0.0-0",
		"v0.0.0-20261123010000-123456789abc",
		"v1.2.4-0.20261123010000-123456789abc",
		"v1.3.0-rc.1.0.20261123010000-123456789abc",
	} {
		cv, err := module.ParseCanonicalVersion(v)
		if err != nil {
			t.Errorf("ParseCanonicalVersion(%q): %v", v, err)
			continue
		}
		if cv.String() != v {
			t.Errorf("ParseCanonicalVersion(%q).String() = %q", v, cv.String())
		}
	}
}

func TestParseCanonicalVersionInvalid(t *testing.T) {
	tests := []struct {
		version string
		code    module.RevisionErrorCode
	}{
		{"", module.ErrRevisionSyntax},
		{"1.2.3", module.ErrRevisionSyntax},
		{"v1", module.ErrRevisionSyntax},
		{"v1.2", module.ErrRevisionSyntax},
		{"v1.2.3.4", module.ErrRevisionSyntax},
		{"v1.2.3-", module.ErrRevisionSyntax},
		{"v1.2.3+", module.ErrRevisionSyntax},
		{"v1.2.3-alpha..1", module.ErrRevisionSyntax},
		{"v1.2.3-01", module.ErrRevisionSyntax},
		{"v1.2.3-rc.01", module.ErrRevisionSyntax},
		{"v1.2.3+build", module.ErrRevisionSyntax},
		{"v1.2.3+build.1", module.ErrRevisionSyntax},
		{"v9223372036854775808.0.0", module.ErrRevisionOverflow},
		{"v1.9223372036854775808.0", module.ErrRevisionOverflow},
		{"v1.2.9223372036854775808", module.ErrRevisionOverflow},
	}
	for _, tt := range tests {
		_, err := module.ParseCanonicalVersion(tt.version)
		if err == nil {
			t.Errorf("ParseCanonicalVersion(%q) succeeded, want %s", tt.version, tt.code)
			continue
		}
		if got := revisionCode(t, err); got != tt.code {
			t.Errorf("ParseCanonicalVersion(%q) code = %s, want %s", tt.version, got, tt.code)
		}
	}
}

func TestVersionTagBuildMetadata(t *testing.T) {
	for _, v := range []string{"v1.2.3+build", "v1.2.3+meta.1", "v1.2.3-rc.1+sha.abcdef"} {
		tag, err := module.ParseVersionTag(v)
		if err != nil {
			t.Fatalf("ParseVersionTag(%q): %v", v, err)
		}
		if tag.String() != v {
			t.Errorf("ParseVersionTag(%q).String() = %q", v, tag.String())
		}
		if tag.Version.String() == v {
			t.Errorf("ParseVersionTag(%q) kept build metadata in the canonical version", v)
		}
	}
	tag, err := module.ParseVersionTag("v1.2.3+build")
	if err != nil {
		t.Fatal(err)
	}
	if tag.Version.String() != "v1.2.3" || tag.Build != "build" {
		t.Fatalf("VersionTag = %q build %q", tag.Version.String(), tag.Build)
	}
}

func TestCanonicalBuildMetadataDoesNotTieDifferentTags(t *testing.T) {
	a, err := module.ParseVersionTag("v1.2.3+a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := module.ParseVersionTag("v1.2.3+b")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Version.Equal(b.Version) {
		t.Fatal("canonical versions of two build-metadata tags must have equal precedence")
	}
	if a.String() == b.String() {
		t.Fatal("the full tags must stay distinguishable")
	}
}

func TestCanonicalComparisonOrder(t *testing.T) {
	order := []string{
		"v0.0.0-20261123010000-123456789abc",
		"v0.0.0-20261123020000-abcdef123456",
		"v0.1.0",
		"v0.1.1-0.20261123020000-abcdef123456",
		"v0.1.1",
		"v1.0.0-alpha",
		"v1.0.0-alpha.1",
		"v1.0.0-alpha.beta",
		"v1.0.0-beta",
		"v1.0.0-beta.2",
		"v1.0.0-beta.11",
		"v1.0.0-rc.1",
		"v1.0.0",
		"v1.0.1",
		"v1.1.0",
	}
	vs := make([]module.CanonicalVersion, len(order))
	for i, s := range order {
		vs[i] = mustRevisionCanonical(t, s)
	}
	for i := range vs {
		for j := range vs {
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got := vs[i].Compare(vs[j]); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", order[i], order[j], got, want)
			}
		}
	}
}

func TestCanonicalComparisonProperties(t *testing.T) {
	versions := []string{
		"v0.0.0-20261123010000-123456789abc",
		"v0.1.0",
		"v1.0.0-alpha",
		"v1.0.0-alpha.1",
		"v1.0.0",
		"v2.0.0",
	}
	vs := make([]module.CanonicalVersion, len(versions))
	for i, s := range versions {
		vs[i] = mustRevisionCanonical(t, s)
	}
	for _, a := range vs {
		if a.Compare(a) != 0 {
			t.Errorf("%s is not equal to itself", a.String())
		}
		for _, b := range vs {
			if a.Compare(b) != -b.Compare(a) {
				t.Errorf("Compare is not antisymmetric for %s and %s", a.String(), b.String())
			}
			for _, c := range vs {
				if a.Compare(b) <= 0 && b.Compare(c) <= 0 && a.Compare(c) > 0 {
					t.Errorf("Compare is not transitive for %s, %s, %s", a.String(), b.String(), c.String())
				}
			}
		}
	}
}

func TestZeroCanonicalVersionSortsFirst(t *testing.T) {
	var zero module.CanonicalVersion
	v := mustRevisionCanonical(t, "v0.0.0")
	if !zero.IsZero() {
		t.Fatal("zero value is not zero")
	}
	if zero.Compare(v) >= 0 || v.Compare(zero) <= 0 {
		t.Fatal("zero value must sort before parsed versions")
	}
}

func TestCanonicalVersionComparable(t *testing.T) {
	a := mustRevisionCanonical(t, "v1.2.3-rc.1")
	b := mustRevisionCanonical(t, "v1.2.3-rc.1")
	m := map[module.CanonicalVersion]bool{a: true}
	if !m[b] {
		t.Fatal("equal canonical versions must be usable as one map key")
	}
	if a != b {
		t.Fatal("equal canonical versions must compare equal with ==")
	}
}
