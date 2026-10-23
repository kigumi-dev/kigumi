package sem

import "kigumi/internal/token"

import "kigumi/internal/syntax"

type Target uint8

const (
	Hosted Target = iota
	Freestanding
)

// Package is one directory of parsed files. Module is "" for the main
// module, "std" for the stubs, any other name for a dependency.
type Package struct {
	Path   string
	Module string
	Files  []*syntax.Tree
	Std    bool
	Entry  *syntax.Tree
}

type Module struct {
	Packages      []*Package
	Target        Target
	NestedModules []string
	// Sources numbers files across the module so diagnostics can point into any of them.
	Sources *token.SourceStore
	// derived marks the second pass, which carries the generated codecs.
	derived bool
	// deriveGen produces derive source; CheckWith always sets it (Check's
	// own is noDeriveGenerator).
	deriveGen DeriveGenerator
	// BuildProgram is the path of the package whose `configure` the build VM enters with no provider bound.
	BuildProgram string
	// NoDefaultAllocator is the `allocator = none` profile: code
	// reachable from the entry may allocate only inside `allocator` blocks.
	NoDefaultAllocator bool
	// RecoverableOnly is the recoverable-only profile: no path reachable
	// from the entry may terminate non-returning on an allocation failure.
	RecoverableOnly bool
	// NoPlatform is the `--sys none` profile. Freestanding
	// implies it, but the driver sets it independently so a hosted target can
	// opt in too.
	NoPlatform bool
	// BlockingProfile is "" (hosted default: warns on a direct-blocking op reached from async) or a strict
	// profile name (isr, alloc-free-executor, single-thread-event-loop, ...) that rejects it instead.
	BlockingProfile string
	// PtrBits is the target pointer width (C's size_t, raw pointers); 0 means 64.
	PtrBits int
	// Align64 is the alignment in bytes of a 64-bit integer or float inside a C struct; 0 means 8.
	Align64 int
	// Arch names the target architecture (amd64, arm64, riscv64, 386, arm,
	// wasm32) for inline asm; "" means amd64.
	Arch string
	// Deny, LayerCeiling and ArtifactName mirror Artifact.availability's
	// declaration: denied import paths, the
	// layer ceiling, and the diagnostic's artifact name.
	Deny         []string
	LayerCeiling string
	ArtifactName string
	// Cancel is polled between phases and before each body; true stops the check.
	Cancel func() bool
	// LazyBodies names the packages whose bodies are checked only when an eagerly checked body names them.
	LazyBodies func(path string) bool
	// OS names the target operating system ("" means a SysV host) for verifying a handler's asm.
	OS string
	// IsrContracts are the reviewed contracts of bodiless std functions; nil trusts none.
	IsrContracts IsrContracts
	// Interrupt is the root module's interrupt configuration.
	Interrupt InterruptConfig
	// DepInterrupt are the dependencies' Interrupt records: they only narrow their own handlers within the root limit.
	DepInterrupt []DepInterrupt
}
