package lsp_test

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// An unannotated let gets its inferred type; an annotated one keeps its
// written type and gets no hint.
func TestInlayHintInferredType(t *testing.T) {
	src := "fn make() -> Int {\n    1\n}\n\nlet a = make()\nlet b: Int = make()\n"
	s, ctx, doc, _ := openModule(t, src)
	hints, err := s.InlayHint(ctx, &protocol.InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: doc},
		Range:        protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 99}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != 1 {
		t.Fatalf("hints = %+v, want exactly the unannotated binding", hints)
	}
	h := hints[0]
	if h.Position != (protocol.Position{Line: 4, Character: 5}) {
		t.Fatalf("hint position = %+v, want end of `a`", h.Position)
	}
	if label, _ := h.Label.(protocol.String); string(label) != ": Int" {
		t.Fatalf("hint label = %v, want %q", h.Label, ": Int")
	}
	if h.Kind != protocol.InlayHintKindType {
		t.Fatalf("hint kind = %v", h.Kind)
	}
}

// A binding name outside ASCII reports its position in UTF-16 units, not
// bytes.
func TestInlayHintUnicodeName(t *testing.T) {
	src := "fn make() -> Int {\n    1\n}\n\nlet ñ = make()\n"
	s, ctx, doc, _ := openModule(t, src)
	hints, err := s.InlayHint(ctx, &protocol.InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: doc},
		Range:        protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 99}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != 1 {
		t.Fatalf("hints = %+v", hints)
	}
	if hints[0].Position != (protocol.Position{Line: 4, Character: 5}) {
		t.Fatalf("UTF-16 position = %+v, want character 5 (after `let ` and `ñ`)", hints[0].Position)
	}
}

// The requested range filters hints to the visible lines.
func TestInlayHintRangeFilter(t *testing.T) {
	src := "fn make() -> Int {\n    1\n}\n\nlet a = make()\nlet bb = make()\n"
	s, ctx, doc, _ := openModule(t, src)
	hints, err := s.InlayHint(ctx, &protocol.InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: doc},
		Range:        protocol.Range{Start: protocol.Position{Line: 5}, End: protocol.Position{Line: 6}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != 1 || hints[0].Position != (protocol.Position{Line: 5, Character: 6}) {
		t.Fatalf("filtered hints = %+v, want only `bb` on line 5", hints)
	}
}

// Type definition on a local binding and on its use both land on the
// declaration of the binding's named type.
func TestTypeDefinition(t *testing.T) {
	src := "type Point = { pub x Int, pub y Int }\n\nfn f() -> Int {\n    let p = Point { x: 1, y: 2 }\n    p.x\n}\n"
	s, ctx, doc, _ := openModule(t, src)
	for _, pos := range []protocol.TextDocumentPositionParams{at(doc, 3, 8), at(doc, 4, 4)} {
		res, err := s.TypeDefinition(ctx, &protocol.TypeDefinitionParams{TextDocumentPositionParams: pos})
		if err != nil {
			t.Fatal(err)
		}
		loc, ok := res.(*protocol.Location)
		if !ok || loc == nil {
			t.Fatalf("type definition at %+v = %#v, want one location", pos.Position, res)
		}
		if loc.URI != doc || loc.Range.Start != (protocol.Position{Line: 0, Character: 5}) || loc.Range.End != (protocol.Position{Line: 0, Character: 10}) {
			t.Fatalf("type definition at %+v = %+v, want the `Point` name", pos.Position, loc)
		}
	}
}

// Both providers are advertised in the initialize result.
func TestInlayHintAndTypeDefinitionCapabilities(t *testing.T) {
	s := newServer("../../../std")
	rec := &recorder{}
	ctx := protocol.WithClient(context.Background(), rec)
	res, err := s.Initialize(ctx, initParams(uri.File(t.TempDir())))
	if err != nil {
		t.Fatal(err)
	}
	if res.Capabilities.InlayHintProvider != protocol.Boolean(true) {
		t.Fatalf("InlayHintProvider = %#v", res.Capabilities.InlayHintProvider)
	}
	if res.Capabilities.TypeDefinitionProvider != protocol.Boolean(true) {
		t.Fatalf("TypeDefinitionProvider = %#v", res.Capabilities.TypeDefinitionProvider)
	}
}
