package lsp_test

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestSignatureHelpDefaults(t *testing.T) {
	src := "fn connect(host: String, port: Int = 443, tls: Bool = true) -> Int {\n    port\n}\n\nlet a = connect(\"h\", 1)\nlet b = connect(\"h\", tls: false)\n"
	s, ctx, doc, _ := openModule(t, src)
	help, err := s.SignatureHelp(ctx, &protocol.SignatureHelpParams{TextDocumentPositionParams: at(doc, 4, 22)})
	if err != nil || help == nil || len(help.Signatures) != 1 {
		t.Fatalf("signature help: %v %+v", err, help)
	}
	sig := help.Signatures[0]
	if !strings.Contains(sig.Label, "port: Int = 443") || len(sig.Parameters) != 3 {
		t.Fatalf("signature: %+v", sig)
	}
	if got := sig.Parameters[1].Label; got != protocol.String("port: Int = 443") {
		t.Fatalf("defaulted parameter label = %v", got)
	}
	if got := sig.Parameters[0].Label; got != protocol.String("host: String") {
		t.Fatalf("required parameter label = %v", got)
	}
	if v, ok := help.ActiveParameter.Get(); !ok || v != 1 {
		t.Fatalf("active parameter at `1` = %+v, want 1", help.ActiveParameter)
	}
	skipped, err := s.SignatureHelp(ctx, &protocol.SignatureHelpParams{TextDocumentPositionParams: at(doc, 5, 28)})
	if err != nil || skipped == nil {
		t.Fatalf("signature help for the call that skips a default: %v %+v", err, skipped)
	}
	if v, ok := skipped.ActiveParameter.Get(); !ok || v != 2 {
		t.Fatalf("active parameter inside `tls: false` = %+v, want 2", skipped.ActiveParameter)
	}
}

func TestFoldingIgnoresDefaultThunks(t *testing.T) {
	src := "fn connect(\n    host: String,\n    port: Int = 443,\n) -> Int {\n    port\n}\n"
	s, ctx, doc, _ := openModule(t, src)
	got, err := s.FoldingRanges(ctx, &protocol.FoldingRangeParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].StartLine != 0 || got[0].EndLine != 5 || got[1].StartLine != 3 || got[1].EndLine != 5 {
		t.Fatalf("folding ranges = %+v, want the function and its body", got)
	}
}
