package mirfixture

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirverify "kigumi/internal/compiler/mir/verify"
)

func WantRule(t *testing.T, p *mir.Program, rule, fragment string) {
	t.Helper()
	err := mirverify.Program(p)
	ve, ok := err.(*mirverify.Error)
	if !ok || ve.Rule != rule || !strings.Contains(ve.Msg, fragment) {
		t.Fatalf("got %v, want rule %s mentioning %q", err, rule, fragment)
	}
}

func Built(t *testing.T, src string) *mir.Program {
	t.Helper()
	p := mirbuild.Program(CheckSourceMir(t, src))
	if p.HasErrors() {
		t.Fatalf("build errors:\n%s", p.Render())
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("the base program does not verify: %v", err)
	}
	return p
}

func DirectRule(t *testing.T, p *mir.Program, fragment string) {
	t.Helper()
	ve, ok := mirverify.Program(p).(*mirverify.Error)
	if !ok || ve.Rule != "direct" || !strings.Contains(ve.Msg, fragment) {
		t.Fatalf("got %v, want a direct error containing %q", ve, fragment)
	}
}
