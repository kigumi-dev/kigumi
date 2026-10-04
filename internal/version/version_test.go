package version

import "testing"

func TestCurrentReportsRelease(t *testing.T) {
	p := Current()
	if p.Release != Release {
		t.Errorf("Release = %q, want %q", p.Release, Release)
	}
	if p.Commit == "" {
		t.Error("Commit must default to a non-empty placeholder, not \"\"")
	}
}

func TestWrapStampsFormatVersionAndKind(t *testing.T) {
	e := Wrap("widget", 3, map[string]int{"x": 1})
	if e.FormatVersion != 3 || e.Kind != "widget" {
		t.Errorf("Wrap: formatVersion=%d kind=%q", e.FormatVersion, e.Kind)
	}
	if e.Toolchain.Release != Release {
		t.Errorf("Wrap did not stamp the running release")
	}
}
