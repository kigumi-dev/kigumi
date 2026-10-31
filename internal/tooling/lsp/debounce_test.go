package lsp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/toolchain/driver"
)

// badAt puts a type error on line n (zero-based), so the published range says which edit it came from.
func badAt(n int) string {
	return strings.Repeat("\n", n) + "let bad: String = 1\n"
}

func TestEditsAreCoalescedByTheDebounce(t *testing.T) {
	root := writeModule(t, map[string]string{"main/main.kg": badAt(0)})
	s, ctx, c := startServer(t, root, 80*time.Millisecond)
	doc := uri.File(filepath.Join(root, "main", "main.kg"))
	openDoc(s, ctx, doc, 1, badAt(0))
	before := s.Counters()
	opened := len(c.published(doc))
	for v := int32(2); v <= 6; v++ {
		editDoc(s, ctx, doc, v, badAt(int(v)))
	}
	s.WaitIdle()
	got := s.Counters()
	if got.Built-before.Built != 1 {
		t.Fatalf("five keystrokes inside the window should run one analysis, ran %d", got.Built-before.Built)
	}
	pubs := c.published(doc)
	last := pubs[len(pubs)-1]
	if v, _ := last.Version.Get(); v != 6 || len(last.Diagnostics) != 1 || last.Diagnostics[0].Range.Start.Line != 6 {
		t.Fatalf("the last publish should describe the last edit: %+v", last)
	}
	for _, p := range pubs[opened:] {
		if v, _ := p.Version.Get(); v != 6 {
			t.Fatalf("a publish for an earlier edit leaked: version %d", v)
		}
	}
}

func TestSupersededAnalysisIsNeverPublished(t *testing.T) {
	root := writeModule(t, map[string]string{"main/main.kg": badAt(0)})
	s, ctx, c := startServer(t, root, 20*time.Millisecond)
	doc := uri.File(filepath.Join(root, "main", "main.kg"))
	openDoc(s, ctx, doc, 1, badAt(0))
	opened := len(c.published(doc))

	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.hook = func(stage string, in *analysisInput) {
		if stage != "loaded" {
			return
		}
		once.Do(func() {
			close(started)
			<-release
		})
	}
	editDoc(s, ctx, doc, 2, badAt(2))
	wait(t, started, "the first analysis to start")
	editDoc(s, ctx, doc, 3, badAt(3))
	close(release)
	s.WaitIdle()

	for _, p := range c.published(doc)[opened:] {
		if v, _ := p.Version.Get(); v != 3 {
			t.Fatalf("stale generation published: version %d, %+v", v, p.Diagnostics)
		}
		for _, d := range p.Diagnostics {
			if d.Range.Start.Line == 2 {
				t.Fatalf("diagnostic of the superseded edit was published: %+v", d)
			}
		}
	}
	if got := s.Counters(); got.Dropped < 1 {
		t.Fatalf("the superseded analysis should be dropped: %+v", got)
	}
}

func TestHoverAnswersFromTheLastSnapshotWhileAnalyzing(t *testing.T) {
	src := "fn double(x: Int) -> Int {\n    x * 2\n}\n\nlet value = double(21)\nprint \"${value}\"\n"
	root := writeModule(t, map[string]string{"main/main.kg": src})
	s, ctx, _ := startServer(t, root, 20*time.Millisecond)
	doc := uri.File(filepath.Join(root, "main", "main.kg"))
	openDoc(s, ctx, doc, 1, src)

	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.hook = func(stage string, in *analysisInput) {
		if stage == "loaded" {
			once.Do(func() {
				close(started)
				<-release
			})
		}
	}
	edited := "// header\n// header\n" + src
	editDoc(s, ctx, doc, 2, edited)
	wait(t, started, "the re-analysis to start")
	built := s.Counters().Built

	hover := func(line, char uint32) (*protocol.Hover, error) {
		return s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: line, Character: char}}})
	}
	h, err := hover(6, 15)
	if err != nil || h == nil || !strings.Contains(h.Contents.(*protocol.MarkupContent).Value, "fn double(x: Int) -> Int") {
		t.Fatalf("hover during analysis: %v %+v", err, h)
	}
	if h.Range == nil || h.Range.Start.Line != 6 {
		t.Fatalf("the range must be in the current text, not the snapshot's: %+v", h.Range)
	}
	if h, _ := hover(0, 4); h != nil {
		t.Fatalf("a position inside text edited since the snapshot has no answer: %+v", h)
	}
	if got := s.Counters().Built; got != built {
		t.Fatalf("hover started an analysis: %d -> %d", built, got)
	}
	close(release)
	s.WaitIdle()
	if h, _ := hover(6, 15); h == nil || h.Range.Start.Line != 6 {
		t.Fatalf("hover after the analysis settled: %+v", h)
	}
}

func TestTypingNeverEvaluatesComptime(t *testing.T) {
	src := "fn boom(n: usize) -> Int {\n    let xs = Array.of(1, 2)\n    xs[n]\n}\n\nlet a = comptime {\n    boom(9)\n}\nprint \"${a}\"\n"
	root := writeModule(t, map[string]string{"main/main.kg": src})

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: internalTestCompiler(), StdRoot: "../../../std", Test: true})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil || !strings.Contains(a.Res.Render(), "E2240") {
		t.Fatalf("the fixture must fail in a build: %v\n%s", err, a.Res.Render())
	}

	s, ctx, c := startServer(t, root, 20*time.Millisecond)
	doc := uri.File(filepath.Join(root, "main", "main.kg"))
	openDoc(s, ctx, doc, 1, src)
	for v := int32(2); v <= 4; v++ {
		editDoc(s, ctx, doc, v, src+strings.Repeat("// edit\n", int(v)))
	}
	s.WaitIdle()
	for _, p := range c.published(doc) {
		for _, d := range p.Diagnostics {
			if strings.Contains(fmt.Sprint(d.Message), "comptime") {
				t.Fatalf("comptime ran while typing: %+v", d)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.last[root].mod.Stats(); st != (driver.Stats{}) {
		t.Fatalf("the editor path must not check for a build, lower MIR or emit: %+v", st)
	}
}

func TestSlowAnalysisIsCancelledByTheNextEdit(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20000; i++ {
		fmt.Fprintf(&sb, "fn f%d(x: Int) -> Int {\n    let a = x + %d\n    let b = a * 2\n    if a > b {\n        a\n    } else {\n        b\n    }\n}\n\n", i, i)
	}
	big := sb.String()
	root := writeModule(t, map[string]string{"main/main.kg": big})
	s, ctx, _ := startServer(t, root, 20*time.Millisecond)
	doc := uri.File(filepath.Join(root, "main", "main.kg"))

	t0 := time.Now()
	openDoc(s, ctx, doc, 1, big)
	full := time.Since(t0)

	checkedCh := make(chan error, 8)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.hook = func(stage string, in *analysisInput) {
		switch stage {
		case "loaded":
			once.Do(func() {
				close(started)
				<-release
			})
		case "checked":
			checkedCh <- in.ctx.Err()
		}
	}
	editDoc(s, ctx, doc, 2, big+"// one\n")
	wait(t, started, "the first analysis")
	editDoc(s, ctx, doc, 3, big+"// two\n")
	releasedAt := time.Now()
	close(release)

	if err := <-checkedCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("the superseded analysis should be cancelled, not run to completion: %v", err)
	}
	if stop := time.Since(releasedAt); stop > full {
		t.Fatalf("cancellation took %v, longer than a full %v analysis", stop, full)
	}
	s.WaitIdle()
	s.mu.Lock()
	defer s.mu.Unlock()
	if got := s.last[root]; got == nil || !strings.HasSuffix(string(got.mod.Packages["main"].Files[0].File.Src), "// two\n") {
		t.Fatal("the surviving snapshot must be the last edit's")
	}
}
