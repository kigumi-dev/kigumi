package lsp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"kigumi/internal/driver"
)

// TestDependencyCacheOrigin: a definition inside a module-cache dependency
// points at the cache's real file, not a fabricated workspace path.
func TestDependencyCacheOrigin(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	depDir, err := driver.CacheDir(testCompiler(), driver.Require{Name: "example.com/greet", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(depDir, 0o755)
	os.WriteFile(filepath.Join(depDir, "mod.kg"), []byte("Module {\n    name: \"example.com/greet\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(depDir, "greet.kg"), []byte("pub fn hello() -> Int {\n    1\n}\n"), 0o644)
	os.WriteFile(filepath.Join(depDir, driver.OriginFileName), []byte("cafebabe\n"), 0o644)
	hash, err := driver.TreeHash(depDir)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"example.com/greet\", version: \"v1.0.0\" }\n"), 0o644)
	lock := driver.Lock{Entries: map[string]driver.LockEntry{"example.com/greet": {Version: "v1.0.0", Commit: "cafebabe", Hash: hash}}}
	if err := driver.WriteLock(root, lock); err != nil {
		t.Fatal(err)
	}
	src := "import {hello} from example.com/greet\nprint \"${hello()}\"\n"
	path := filepath.Join(root, "main.kg")
	os.WriteFile(path, []byte(src), 0o644)

	std, _ := filepath.Abs("../../std")
	ctx := protocol.WithClient(context.Background(), &recorder{})
	s := newServer(std)
	s.Initialize(ctx, initParams(uri.File(root)))
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Text: src}})

	at := protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: 1, Character: 11}}
	def, _ := s.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: at})
	loc, ok := def.(*protocol.Location)
	wantURI := uri.File(filepath.Join(depDir, "greet.kg"))
	if !ok || loc.URI != wantURI {
		t.Fatalf("definition of a cache dependency symbol must point at the cache file: got %+v, want %s", def, wantURI)
	}

	if _, err := s.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at, NewName: "greetings"}); err == nil {
		t.Fatalf("renaming a dependency's own declaration must be rejected")
	}
}

// TestDependencyLocalReplaceOrigin: a mod.local.kg replacement resolves to
// the developer's own checkout, and stays non-editable from here too.
func TestDependencyLocalReplaceOrigin(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "the-lib")
	app := filepath.Join(base, "app")
	os.MkdirAll(lib, 0o755)
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(lib, "mod.kg"), []byte("Module {\n    name: \"the-lib\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(lib, "lib.kg"), []byte("pub fn twice(x: Int) -> Int {\n    x * 2\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"the-lib\", version: \"v1.0.0\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "mod.local.kg"), []byte("Replace { name: \"the-lib\", path: \"../the-lib\" }\n"), 0o644)
	src := "import {twice} from the-lib\nprint \"${twice(2)}\"\n"
	path := filepath.Join(app, "main.kg")
	os.WriteFile(path, []byte(src), 0o644)

	std, _ := filepath.Abs("../../std")
	ctx := protocol.WithClient(context.Background(), &recorder{})
	s := newServer(std)
	s.Initialize(ctx, initParams(uri.File(app)))
	doc := uri.File(path)
	s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{URI: doc, Text: src}})

	at := protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: doc}, Position: protocol.Position{Line: 1, Character: 11}}
	def, _ := s.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: at})
	loc, ok := def.(*protocol.Location)
	wantURI := uri.File(filepath.Join(lib, "lib.kg"))
	if !ok || loc.URI != wantURI {
		t.Fatalf("definition of a locally-replaced dependency must point at the replacement checkout: got %+v, want %s", def, wantURI)
	}

	if _, err := s.Rename(ctx, &protocol.RenameParams{TextDocumentPositionParams: at, NewName: "double"}); err == nil {
		t.Fatalf("renaming a local-replacement dependency's own declaration must be rejected")
	}
}
