package driver

import "time"

// indexedFile is one source file as the package index knows it: metadata first, content hash and imports only once a caller needs them.
type indexedFile struct {
	name    string
	display string
	abs     string
	size    int64
	mtime   int64
	ctime   int64
	scanned bool
	hash    string
	imports []string
	src     []byte
	loaded  bool
	overlay bool
}

// indexedPackage is a package header: where it lives and which files it has. Nothing is parsed until a root set reaches it.
type indexedPackage struct {
	path   string
	dir    string
	std    bool
	module string
	files  []*indexedFile
	parsed bool
}

type pkgIndex struct {
	pkgs  map[string]*indexedPackage
	order []string
	memo  *indexMemo
}

// IndexStats counts the work behind one module load, so a test can pin that a no-op load reads nothing and parses nothing.
type IndexStats struct {
	PackagesIndexed int
	PackagesParsed  int
	FilesParsed     int
	FilesRead       int
	MemoHits        int
}

func newPkgIndex(cacheDir, root string, o CompilerOptions, now func() time.Time) *pkgIndex {
	return &pkgIndex{pkgs: map[string]*indexedPackage{}, memo: loadIndexMemo(cacheDir, root, o, now)}
}

// IndexStats reports what loading this module read and parsed.
func (m *Module) IndexStats() IndexStats { return m.idxStats }

func (m *Module) saveMemo() {
	if m.index != nil {
		m.index.memo.save()
	}
}
