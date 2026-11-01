// Package module reads and edits manifests, lock files and local overrides, selects dependency versions and locates the fetched-module cache.
package module

import (
	"fmt"
	"path/filepath"
	"strings"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/isr"
	"kigumi/internal/lang/token"
)

// ManifestName is the manifest that marks a module root. It is written in
// Kigumi record-literal syntax, one record per statement:
//
//	Module { name: "app" }
//
//	Require {
//	    name: "greet"
//	    version: "v0.1.0"
//	    url: "https://example.com/greet.git"
//	}
//	Replace {
//	    name: "greet"
//	    path: "../greet"
//	}
//
// `zig` (on Module, or a standalone `Zig { version: "..." }` record) pins the exact zig version a native build or Header import must resolve.
const ManifestName = "mod.kg"

// LockName records what each Require resolved to.
const LockName = "mod.lock.kg"

// FrameworkFileName describes a Framework: its name, version and the
// external tools a build program may run through it.
const FrameworkFileName = "kigumi.framework.kg"

// IsManifest reports whether a file name is a manifest, a lock file
// (mod.lock.kg or a script's <file>.lock.kg) or the local override; none of
// them is a package file.
func IsManifest(name string) bool {
	return name == ManifestName || name == LocalName || name == FrameworkFileName || strings.HasSuffix(name, ".lock.kg")
}

type ModFile struct {
	Name string
	// Kigumi is the toolchain version the module was written for.
	Kigumi   string
	Requires []Require
	Links    []Link
	// Sources are candidate native sources (a file or a directory of
	// them), relative to the module directory; see Source.
	Sources []NativeSource
	// Headers are C headers to import as generated packages.
	Headers []HeaderImport
	// Zig is the version a native build or Header import must resolve ("" unpinned);
	// only the root module's pin is consulted, never a dependency's.
	Zig string
	// ZigSpan is where Zig was declared (the Module record's `zig` field,
	// or a standalone Zig record), for a build-time resolution diagnostic.
	ZigSpan  token.Span
	Embedder []string
	// Interrupt is the interrupt configuration of `Interrupt` records; the root module's is the limit, a dependency's only narrows its own handlers.
	Interrupt isr.Config
}

// NativeSource is a `Source { c }` or `Source { asm }` record: a C or assembly
// file, or a directory searched for them. A candidate whose name carries a
// platform suffix (start_amd64.s) is compiled only for that target.
type NativeSource struct {
	Path string `json:"path"`
	Asm  bool   `json:"asm,omitempty"`
}

// Link is a C library (`-l`) or search path (`-L`) the module's extern
// blocks need; source files carry no link attributes.
type Link struct {
	Library, Search string
}

type Require struct {
	Name, Version, URL string
}

// HeaderImport is a `Header { path, package }` record: a C header, relative
// to the module directory, imported as the package `c/<package>`
type HeaderImport struct {
	Path, Package string
}

var manifestFields = map[string][]string{
	"Module":    {"name", "kigumi", "zig"},
	"Require":   {"name", "version", "url"},
	"Replace":   {"name", "path"},
	"Link":      {"library", "search"},
	"Source":    {"c", "asm"},
	"Header":    {"path", "package"},
	"Zig":       {"version"},
	"Interrupt": {"handler", "allow"},
	"Embedder":  {"symbol"},
}

// ReadModFile parses root/mod.kg; ok is false when there is none.
func ReadModFile(root string) (ModFile, bool, error) {
	src, ok, err := readFileMaybe(filepath.Join(root, ManifestName))
	if err != nil {
		return ModFile{}, false, err
	}
	if !ok {
		return ModFile{}, false, nil
	}
	f := token.NewFile(ManifestName, src)
	mf, ds := ParseModFile(f)
	if len(ds) > 0 {
		return mf, true, fmt.Errorf("%s: %s", filepath.Join(root, ManifestName), diag.RenderAll(f, ds[:1]))
	}
	return mf, true, nil
}

// ParseModFile decodes a manifest; every problem becomes a diagnostic
// positioned in f so editors can show it.
func ParseModFile(f *token.File) (ModFile, []diag.Diagnostic) {
	recs, ds := ParseRecords(f, manifestFields)
	mf, more := modFileOf(recs, true)
	return mf, append(ds, more...)
}
