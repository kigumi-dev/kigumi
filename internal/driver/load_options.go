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
	// VerifyPasses verifies the MIR before the passes and after each one, not only after the last.
	VerifyPasses bool
	// FileOrder reorders each directory's files and the package list after the name sort; tests only.
	FileOrder func(names []string)
	// Compiler carries the process facts the compiler consults; internal/cli fills it in.
	Compiler CompilerOptions
}
