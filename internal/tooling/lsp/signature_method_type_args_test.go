package lsp_test

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestSignatureHelpMethodTypeArgs(t *testing.T) {
	src := "fn Box.conv[T](self, v: T) -> T {\n    v\n}\n\ntype Box = {\n    x Int\n}\n\nlet b = Box { x: 1 }\nlet w: u16 = 7\nlet a = b.conv[Int](2)\nlet c = w.truncate[u8]()\n"
	s, ctx, doc, _ := openModule(t, src)
	help, err := s.SignatureHelp(ctx, &protocol.SignatureHelpParams{TextDocumentPositionParams: at(doc, 10, 20)})
	if err != nil || help == nil || len(help.Signatures) != 1 {
		t.Fatalf("signature help: %v %+v", err, help)
	}
	if got := help.Signatures[0].Label; !strings.Contains(got, "conv[T]") || len(help.Signatures[0].Parameters) != 1 {
		t.Fatalf("signature = %+v, want the generic method with its parameter", help.Signatures[0])
	}
	std, err := s.SignatureHelp(ctx, &protocol.SignatureHelpParams{TextDocumentPositionParams: at(doc, 11, 23)})
	if err != nil || std == nil || len(std.Signatures) != 1 || !strings.Contains(std.Signatures[0].Label, "truncate[T]") {
		t.Fatalf("std signature help: %v %+v", err, std)
	}
}
