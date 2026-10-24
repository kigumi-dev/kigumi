package driver

import "io"

// LoadOptions controls which files become part of the module.
type LoadOptions struct {
	// Test includes name_test.kg files.
	Test bool
	// StdRoot is the std directory itself (its subdirectories become the
	// std/... packages); empty skips std.
	StdRoot string
	// Entry names the entry file (root-relative); empty selects the script
	// file of a package under cmd/.
	Entry string
	// NoDeps leaves required modules out (manifest tooling that must work
	// before `kigumi get`).
	NoDeps bool
	// Target selects platform-suffixed files; the zero value is the host.
	Target Target
	// NoLocal ignores mod.local.kg, so a build sees the same versions the
	// module's users get.
	NoLocal bool
	// Overlay maps absolute paths to contents that replace the files on disk
	// (editor buffers); paths that do not exist on disk are added.
	Overlay map[string][]byte
	// Deny and Ceiling are an artifact's declared availability: Deny bars exact std import paths, Ceiling caps std packages by layer; ArtifactName names the declaring artifact.
	Deny         []string
	Ceiling      string
	ArtifactName string
	// Policy governs the release predicate: under Release a local replacement or
	// unlocked dependency-bearing script fails the load instead of only warning.
	Policy BuildPolicy
	// ZigPath is $KIGUMI_ZIG; internal/cli reads the environment and passes
	// it in here. "" resolves zig from PATH instead.
	ZigPath string
	// ZigHome overrides the managed zig store's directory (KIGUMI_ZIG_HOME,
	// or a test); "" uses the default (internal/zigtool/store.Root("")).
	ZigHome string
	// ZigIndexURL overrides ziglang.org's download index, for a test's
	// httptest server; "" uses the real one.
	ZigIndexURL string
	// ZigPublicKey overrides the pinned Zig signing key for a test's fake
	// server; internal/cli never sets this.
	ZigPublicKey string
	// Offline stops ResolveZig from fetching a pinned zig on demand; a pin
	// nothing local satisfies becomes a diagnostic instead.
	Offline bool
	// ZigNotice receives ResolveZig's one-line on-demand-fetch notice; nil
	// means os.Stderr.
	ZigNotice io.Writer
	// RootDirs, when non-nil, makes the own packages at or under these root-relative directories ("" is the whole module) the roots of the package graph instead of every own package.
	RootDirs []string
	// RootImports add packages, by import path, to the roots; the build program starts from what build.kg imports.
	RootImports []string
	// AllOwnPackages keeps every own package a root even when Entry names one, for tools that describe the whole module (doc, schema).
	AllOwnPackages bool
	// AllPackages makes every indexed package a root, dependencies and unimported ones included, as the editor's auto-import needs.
	AllPackages bool
	// DeferParse stops the load after the package index and root set: nothing is parsed until a call needs the trees, so a settled module can answer from the package cache without reading sources.
	DeferParse bool
	// UnitCheck lets CheckUnits skip packages whose cached outcome is clean; it needs PackageCache.
	UnitCheck bool
	// LazyStd checks a std body only when the module names it; Check does so under UnitCheck too.
	LazyStd bool
	// PackageCache is the directory holding the index memo, package API artifacts and settled check results; "" turns persistence off.
	PackageCache string
	// VerifyPasses verifies the MIR before the passes and after each one, not only after the last.
	VerifyPasses bool
	// FileOrder reorders each directory's files and the package list after the name sort; tests only.
	FileOrder func(names []string)
	// Compiler carries the process facts the compiler consults; internal/cli fills it in.
	Compiler CompilerOptions
	// headerTriple, when set, is the triple Header records translate against instead of the default link's.
	headerTriple *string
}
