package lsp

import (
	"testing"

	"kigumi/internal/driver"
)

// The server loads through the driver's package index and cache like the
// CLI: a second analysis of an unchanged tree reads its file metadata from the memo.
func TestServerUsesThePackageCache(t *testing.T) {
	root := writeModule(t, map[string]string{
		"mod.kg":          "Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n",
		"lib/lib.kg":      "pub fn one() -> Int {\n    1\n}\n",
		"cmd/app/main.kg": "import {one} from app/lib\nprint \"${one()}\"\n",
	})
	s, ctx, _ := startServer(t, root, 0)
	s.WithPackageCache(t.TempDir(), driver.CompilerOptions{})
	s.mu.Lock()
	first := s.refresh(ctx, root, "", true)
	second := s.refresh(ctx, root, "", true)
	s.mu.Unlock()
	if first.mod.IndexStats().MemoHits != 0 {
		t.Fatalf("a cold cache cannot hit: %+v", first.mod.IndexStats())
	}
	if st := second.mod.IndexStats(); st.MemoHits == 0 {
		t.Fatalf("the second load should reuse the index memo: first %+v, second %+v", first.mod.IndexStats(), st)
	}
}
