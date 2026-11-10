package driver

import (
	"fmt"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/toolchain/artifact"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/version"
)

// targetIdentity falls back to OS/Arch/Sys because HostTarget leaves
// Triple empty.
func targetIdentity(t buildconfig.Target) string {
	if t.Triple != "" {
		return t.Triple
	}
	id := fmt.Sprintf("%s-%s-%s", t.OS, t.Arch, t.Sys)
	if t.Allocator != "" {
		id += "+allocator=" + t.Allocator
	}
	if t.Profile != "" {
		id += "+profile=" + t.Profile
	}
	return id
}

// CurrentFingerprint is this toolchain's own fingerprint for a build
// targeting t; zigVersion comes from link.ZigVersionOf.
func CurrentFingerprint(t buildconfig.Target, zigVersion string) artifact.Fingerprint {
	p := version.Current()
	return artifact.Fingerprint{CompilerRelease: p.Release, CompilerCommit: p.Commit, CompilerDirty: p.Dirty, Target: targetIdentity(t), AbiRevision: version.AbiRevision, ZigVersion: zigVersion, AbiBase: abiBase(t)}
}

// abiBase hashes the op registry and value layout under the target they are read for: a pointer width or scalar alignment differs
// per target, so an archive built for one target is never this target's base.
func abiBase(t buildconfig.Target) string {
	return llgen.CurrentAbiFingerprint(version.AbiRevision, []string{"target=" + targetIdentity(t)})
}

// ToolchainMatch lists why fp was not produced by this toolchain for a build targeting t; nil means the sidecar could be this
// toolchain's own. A hosted build records the C library triple its link resolved (static musl on Linux), so the target resolveLinkage
// would pick counts as t as well, and no other target does.
func ToolchainMatch(fp artifact.Fingerprint, t buildconfig.Target) []string {
	var first []string
	for _, c := range fingerprintTargets(t) {
		reasons := artifact.FingerprintMismatch(fp, CurrentFingerprint(c, fp.ZigVersion))
		if len(reasons) == 0 {
			return nil
		}
		if first == nil {
			first = reasons
		}
	}
	return first
}

// fingerprintTargets is t and, for a hosted target with no triple of its own, the static C library triple resolveLinkage would give it.
func fingerprintTargets(t buildconfig.Target) []buildconfig.Target {
	out := []buildconfig.Target{t}
	if t.Bare() || t.Triple != "" {
		return out
	}
	if triple, ok := muslTriples[t.Arch]; ok && t.OS == "linux" {
		static := t
		static.Triple = triple
		out = append(out, static)
	}
	return out
}

func withProvenance(fp *artifact.Fingerprint, m *Module, t buildconfig.Target, code compiled) {
	if a := m.LastAnalysis(); a != nil {
		fp.DeriveProvenance = a.Provenance
	}
	fp.CapabilityProvision = ProvidedCapabilities(t)
	fp.CapabilityReasons = code.Reasons
	fp.RuntimeAbi = code.Abi
}
