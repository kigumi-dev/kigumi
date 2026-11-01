package driver

import (
	"os"

	"kigumi/internal/toolchain/module"
)

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
		f.hash, f.imports, f.scanned = module.ContentHash(f.src), module.ScanImports(f.src), true
		m.index.memo.put(f.abs, f.size, f.mtime, f.ctime, f.hash, f.imports)
		return nil
	}
	f.hash, f.imports, f.scanned = module.ContentHash(f.src), module.ScanImports(f.src), true
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
