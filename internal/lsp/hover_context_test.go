package lsp_test

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// A function that only calls `print` carries a real `stdout` requirement
// (joined into CtxRequires, not a disconnected bookkeeping list), visible
// transitively on hover at any of its callers.
func TestHoverContextRequiresFromPrint(t *testing.T) {
	src := "fn greet() -> Unit {\n    print \"hi\"\n}\n\nfn wrapper() -> Unit {\n    greet()\n}\n\nwrapper()\n"
	s, ctx, doc, _ := openModule(t, src)
	hoverAt := func(pos protocol.TextDocumentPositionParams) string {
		h, err := s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: pos})
		if err != nil {
			t.Fatal(err)
		}
		if h == nil {
			return ""
		}
		return h.Contents.(*protocol.MarkupContent).Value
	}
	if got := hoverAt(at(doc, 0, 4)); !strings.Contains(got, "requires: std/stdio.stdout") {
		t.Fatalf("hover on `greet` itself: %q", got)
	}
	if got := hoverAt(at(doc, 5, 5)); !strings.Contains(got, "requires: std/stdio.stdout") {
		t.Fatalf("hover on `wrapper`, which only calls greet: %q", got)
	}
}
