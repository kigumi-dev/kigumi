package syntax

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"kigumi/internal/lang/token"
)

// ModuleID is a content-derived module identity: a SHA-256 over the module
// name and its sorted package identities. Two loads that read the same bytes
// get the same ModuleID regardless of package or file enumeration order.
type ModuleID string

// PackageID is a content-derived package identity: a SHA-256 over the
// dependency module, the import path and the sorted file identities. It does
// not move when a sibling package or an unrelated file is added.
type PackageID string

// FileID is a content-derived file identity within a ParseSnapshot.
type FileID string

// ParsePackage is one package offered to NewParseSnapshot.
type ParsePackage struct {
	// Path is the canonical import path.
	Path string
	// Module is the dependency the package came from; "" for the module
	// being built.
	Module string
	// Std marks declaration stubs.
	Std bool
	// Files are the parsed sources; their File.Name and File.Src are read.
	Files []*token.File
}

// ParseSnapshot is the immutable identity of parsed input. The checker keeps
// its own mutable tree builder, so the snapshot records the source files and
// their content-derived ids rather than aliasing a tree the checker rewrites.
type ParseSnapshot struct {
	root    string
	module  ModuleID
	pkgs    []ParsePackage
	pkgIDs  map[string]PackageID
	fileIDs map[fileKey]FileID
}

type fileKey struct{ pkg, name string }

// NewParseSnapshot sorts its input by module, path and file name, so neither
// package enumeration nor file read order can change any id.
func NewParseSnapshot(root string, pkgs []ParsePackage) *ParseSnapshot {
	s := &ParseSnapshot{
		root:    root,
		pkgIDs:  map[string]PackageID{},
		fileIDs: map[fileKey]FileID{},
	}
	s.pkgs = make([]ParsePackage, 0, len(pkgs))
	for _, p := range pkgs {
		files := append([]*token.File(nil), p.Files...)
		slices.SortStableFunc(files, func(a, b *token.File) int {
			return strings.Compare(fileName(a), fileName(b))
		})
		s.pkgs = append(s.pkgs, ParsePackage{Path: p.Path, Module: p.Module, Std: p.Std, Files: files})
	}
	slices.SortStableFunc(s.pkgs, func(a, b ParsePackage) int {
		if c := strings.Compare(a.Module, b.Module); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	pkgIDs := make([]PackageID, 0, len(s.pkgs))
	for _, p := range s.pkgs {
		id := packageIdentity(p.Module, p.Path, p.Files)
		s.pkgIDs[p.Path] = id
		pkgIDs = append(pkgIDs, id)
		for _, f := range p.Files {
			if f == nil {
				continue
			}
			s.fileIDs[fileKey{pkg: p.Path, name: f.Name}] = fileIdentity(p.Path, f)
		}
	}
	s.module = ModuleIdentity(root, pkgIDs)
	return s
}

// ModuleID returns the snapshot's content-derived module identity.
func (s *ParseSnapshot) ModuleID() ModuleID { return s.module }

// Packages returns a copy of the package descriptors, in a stable order.
func (s *ParseSnapshot) Packages() []ParsePackage {
	if s == nil {
		return nil
	}
	out := make([]ParsePackage, len(s.pkgs))
	for i, p := range s.pkgs {
		out[i] = ParsePackage{Path: p.Path, Module: p.Module, Std: p.Std, Files: append([]*token.File(nil), p.Files...)}
	}
	return out
}

// PackageID returns the content-derived id of path.
func (s *ParseSnapshot) PackageID(path string) (PackageID, bool) {
	if s == nil {
		return "", false
	}
	id, ok := s.pkgIDs[path]
	return id, ok
}

// FileID returns the content-derived id of path's file name.
func (s *ParseSnapshot) FileID(path, name string) (FileID, bool) {
	if s == nil {
		return "", false
	}
	id, ok := s.fileIDs[fileKey{pkg: path, name: name}]
	return id, ok
}

// PackageIdentity is the content-derived id of one package, for callers that
// hold parsed files rather than a ParseSnapshot (the semantic result).
func PackageIdentity(module, path string, files []*token.File) PackageID {
	return packageIdentity(module, path, files)
}

// ModuleIdentity is the content-derived id of a module from its package ids.
func ModuleIdentity(root string, pkgIDs []PackageID) ModuleID {
	ids := make([]string, len(pkgIDs))
	for i, id := range pkgIDs {
		ids[i] = string(id)
	}
	slices.Sort(ids)
	return ModuleID(digest(append([]string{"kigumi/module/1", root}, ids...)...))
}

func packageIdentity(module, path string, files []*token.File) PackageID {
	ids := make([]string, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		ids = append(ids, string(fileIdentity(path, f)))
	}
	slices.Sort(ids)
	parts := append([]string{"kigumi/pkg/1", module, path}, ids...)
	return PackageID(digest(parts...))
}

func fileIdentity(pkg string, f *token.File) FileID {
	return FileID(digest("kigumi/file/1", pkg, f.Name, contentDigest(f.Src)))
}

func fileName(f *token.File) string {
	if f == nil {
		return ""
	}
	return f.Name
}

func contentDigest(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])
}

// digest hashes its parts with a separator no file name or path contains, so
// ("a/b","c") and ("a","b/c") stay distinct.
func digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
