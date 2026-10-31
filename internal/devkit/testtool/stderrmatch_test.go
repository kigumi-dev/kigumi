package testtool_test

import (
	"testing"

	"kigumi/internal/devkit/testtool"
)

func TestCheckStderrExactDefaultsToEmpty(t *testing.T) {
	if err := testtool.CheckStderr(nil, "", "panic: boom\n"); err == nil {
		t.Fatal("expected a mismatch against the implicit empty expectation")
	}
	if err := testtool.CheckStderr(nil, "", ""); err != nil {
		t.Fatalf("empty vs empty should match: %v", err)
	}
}

func TestCheckStderrExactIgnoresTrailingNewline(t *testing.T) {
	if err := testtool.CheckStderr(nil, "error: bad line 1\n", "error: bad line 1"); err != nil {
		t.Fatalf("trailing newline should not matter: %v", err)
	}
}

func TestCheckStderrRegex(t *testing.T) {
	match := &testtool.StderrMatch{Type: "regex", Pattern: "^panic: integer overflow"}
	if err := testtool.CheckStderr(match, "", "panic: integer overflow\n  at main/main.kg:4:9\n"); err != nil {
		t.Fatalf("interp-shaped stderr should match: %v", err)
	}
	if err := testtool.CheckStderr(match, "", "panic: out of memory\n"); err == nil {
		t.Fatal("wrong panic message should not match")
	}
}

func TestCheckStderrIgnore(t *testing.T) {
	if err := testtool.CheckStderr(&testtool.StderrMatch{Type: "ignore"}, "", "anything at all"); err != nil {
		t.Fatalf("ignore should accept anything: %v", err)
	}
}

func TestCheckStderrUnknownType(t *testing.T) {
	if err := testtool.CheckStderr(&testtool.StderrMatch{Type: "bogus"}, "", ""); err == nil {
		t.Fatal("unknown matcher type should error")
	}
}
