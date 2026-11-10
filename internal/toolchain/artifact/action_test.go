package artifact_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/artifact"
)

func baseAction() artifact.Action {
	return artifact.Action{
		Tool:    "framework:toolchain:cc",
		Version: "1",
		Runner:  "0.1.0 abc123",
		Digest:  "1111111111111111111111111111111111111111111111111111111111111111",
		Args:    []string{"-c", "$in:0", "-o", "$out:0"},
		Env:     []string{"PATH=/bin", "KIGUMI_TARGET_TRIPLE=x86_64-linux"},
		Inputs:  []string{"aaaa"},
		Outputs: []string{"hello.o"},
	}
}

// TestActionKeyStable pins that an action with the same invocation keys the
// same way regardless of the order its fields were assembled in.
func TestActionKeyStable(t *testing.T) {
	a := baseAction()
	if got := a.Key(); got != a.Key() {
		t.Fatalf("the same action keyed as %s and %s", got, a.Key())
	}
	b := a
	b.Args = []string{"-c", "$in:0", "-o", "$out:0"}
	if a.Key() != b.Key() {
		t.Error("an identically built action keyed differently")
	}
}

// TestActionKeySplitsEveryComponent is the identity contract: a change in the
// tool, its version, its digest, argv, environment, input hashes or declared
// outputs must never reuse a recorded action.
func TestActionKeySplitsEveryComponent(t *testing.T) {
	base := baseAction()
	key := base.Key()
	cases := []struct {
		name   string
		mutate func(*artifact.Action)
	}{
		{"tool", func(a *artifact.Action) { a.Tool = "framework:toolchain:ld" }},
		{"tool version", func(a *artifact.Action) { a.Version = "2" }},
		{"runner", func(a *artifact.Action) { a.Runner = "0.2.0 def456" }},
		{"tool digest", func(a *artifact.Action) {
			a.Digest = "2222222222222222222222222222222222222222222222222222222222222222"
		}},
		{"argv", func(a *artifact.Action) { a.Args = []string{"-c", "$in:0", "-On", "-o", "$out:0"} }},
		{"env value", func(a *artifact.Action) { a.Env = []string{"PATH=/usr/bin", "KIGUMI_TARGET_TRIPLE=x86_64-linux"} }},
		{"env name", func(a *artifact.Action) { a.Env = []string{"PATH=/bin", "KIGUMI_TARGET_SYS=linux"} }},
		{"input hash", func(a *artifact.Action) { a.Inputs = []string{"bbbb"} }},
		{"input count", func(a *artifact.Action) { a.Inputs = []string{"aaaa", "bbbb"} }},
		{"output name", func(a *artifact.Action) { a.Outputs = []string{"other.o"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := base
			c.mutate(&got)
			if got.Key() == key {
				t.Errorf("a changed %s keyed the same as the original action", c.name)
			}
		})
	}
}

// TestActionKeyFraming checks that field boundaries are part of the key, so
// two different argument lists cannot hash alike by concatenation.
func TestActionKeyFraming(t *testing.T) {
	a := baseAction()
	a.Args = []string{"ab", "c"}
	b := baseAction()
	b.Args = []string{"a", "bc"}
	if a.Key() == b.Key() {
		t.Error("argument lists that only differ in their split keyed the same")
	}
}

// TestCanonicalEnv covers the rule the key and the child process share: the
// last value of a variable wins, entries sort by name, and reordering the
// allowlist cannot change either the key or the environment.
func TestCanonicalEnv(t *testing.T) {
	got := artifact.CanonicalEnv([]string{"B=2", "A=1", "B=3", "junk", "A=1"})
	want := []string{"A=1", "B=3"}
	if len(got) != len(want) {
		t.Fatalf("CanonicalEnv = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CanonicalEnv = %q, want %q", got, want)
		}
	}
	a := baseAction()
	b := a
	b.Env = []string{"KIGUMI_TARGET_TRIPLE=x86_64-linux", "PATH=/bin"}
	if a.Key() != b.Key() {
		t.Error("a reordered environment split the action key")
	}
}

// TestDigestFile covers the content digest action keys use: path-independent,
// and different bytes never share a digest.
func TestDigestFile(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a")
	second := filepath.Join(dir, "b")
	if err := os.WriteFile(first, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := artifact.DigestFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := artifact.DigestFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("equal contents at different paths digested differently: %s, %s", a, b)
	}
	if err := os.WriteFile(first, []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := artifact.DigestFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if c == a {
		t.Error("changed content kept its digest")
	}
	if _, err := artifact.DigestFile(filepath.Join(dir, "missing")); err == nil {
		t.Error("a missing file must report an error, not digest as empty")
	}
}
