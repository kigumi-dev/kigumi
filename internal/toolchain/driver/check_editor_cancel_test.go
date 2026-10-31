package driver_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// pollCtx counts Err calls and reports Canceled from the cancelAt-th on (0 never cancels).
type pollCtx struct {
	context.Context
	polls    atomic.Int64
	cancelAt int64
}

func (c *pollCtx) Err() error {
	if n := c.polls.Add(1); c.cancelAt > 0 && n >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func manyFunctions(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "fn f%d(x: Int) -> Int {\n    let a = x + %d\n    a * 2\n}\n\n", i, i)
	}
	sb.WriteString("print \"${f1(1)}\"\n")
	return sb.String()
}

func TestCheckForEditorStopsWhenCancelled(t *testing.T) {
	t.Parallel()
	src := manyFunctions(400)
	full := &pollCtx{Context: context.Background()}
	if _, err := driver.CheckForEditorContext(full, editorModule(t, src)); err != nil {
		t.Fatal(err)
	}
	total := full.polls.Load()
	if total < 400 {
		t.Fatalf("the checker should poll between bodies, polled %d times", total)
	}
	cut := &pollCtx{Context: context.Background(), cancelAt: total / 10}
	res, err := driver.CheckForEditorContext(cut, editorModule(t, src))
	if !errors.Is(err, context.Canceled) || res != nil {
		t.Fatalf("a cancelled check returns the context error and no result: %v %v", res, err)
	}
	if after := cut.polls.Load(); after > cut.cancelAt+1 {
		t.Fatalf("the check kept going after the cancel: %d polls, cancelled at %d", after, cut.cancelAt)
	}
}

const comptimeFailsSrc = `fn boom(n: usize) -> Int {
    let xs = Array.of(1, 2)
    xs[n]
}

let a = comptime {
    boom(9)
}
print "${a}"
`

func TestCheckForEditorNeverEvaluatesComptime(t *testing.T) {
	t.Parallel()
	for name, src := range map[string]string{
		"plain":       comptimeFailsSrc,
		"alloc claim": strings.Replace(deferredBoxSrc, "fn main() -> Void! {", "fn main() -> Void! {\n    let a = comptime {\n        boom(9)\n    }\n    print \"${a}\"", 1) + "\n" + comptimeFailsSrc[:strings.Index(comptimeFailsSrc, "let a")],
	} {
		m := editorModule(t, src)
		res, err := driver.CheckForEditor(m)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "alloc claim" && !strings.Contains(res.Render(), "E1570") {
			t.Fatalf("the case must reach the lowering half of the editor check:\n%s", res.Render())
		}
		if strings.Contains(res.Render(), "E2240") || strings.Contains(res.Render(), "E2241") {
			t.Fatalf("%s: comptime ran in the editor path:\n%s", name, res.Render())
		}
		if st := m.Stats(); st.Check != 0 || st.MIR != 0 || st.Emit != 0 {
			t.Fatalf("%s: the editor path must not run a build analysis, lower the whole program or emit: %+v", name, st)
		}
	}
	a, err := driver.Analyze(editorModule(t, comptimeFailsSrc))
	if err != nil || !strings.Contains(a.Res.Render(), "E2240") {
		t.Fatalf("a build must still evaluate it: %v\n%s", err, a.Res.Render())
	}
}

func TestPackageDigestFollowsPackageContent(t *testing.T) {
	t.Parallel()
	m := editorModule(t, "fn one() -> Int { 1 }\n")
	d := m.PackageDigest("editor")
	if d == "" || d != m.PackageDigest("editor") {
		t.Fatalf("digest should be stable: %q", d)
	}
	if m.PackageDigest("no/such/package") != "" {
		t.Fatal("an unknown package has no digest")
	}
	other := editorModule(t, "fn one() -> Int { 2 }\n")
	if other.PackageDigest("editor") == d {
		t.Fatal("different content must change the digest")
	}
}
