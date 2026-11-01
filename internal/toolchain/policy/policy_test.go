package policy_test

import (
	"strings"
	"testing"

	"kigumi/internal/toolchain/policy"
)

// Development is BuildPolicy's zero value, so every pre-existing call
// site that never mentions Policy keeps today's behavior.
func TestDevelopmentIsZeroValue(t *testing.T) {
	if policy.Development() != (policy.BuildPolicy{}) {
		t.Fatal("Development() must equal the zero BuildPolicy")
	}
	if policy.Development().IsRelease() {
		t.Fatal("Development must not be a release policy")
	}
	if !policy.Release().IsRelease() {
		t.Fatal("Release must be a release policy")
	}
	if got := policy.Development().String(); got != "development" {
		t.Fatalf("Development().String() = %q", got)
	}
	if got := policy.Release().String(); got != "release-v1" {
		t.Fatalf("Release().String() = %q", got)
	}
}

// TestReportRenderShape checks the report rendering: a severity word, the
// stable id, detail lines, a release-policy reason, a help line, and a summary.
func TestReportRenderShape(t *testing.T) {
	report := policy.Evaluate([]policy.Violation{{
		ID:      "release-local-replace",
		Subject: "github.com/acme/codec is resolved from a local directory",
		Detail:  []policy.KV{{Key: "selected by", Value: "/work/mod.local.kg:3"}, {Key: "replacement", Value: "/work/codec"}},
		Policy:  "rejected because the content is not the locked module tree",
		Help:    "update and lock the dependency, or keep this as a development build",
	}}, policy.Development())
	out := report.Render()
	for _, want := range []string{
		"warning[release-local-replace]: github.com/acme/codec is resolved from a local directory\n",
		"  selected by: /work/mod.local.kg:3\n",
		"  replacement: /work/codec\n",
		"  release policy: rejected because the content is not the locked module tree\n",
		"  help: update and lock the dependency, or keep this as a development build\n",
		"release readiness: 1 warning(s); would fail with --release\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q in:\n%s", want, out)
		}
	}
}

// Ambient violations only the CLI can detect (KIGUMI_STD, KIGUMI_CFLAGS)
// plug into policy.Evaluate the same way as load-time ones.
func TestAmbientViolationsSeverity(t *testing.T) {
	extra := []policy.Violation{policy.AmbientStdViolation("/env/std"), policy.AmbientCFlagsViolation([]string{"-march=native"})}
	dev := policy.Evaluate(nil, policy.Development(), extra...)
	if len(dev.Warnings) != 2 || dev.HasErrors() {
		t.Fatalf("development: want 2 warnings, 0 errors, got %+v", dev)
	}
	rel := policy.Evaluate(nil, policy.Release(), extra...)
	if len(rel.Errors) != 2 || len(rel.Warnings) != 0 {
		t.Fatalf("release: want 2 errors, 0 warnings, got %+v", rel)
	}
}
