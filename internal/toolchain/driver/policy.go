package driver

// DependencyPolicy says whether a dependency-bearing script may build
// without a sidecar lock. The zero value is the development default.
type DependencyPolicy string

const (
	DependencyAllowUnlockedScript DependencyPolicy = ""
	DependencyLocked              DependencyPolicy = "locked"
)

// LocalOverridePolicy says whether mod.local.kg may redirect a dependency.
// The zero value is the development default.
type LocalOverridePolicy string

const (
	LocalOverrideAllow LocalOverridePolicy = ""
	LocalOverrideDeny  LocalOverridePolicy = "deny"
)

// AmbientPolicy says whether an input taken from the process environment
// (KIGUMI_STD, KIGUMI_CFLAGS) is acceptable; the zero value is the development default.
type AmbientPolicy string

const (
	AmbientWarn AmbientPolicy = ""
	AmbientDeny AmbientPolicy = "deny"
)

// BuildPolicy is a resolved build policy: independent fields a release predicate
// reads, never a bool it branches on. Only CLI flag parsing picks a preset.
type BuildPolicy struct {
	// Optimized records the build's declared optimization intent for the
	// summary; it does not by itself change codegen flags.
	Optimized     bool
	Dependency    DependencyPolicy
	LocalOverride LocalOverridePolicy
	Ambient       AmbientPolicy
	// PolicyVersion names the policy for the build summary; "" prints as "development".
	PolicyVersion string
}

// Development is the default build policy (and BuildPolicy's zero value): a local
// replacement, unlocked script, or ambient override is a warning, never an error.
func Development() BuildPolicy { return BuildPolicy{} }

// Release is the --release build policy: the same violations Development
// warns about are build errors instead.
func Release() BuildPolicy {
	return BuildPolicy{
		Optimized:     true,
		Dependency:    DependencyLocked,
		LocalOverride: LocalOverrideDeny,
		Ambient:       AmbientDeny,
		PolicyVersion: "release-v1",
	}
}

// IsRelease reports whether p rejects the violations Development only warns about.
func (p BuildPolicy) IsRelease() bool { return p.PolicyVersion != "" }

// String names the policy for the build summary.
func (p BuildPolicy) String() string {
	if p.PolicyVersion == "" {
		return "development"
	}
	return p.PolicyVersion
}
