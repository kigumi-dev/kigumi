package module_test

import (
	"strings"
	"testing"

	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/module"
)

func TestParseLocalRejectsNonCanonicalReplaceName(t *testing.T) {
	t.Parallel()
	_, ds := module.ParseLocal(token.NewFile(module.LocalName, []byte("Replace { name: \"Example.com/dep\", path: \"../dep\" }\n")), "/w")
	if len(ds) == 0 || !strings.Contains(ds[0].Msg, "not canonical") {
		t.Fatalf("diagnostics = %v", ds)
	}
}

func TestParseLocalRejectsDuplicateReplace(t *testing.T) {
	t.Parallel()
	src := "Replace { name: \"example.com/dep\", path: \"/a\" }\nReplace { name: \"example.com/dep\", path: \"../b\" }\n"
	lc, ds := module.ParseLocal(token.NewFile(module.LocalName, []byte(src)), "/w")
	if len(ds) == 0 || !strings.Contains(ds[0].Msg, "replaced twice") {
		t.Fatalf("diagnostics = %v", ds)
	}
	if lc.Replaces["example.com/dep"] != "/a" {
		t.Fatalf("the first record must stay, got %q", lc.Replaces["example.com/dep"])
	}
}
