// Package cheader imports a C header into Kigumi extern(C) declarations by
// running `zig translate-c` and reading its output, instead of embedding a
// C parser.
package cheader

import (
	"fmt"
	"os"
	"path/filepath"
)

// Options selects one header import.
type Options struct {
	// ZigPath is the zig binary to run (internal/driver's own lookup).
	ZigPath string
	// Target is the compile triple translate-c should see, matching the
	// current build (internal/driver Target.compileTriple()); "" for host.
	Target string
	ABI    ABI
	// HeaderPath is the header's absolute path.
	HeaderPath string
	// CacheDir is the base directory Import keys entries under; the
	// driver points it at the module cache root it already uses.
	CacheDir string
	// Run overrides how translate-c is invoked; nil uses RunZig.
	Run Runner
	// DepRun overrides how the include closure is discovered; nil uses
	// RunZigDeps.
	DepRun DepRunner
}

// Result is one header's generated Kigumi source.
type Result struct {
	// Source is the generated package's file content.
	Source string
	// Skipped lists what the header declared that this importer left
	// out, one entry per declaration; nil when
	// nothing was skipped.
	Skipped []string
	// FromCache is true when Source came from a prior run's cache entry
	// rather than a fresh zig translate-c invocation.
	FromCache bool
	// CachePath is the generated file's path on disk, so the driver can
	// load it like any other package file (real spans for diagnostics,
	// hover and doc).
	CachePath string
	// Manifest is the published cache entry's full identity.
	Manifest Manifest
}

// translateArgv is the non-path flags RunZig passes to translate-c; part of
// the cache key, unlike HeaderPath (closureSources hashes its content instead).
func translateArgv(target string) []string {
	argv := []string{"translate-c", "-lc"}
	if target != "" {
		argv = append(argv, "-target", target)
	}
	return argv
}

// closureSources hashes headerPath and every file depRun reported; one that
// no longer exists by the time this runs is dropped, not a failure.
func closureSources(headerPath string, deps []string) ([]SourceFile, error) {
	seen := map[string]bool{}
	var out []SourceFile
	add := func(path string, entry bool) error {
		if seen[path] {
			return nil
		}
		seen[path] = true
		sum, err := hashFile(path)
		if err != nil {
			if entry {
				return err
			}
			return nil
		}
		out = append(out, SourceFile{Path: path, Sha256: sum, Entry: entry})
		return nil
	}
	if err := add(headerPath, true); err != nil {
		return nil, err
	}
	for _, d := range deps {
		if err := add(d, false); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Import keys on opts.HeaderPath's full include closure and, on a miss,
// invokes zig translate-c and publishes a fresh cache entry.
func Import(opts Options) (Result, error) {
	run := opts.Run
	if run == nil {
		run = RunZig
	}
	depRun := opts.DepRun
	if depRun == nil {
		depRun = RunZigDeps
	}
	zigDigest, err := zigFingerprint(opts.ZigPath)
	if err != nil {
		return Result{}, err
	}
	deps, err := depRun(opts.ZigPath, opts.Target, opts.HeaderPath)
	if err != nil {
		return Result{}, fmt.Errorf("discovering %s's includes: %w", opts.HeaderPath, err)
	}
	closure, err := closureSources(opts.HeaderPath, deps)
	if err != nil {
		return Result{}, err
	}
	argv := translateArgv(opts.Target)
	dir := filepath.Join(opts.CacheDir, cacheKey(closure, zigDigest, argv, opts.ABI))
	genPath := filepath.Join(dir, "gen.kg")
	if mf, ok := readManifest(dir); ok {
		if src, err := os.ReadFile(genPath); err == nil {
			return Result{Source: string(src), Skipped: mf.Skipped, FromCache: true, CachePath: genPath, Manifest: mf}, nil
		}
	}
	full, baseline, err := translateBoth(run, opts.ZigPath, opts.Target, opts.HeaderPath)
	if err != nil {
		return Result{}, fmt.Errorf("importing %s: %w", opts.HeaderPath, err)
	}
	source, skipped := Generate(full, baseline, opts.ABI)
	mf := Manifest{
		FormatVersion:     formatVersion,
		ZigDigest:         zigDigest,
		TranslatorVersion: zigVersionString(opts.ZigPath),
		Target:            opts.Target,
		ABI:               opts.ABI,
		Argv:              argv,
		Sources:           closure,
		Skipped:           skipped,
	}
	if err := os.MkdirAll(opts.CacheDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := publishEntry(opts.CacheDir, dir, source, mf); err != nil {
		return Result{}, err
	}
	return Result{Source: source, Skipped: skipped, FromCache: false, CachePath: genPath, Manifest: mf}, nil
}
