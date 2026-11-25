package module_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/module"
)

func sha1Hex(t *testing.T, prefix string) module.CommitID {
	t.Helper()
	return commitFromShort(t, prefix)
}

func TestParseCommitID(t *testing.T) {
	sha1 := sha1Hex(t, "123456789abc")
	if sha1.String() != "sha1:"+sha1.Hex {
		t.Fatalf("String() = %q", sha1.String())
	}
	if sha1.Short() != "123456789abc" {
		t.Fatalf("Short() = %q", sha1.Short())
	}
	parsed, err := module.ParseCommitID(sha1.String())
	if err != nil {
		t.Fatal(err)
	}
	if parsed != sha1 {
		t.Fatalf("round trip = %#v, want %#v", parsed, sha1)
	}

	sha256Hex := strings.Repeat("ab", 32)
	sha256, err := module.ParseCommitID("sha256:" + sha256Hex)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Algorithm != module.CommitSHA256 || sha256.Hex != sha256Hex {
		t.Fatalf("sha256 parse = %#v", sha256)
	}

	for _, bad := range []string{
		"",
		sha1.Hex,
		"sha1:" + strings.ToUpper(sha1.Hex),
		"sha1:" + sha1.Hex[:39],
		"sha1:" + sha1.Hex + "0",
		"sha256:" + sha1.Hex,
		"md5:" + sha1.Hex,
	} {
		if _, err := module.ParseCommitID(bad); err == nil {
			t.Errorf("ParseCommitID(%q) succeeded", bad)
		} else if got := revisionCode(t, err); got != module.ErrRevisionHash {
			t.Errorf("ParseCommitID(%q) code = %s, want %s", bad, got, module.ErrRevisionHash)
		}
	}
}

func TestParseFullCommitHex(t *testing.T) {
	for _, hex := range []string{
		strings.Repeat("0", 40),
		strings.Repeat("a", 40),
		strings.Repeat("f", 64),
	} {
		if _, err := module.ParseFullCommitHex(hex); err != nil {
			t.Errorf("ParseFullCommitHex(%q): %v", hex, err)
		}
	}
	for _, bad := range []string{"", strings.Repeat("a", 39), strings.Repeat("a", 41), strings.Repeat("a", 63), strings.Repeat("A", 40), strings.Repeat("g", 40)} {
		if _, err := module.ParseFullCommitHex(bad); err == nil {
			t.Errorf("ParseFullCommitHex(%q) succeeded", bad)
		}
	}
}

func TestMatchCommitPrefix(t *testing.T) {
	a := commitWithSuffix(t, "123456789abc", "1")
	b := commitWithSuffix(t, "123456789abc", "2")
	other := commitWithSuffix(t, "abcdef123456", "3")

	got, err := module.MatchCommitPrefix("abcdef1", []module.CommitID{a, b, other})
	if err != nil {
		t.Fatal(err)
	}
	if got != other {
		t.Fatalf("matched %#v, want %#v", got, other)
	}

	if _, err := module.MatchCommitPrefix("123456789abc", []module.CommitID{a, b, other}); err == nil {
		t.Fatal("ambiguous 12-digit prefix was accepted")
	} else if got := revisionCode(t, err); got != module.ErrRevisionAmbiguous {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionAmbiguous)
	}

	if _, err := module.MatchCommitPrefix(a.Hex, []module.CommitID{a, b, other}); err != nil {
		t.Fatalf("disambiguated prefix failed: %v", err)
	}

	if got, err := module.MatchCommitPrefix("1234567", []module.CommitID{a, a, other}); err != nil {
		t.Fatalf("duplicate identical candidate failed: %v", err)
	} else if got != a {
		t.Fatalf("matched %#v", got)
	}

	if _, err := module.MatchCommitPrefix("ffffff", []module.CommitID{a}); err == nil {
		t.Fatal("6-digit prefix was accepted")
	} else if got := revisionCode(t, err); got != module.ErrRevisionHash {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionHash)
	}

	if _, err := module.MatchCommitPrefix("fffffff", []module.CommitID{a}); err == nil {
		t.Fatal("unmatched prefix was accepted")
	} else if got := revisionCode(t, err); got != module.ErrRevisionHash {
		t.Fatalf("code = %s, want %s", got, module.ErrRevisionHash)
	}
}

func TestSourceIdentity(t *testing.T) {
	good := module.SourceIdentity{Name: "github.com/acme/mono/tool/v2", URL: "https://github.com/acme/mono", Subdir: "tool/v2"}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}
	if _, err := module.NewSourceIdentity(good.Name, good.URL, good.Subdir); err != nil {
		t.Fatalf("NewSourceIdentity: %v", err)
	}

	bad := []module.SourceIdentity{
		{Name: ""},
		{Name: "GitHub.com/acme/mod"},
		{Name: "example.com/mod/v0"},
		{Name: "example.com/mod/v2", Subdir: "../escaped"},
		{Name: "example.com/mod", Subdir: "/abs"},
		{Name: "example.com/mod", Subdir: "a//b"},
		{Name: "example.com/mod", Subdir: `a\b`},
		{Name: "example.com/mod", Subdir: "."},
		{Name: "example.com/mod", URL: "https://example.com/a b"},
	}
	for _, s := range bad {
		if err := s.Validate(); err == nil {
			t.Errorf("SourceIdentity %#v was accepted", s)
		} else if got := revisionCode(t, err); got != module.ErrRevisionSource {
			t.Errorf("SourceIdentity %#v code = %s, want %s", s, got, module.ErrRevisionSource)
		}
	}
}

func TestNewRevisionRequestFromRequire(t *testing.T) {
	tests := []struct {
		req     module.Require
		wantURL string
		wantSub string
	}{
		{module.Require{Name: "github.com/acme/lib", Version: "v1.2.3"}, "https://github.com/acme/lib", ""},
		{module.Require{Name: "github.com/acme/mono/tool/v2", Version: "v2.0.0"}, "https://github.com/acme/mono", "tool/v2"},
		{module.Require{Name: "example.com/plain", Version: "1234567"}, "", ""},
	}
	for _, tt := range tests {
		got, err := module.NewRevisionRequest(tt.req)
		if err != nil {
			t.Errorf("NewRevisionRequest(%#v): %v", tt.req, err)
			continue
		}
		if got.Source.URL != tt.wantURL || got.Source.Subdir != tt.wantSub {
			t.Errorf("NewRevisionRequest(%#v) source = %#v, want url %q subdir %q", tt.req, got.Source, tt.wantURL, tt.wantSub)
		}
	}
}

func TestRevisionRequestValidation(t *testing.T) {
	src := module.SourceIdentity{Name: "github.com/acme/lib", URL: "https://github.com/acme/lib"}
	for _, version := range []string{"v1.2.3", "v1.2.3+meta", pseudoA, "1234567"} {
		if err := (module.RevisionRequest{Version: version, Source: src}).Validate(); err != nil {
			t.Errorf("version %q rejected: %v", version, err)
		}
	}
	for _, version := range []string{"", "1.2.3", "v1.2", "v1.2.3-01", "123456"} {
		if err := (module.RevisionRequest{Version: version, Source: src}).Validate(); err == nil {
			t.Errorf("version %q accepted", version)
		}
	}
	if err := (module.RevisionRequest{Version: "v2.0.0", Source: src}).Validate(); err == nil {
		t.Error("v2 request on an unsuffixed path was accepted")
	}
	if err := (module.RevisionRequest{Version: "v1.2.3", Source: src, Alias: "std/lib"}).Validate(); err == nil {
		t.Error("reserved alias was accepted")
	}
}

func TestRevisionRequestDiagnosticPosition(t *testing.T) {
	src := module.SourceIdentity{Name: "github.com/acme/lib"}
	loc := diag.Location{Span: token.Span{Start: 17, End: 22}}
	req := module.RevisionRequest{Version: "v1.2", Source: src, Loc: loc}
	err := req.Validate()
	if err == nil {
		t.Fatal("bad version was accepted")
	}
	var re *module.RevisionError
	if !errors.As(err, &re) {
		t.Fatalf("error %v is not a *RevisionError", err)
	}
	if re.Loc != loc {
		t.Fatalf("error location = %#v, want %#v", re.Loc, loc)
	}
	d := re.Diagnostic()
	if d.Loc != loc || d.Code != module.ErrRevisionSyntax.String() {
		t.Fatalf("diagnostic = %#v", d)
	}
	if !strings.Contains(err.Error(), "byte 17") {
		t.Fatalf("error does not mention its position: %v", err)
	}
}

func TestResolvedRevisionValidation(t *testing.T) {
	src := module.SourceIdentity{Name: "github.com/acme/lib"}
	commit := commitWithSuffix(t, "123456789abc", "1")
	baseVersion := mustRevisionCanonical(t, "v1.2.3")

	tag := module.ResolvedRevision{
		Request: module.RevisionRequest{Version: "v1.2.3", Source: src},
		Version: mustRevisionCanonical(t, "v1.2.3"),
		Commit:  commit,
		Time:    pseudoTime,
		Origin:  module.RevisionTag,
	}
	if err := tag.Validate(); err != nil {
		t.Fatalf("valid tag revision rejected: %v", err)
	}

	local := module.ResolvedRevision{
		Request: module.RevisionRequest{Version: "v0.0.0", Source: src},
		Version: mustRevisionCanonical(t, "v0.0.0"),
		Origin:  module.RevisionLocal,
		Local:   true,
	}
	if err := local.Validate(); err != nil {
		t.Fatalf("valid local revision rejected: %v", err)
	}

	pseudo := module.ResolvedRevision{
		Request: module.RevisionRequest{Version: pseudoB, Source: src},
		Version: mustRevisionCanonical(t, pseudoB),
		Commit:  commit,
		Time:    pseudoTime,
		Origin:  module.RevisionPseudo,
		Base: &module.RevisionBase{
			Version: baseVersion,
			Tag:     "v1.2.3",
			Commit:  commitWithSuffix(t, "abcdef123456", "2"),
		},
	}
	if err := pseudo.Validate(); err != nil {
		t.Fatalf("valid pseudo revision rejected: %v", err)
	}

	bad := []struct {
		name string
		mut  func(*module.ResolvedRevision)
		code module.RevisionErrorCode
	}{
		{"missing commit", func(r *module.ResolvedRevision) { r.Commit = module.CommitID{} }, module.ErrRevisionHash},
		{"non-UTC time", func(r *module.ResolvedRevision) {
			r.Time = time.Date(2026, 11, 23, 1, 0, 0, 0, time.FixedZone("X", 3600))
		}, module.ErrRevisionPseudoTime},
		{"fractional time", func(r *module.ResolvedRevision) { r.Time = pseudoTime.Add(500 * time.Millisecond) }, module.ErrRevisionPseudoTime},
		{"major mismatch", func(r *module.ResolvedRevision) { r.Version = mustRevisionCanonical(t, "v2.0.0") }, module.ErrRevisionMajorMismatch},
		{"tag with base", func(r *module.ResolvedRevision) { r.Base = &module.RevisionBase{} }, module.ErrRevisionPseudoBase},
		{"unknown origin", func(r *module.ResolvedRevision) { r.Origin = module.RevisionOrigin(99) }, module.ErrRevisionSource},
		{"local without flag", func(r *module.ResolvedRevision) { r.Local = true }, module.ErrRevisionSource},
		{"based pseudo without base", func(r *module.ResolvedRevision) {
			r.Request.Version = pseudoB
			r.Version = mustRevisionCanonical(t, pseudoB)
			r.Origin = module.RevisionPseudo
			r.Base = nil
			r.Commit = commitWithSuffix(t, "123456789abc", "3")
		}, module.ErrRevisionPseudoBase},
	}
	for _, tt := range bad {
		r := tag
		tt.mut(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s was accepted", tt.name)
		} else if got := revisionCode(t, err); got != tt.code {
			t.Errorf("%s code = %s, want %s", tt.name, got, tt.code)
		}
	}
}
