package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"slices"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// scanImports reads every `import` keyword with the lexer alone, so it may name more packages than the parser, never fewer.
func scanImports(src []byte) []string {
	toks := syntax.Lex(src, &diag.Bag{})
	var out []string
	for i, tk := range toks {
		if tk.Kind != token.KwImport {
			continue
		}
		for j := i + 1; j < len(toks) && toks[j].Kind != token.Newline && toks[j].Kind != token.EOF; j++ {
			if toks[j].Kind != token.Ident || toks[j].Text(src) != "from" {
				continue
			}
			from := int(toks[j].End)
			for from < len(src) && (src[from] == ' ' || src[from] == '\t') {
				from++
			}
			end := from
			for end < len(src) && !isPathEnd(src[end]) {
				end++
			}
			if end > from {
				out = append(out, string(src[from:end]))
			}
			break
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func isPathEnd(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func contentHash(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])
}

// scan fills f's content hash and imports, from the memo when size and mtime still match and from the file otherwise.
func (m *Module) scan(f *indexedFile) error {
	if f.scanned {
		return nil
	}
	if !f.loaded {
		if e, ok := m.index.memo.lookup(f.abs, f.size, f.mtime, f.ctime); ok {
			f.hash, f.imports, f.scanned = e.Hash, e.Imports, true
			m.idxStats.MemoHits++
			return nil
		}
		if _, err := m.source(f); err != nil {
			return err
		}
		f.hash, f.imports, f.scanned = contentHash(f.src), scanImports(f.src), true
		m.index.memo.put(f.abs, f.size, f.mtime, f.ctime, f.hash, f.imports)
		return nil
	}
	f.hash, f.imports, f.scanned = contentHash(f.src), scanImports(f.src), true
	if !f.overlay {
		m.index.memo.put(f.abs, f.size, f.mtime, f.ctime, f.hash, f.imports)
	}
	return nil
}

// source returns f's bytes, reading the file once.
func (m *Module) source(f *indexedFile) ([]byte, error) {
	if f.loaded {
		return f.src, nil
	}
	src, err := os.ReadFile(f.abs)
	if err != nil {
		return nil, err
	}
	m.idxStats.FilesRead++
	f.src, f.loaded = src, true
	return src, nil
}
