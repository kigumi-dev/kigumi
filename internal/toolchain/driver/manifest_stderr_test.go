package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/devkit/testtool"
)

func TestManifestStderrMatchersAreAnchored(t *testing.T) {
	for _, fx := range loadManifest(t).Fixtures {
		if fx.Stderr == nil || fx.Stderr.Type != "regex" {
			continue
		}
		if !strings.HasPrefix(fx.Stderr.Pattern, `\A`) || !strings.HasSuffix(fx.Stderr.Pattern, `\z`) {
			t.Errorf("%s: stderr pattern %q is not anchored with \\A and \\z", fx.ID, fx.Stderr.Pattern)
		}
	}
}

func TestStackOverflowStderrMatcherRejectsExtraLines(t *testing.T) {
	for _, bad := range []string{"warning: x\npanic: stack overflow in main.f\n", "panic: stack overflow in main.f\npanic: again\n"} {
		if err := checkStackOverflowStderr(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	for _, ok := range []string{"panic: stack overflow\n", "panic: stack overflow in main.deep.closure1$boxed\n"} {
		if err := checkStackOverflowStderr(ok); err != nil {
			t.Errorf("rejected %q: %v", ok, err)
		}
	}
}

func checkStackOverflowStderr(got string) error {
	return testtool.CheckStderr(&testtool.StderrMatch{Type: "regex", Pattern: stackOverflowStderr}, "", got)
}
