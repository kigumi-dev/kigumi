package doc_test

import (
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/tooling/doc"
)

func TestDeclarationsKeepEscapingTypeParam(t *testing.T) {
	m := loadSchemaIRProbe(t, "pub fn keep[F: Fn[fn() -> Void]](callback: escaping F, other: F) {}\n")
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	decls := doc.Declarations(m, res, "")
	if len(decls) != 1 || len(decls[0].Params) != 2 {
		t.Fatalf("declarations: %+v", decls)
	}
	if got := decls[0].Params[0].Type; got != "escaping F" {
		t.Errorf("escaping parameter type %q", got)
	}
	if got := decls[0].Params[1].Type; got != "F" {
		t.Errorf("plain parameter type %q", got)
	}
}
