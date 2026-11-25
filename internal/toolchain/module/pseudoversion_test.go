package module_test

import (
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/module"
)

const (
	pseudoA = "v0.0.0-20261123010000-123456789abc"
	pseudoB = "v1.2.4-0.20261123010000-123456789abc"
	pseudoC = "v1.3.0-rc.1.0.20261123010000-123456789abc"
)

var pseudoTime = time.Date(2026, 11, 23, 1, 0, 0, 0, time.UTC)

func commitFromShort(t *testing.T, prefix string) module.CommitID {
	t.Helper()
	if len(prefix) >= 40 {
		t.Fatalf("prefix %q is too long", prefix)
	}
	c, err := module.ParseFullCommitHex(prefix + strings.Repeat("0", 40-len(prefix)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func commitWithSuffix(t *testing.T, prefix, suffix string) module.CommitID {
	t.Helper()
	if len(prefix)+len(suffix) > 40 {
		t.Fatalf("commit parts are too long")
	}
	c, err := module.ParseFullCommitHex(prefix + strings.Repeat("0", 40-len(prefix)-len(suffix)) + suffix)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustPseudo(t *testing.T, v string) module.PseudoVersion {
	t.Helper()
	p, err := module.ParsePseudoVersion(v)
	if err != nil {
		t.Fatalf("ParsePseudoVersion(%q): %v", v, err)
	}
	return p
}

func TestParsePseudoVersionForms(t *testing.T) {
	noBase := mustPseudo(t, pseudoA)
	if noBase.Base != nil {
		t.Errorf("no-base pseudo has base %s", noBase.Base.String())
	}
	if noBase.Rev != "123456789abc" {
		t.Errorf("rev = %q", noBase.Rev)
	}
	if !noBase.Time.Equal(pseudoTime) {
		t.Errorf("time = %s, want %s", noBase.Time, pseudoTime)
	}

	release := mustPseudo(t, pseudoB)
	if release.Base == nil || release.Base.String() != "v1.2.3" {
		t.Fatalf("release base = %v, want v1.2.3", release.Base)
	}
	if release.Version.String() != pseudoB {
		t.Errorf("canonical = %q", release.Version.String())
	}

	pre := mustPseudo(t, pseudoC)
	if pre.Base == nil || pre.Base.String() != "v1.3.0-rc.1" {
		t.Fatalf("prerelease base = %v, want v1.3.0-rc.1", pre.Base)
	}
}

func TestParsePseudoVersionBuildMetadata(t *testing.T) {
	v := pseudoB + "+incompatible"
	p := mustPseudo(t, v)
	if p.Build != "incompatible" {
		t.Errorf("build = %q", p.Build)
	}
	if p.Version.String() != pseudoB {
		t.Errorf("canonical = %q, want %q", p.Version.String(), pseudoB)
	}
	if p.String() != v {
		t.Errorf("String() = %q, want %q", p.String(), v)
	}
}

func TestParsePseudoVersionInvalid(t *testing.T) {
	tests := []struct {
		version string
		code    module.RevisionErrorCode
	}{
		{"v1.2.3", module.ErrRevisionSyntax},
		{"not-a-version", module.ErrRevisionSyntax},
		{"v1.2.4-1.20261123010000-123456789abc", module.ErrRevisionSyntax},
		{"v1.2.4-0.20261301000000-123456789abc", module.ErrRevisionPseudoTime},
		{"v1.2.4-0.20260229000000-123456789abc", module.ErrRevisionPseudoTime},
		{"v1.2.4-0.2026112301000-123456789abc", module.ErrRevisionSyntax},
		{"v1.2.4-0.20261123010000-123456789ab", module.ErrRevisionHash},
		{"v1.2.4-0.20261123010000-123456789ABC", module.ErrRevisionHash},
		{"v1.2.4-0.20261123010000-xyzxyzxyzxyz", module.ErrRevisionHash},
	}
	for _, tt := range tests {
		_, err := module.ParsePseudoVersion(tt.version)
		if err == nil {
			t.Errorf("ParsePseudoVersion(%q) succeeded, want %s", tt.version, tt.code)
			continue
		}
		if got := revisionCode(t, err); got != tt.code {
			t.Errorf("ParsePseudoVersion(%q) code = %s, want %s", tt.version, got, tt.code)
		}
	}
}

func TestNewPseudoVersion(t *testing.T) {
	sha := commitWithSuffix(t, "123456789abc", "d")
	tests := []struct {
		name string
		base string
		want string
	}{
		{"github.com/acme/mod", "", "v0.0.0-20261123010000-123456789abc"},
		{"github.com/acme/mod/v2", "", "v2.0.0-20261123010000-123456789abc"},
		{"github.com/acme/mod", "v1.2.3", "v1.2.4-0.20261123010000-123456789abc"},
		{"github.com/acme/mod/v2", "v2.4.0", "v2.4.1-0.20261123010000-123456789abc"},
		{"github.com/acme/mod", "v1.3.0-rc.1", "v1.3.0-rc.1.0.20261123010000-123456789abc"},
	}
	for _, tt := range tests {
		var base *module.CanonicalVersion
		if tt.base != "" {
			b := mustRevisionCanonical(t, tt.base)
			base = &b
		}
		got, err := module.NewPseudoVersion(tt.name, base, pseudoTime, sha)
		if err != nil {
			t.Errorf("NewPseudoVersion(%s, %v): %v", tt.name, tt.base, err)
			continue
		}
		if got.String() != tt.want {
			t.Errorf("NewPseudoVersion(%s, %v) = %s, want %s", tt.name, tt.base, got.String(), tt.want)
		}
	}
}

func TestNewPseudoVersionTruncatesToWholeSeconds(t *testing.T) {
	sha := commitWithSuffix(t, "123456789abc", "e")
	withFraction := time.Date(2026, 11, 23, 1, 0, 0, 750_000_000, time.UTC)
	got, err := module.NewPseudoVersion("github.com/acme/mod", nil, withFraction, sha)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "v0.0.0-20261123010000-123456789abc" {
		t.Fatalf("got %s", got.String())
	}
}

func TestNewPseudoVersionOverflowAndDatetime(t *testing.T) {
	sha := commitWithSuffix(t, "123456789abc", "f")
	maxPatch := mustRevisionCanonical(t, "v1.2.9223372036854775807")
	_, err := module.NewPseudoVersion("github.com/acme/mod", &maxPatch, pseudoTime, sha)
	if err == nil {
		t.Fatal("patch overflow was accepted")
	}
	if got := revisionCode(t, err); got != module.ErrRevisionOverflow {
		t.Fatalf("overflow code = %s", got)
	}

	for _, at := range []time.Time{
		time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		_, err := module.NewPseudoVersion("github.com/acme/mod", nil, at, sha)
		if err == nil {
			t.Fatalf("year %d was accepted", at.Year())
		}
		if got := revisionCode(t, err); got != module.ErrRevisionPseudoTime {
			t.Fatalf("datetime code = %s, want %s", got, module.ErrRevisionPseudoTime)
		}
	}
}

func TestNewPseudoVersionMajorCompatibility(t *testing.T) {
	sha := commitWithSuffix(t, "123456789abc", "a")
	baseV2 := mustRevisionCanonical(t, "v2.0.0")
	if _, err := module.NewPseudoVersion("github.com/acme/mod", &baseV2, pseudoTime, sha); err == nil {
		t.Fatal("v2 base was accepted for an unsuffixed module path")
	} else if got := revisionCode(t, err); got != module.ErrRevisionMajorMismatch {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionMajorMismatch)
	}
	baseV1 := mustRevisionCanonical(t, "v1.2.3")
	if _, err := module.NewPseudoVersion("github.com/acme/mod/v2", &baseV1, pseudoTime, sha); err == nil {
		t.Fatal("v1 base was accepted for a /v2 module path")
	} else if got := revisionCode(t, err); got != module.ErrRevisionMajorMismatch {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionMajorMismatch)
	}
}

func TestValidatePseudoVersion(t *testing.T) {
	commit := commitWithSuffix(t, "123456789abc", "b")
	baseCommit := commitWithSuffix(t, "abcdef123456", "c")
	baseVersion := mustRevisionCanonical(t, "v1.2.3")
	p := mustPseudo(t, pseudoB)

	if err := module.ValidatePseudoVersion("github.com/acme/mod", p, commit, &module.RevisionBase{
		Version: baseVersion, Tag: "v1.2.3", Commit: baseCommit,
	}); err != nil {
		t.Fatalf("valid evidence rejected: %v", err)
	}

	noBase := mustPseudo(t, pseudoA)
	if err := module.ValidatePseudoVersion("github.com/acme/mod", noBase, commit, nil); err != nil {
		t.Fatalf("valid no-base evidence rejected: %v", err)
	}
	if err := module.ValidatePseudoVersion("github.com/acme/mod", noBase, commit, &module.RevisionBase{
		Version: baseVersion, Tag: "v1.2.3", Commit: baseCommit,
	}); err == nil {
		t.Fatal("no-base pseudo accepted base evidence")
	}

	wrongCommit := commitWithSuffix(t, "abcdef123456", "d")
	if err := module.ValidatePseudoVersion("github.com/acme/mod", p, wrongCommit, &module.RevisionBase{
		Version: baseVersion, Tag: "v1.2.3", Commit: baseCommit,
	}); err == nil {
		t.Fatal("mismatched commit suffix was accepted")
	} else if got := revisionCode(t, err); got != module.ErrRevisionHash {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionHash)
	}

	if err := module.ValidatePseudoVersion("github.com/acme/mod", p, commit, nil); err == nil {
		t.Fatal("based pseudo accepted without base evidence")
	}

	otherBase := mustRevisionCanonical(t, "v1.2.2")
	if err := module.ValidatePseudoVersion("github.com/acme/mod", p, commit, &module.RevisionBase{
		Version: otherBase, Tag: "v1.2.2", Commit: baseCommit,
	}); err == nil {
		t.Fatal("wrong base version was accepted")
	} else if got := revisionCode(t, err); got != module.ErrRevisionPseudoBase {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionPseudoBase)
	}

	if err := module.ValidatePseudoVersion("github.com/acme/mod", p, commit, &module.RevisionBase{
		Version: baseVersion, Tag: commit.String(), Commit: baseCommit,
	}); err == nil {
		t.Fatal("commit id accepted as a base tag")
	} else if got := revisionCode(t, err); got != module.ErrRevisionPseudoBase {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionPseudoBase)
	}
}

func TestValidatePseudoVersionMajorMismatch(t *testing.T) {
	commit := commitWithSuffix(t, "123456789abc", "e")
	baseVersion := mustRevisionCanonical(t, "v1.2.3")
	base := &module.RevisionBase{Version: baseVersion, Tag: "v1.2.3", Commit: commit}
	p := mustPseudo(t, pseudoB)
	for _, name := range []string{"github.com/acme/mod/v2"} {
		err := module.ValidatePseudoVersion(name, p, commit, base)
		if err == nil {
			t.Fatalf("%s accepted a v1 pseudo-version", name)
		}
		if got := revisionCode(t, err); got != module.ErrRevisionMajorMismatch {
			t.Fatalf("code = %s, want %s", got, module.ErrRevisionMajorMismatch)
		}
	}
}

func TestParseRequestVersion(t *testing.T) {
	tests := []struct {
		version string
		kind    module.RequestVersionKind
	}{
		{pseudoA, module.RequestVersionPseudo},
		{pseudoB, module.RequestVersionPseudo},
		{"v1.2.3", module.RequestVersionTag},
		{"v1.2.3+meta", module.RequestVersionTag},
		{"123456789abcdef", module.RequestVersionCommit},
	}
	for _, tt := range tests {
		got, err := module.ParseRequestVersion(tt.version)
		if err != nil {
			t.Errorf("ParseRequestVersion(%q): %v", tt.version, err)
			continue
		}
		if got.Kind != tt.kind {
			t.Errorf("ParseRequestVersion(%q).Kind = %s, want %s", tt.version, got.Kind, tt.kind)
		}
	}
	if _, err := module.ParseRequestVersion("v1.2"); err == nil {
		t.Fatal("abbreviated version was accepted")
	}
	if _, err := module.ParseRequestVersion("123456"); err == nil {
		t.Fatal("short hex below 7 was accepted as a commit")
	}
}

func TestBuildMetadataTagPseudoGeneration(t *testing.T) {
	sha := commitWithSuffix(t, "123456789abc", "f")
	tests := []struct {
		tag  string
		want string
	}{
		{"v1.2.3+meta", "v1.2.4-0.20261123010000-123456789abc"},
		{"v1.3.0-rc.1+incompatible", "v1.3.0-rc.1.0.20261123010000-123456789abc"},
	}
	for _, tt := range tests {
		tag, err := module.ParseVersionTag(tt.tag)
		if err != nil {
			t.Fatalf("ParseVersionTag(%q): %v", tt.tag, err)
		}
		got, err := module.NewPseudoVersion("github.com/acme/mod", &tag.Version, pseudoTime, sha)
		if err != nil {
			t.Fatalf("NewPseudoVersion from %q: %v", tt.tag, err)
		}
		if got.String() != tt.want {
			t.Errorf("pseudo for %q = %s, want %s", tt.tag, got.String(), tt.want)
		}
	}
}

func TestParsePseudoVersionNeverPanics(t *testing.T) {
	for _, v := range []string{
		"v0.0.0-00000000000000-000000000000",
		"v0.0.0-99999999999999-000000000000",
		"v1.2.3-0.00000000000000-abcdef123456",
		"v1.2.3-0.20261123010000-abcdef123456+",
		"v1.2.3-0.20261123010000-abcdef123456+incompatible",
		"v1.2.3-0.20261123010000-abcdef123456.extra",
		"v1.2.3-0.20261123010000-abcdef1234567890",
		"v1.2.3-0.20261123010000",
		"v1.2.3-0",
		"v1.2.3-0.",
		"v1.2.3-0.20261123010000-",
	} {
		module.ParsePseudoVersion(v)
		module.ParseRequestVersion(v)
	}
}
