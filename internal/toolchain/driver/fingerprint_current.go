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
	return artifact.Fingerprint{CompilerRelease: p.Release, CompilerCommit: p.Commit, CompilerDirty: p.Dirty, Target: targetIdentity(t), AbiRevision: version.AbiRevision, ZigVersion: zigVersion, AbiBase: llgen.CurrentAbiFingerprint(version.AbiRevision, nil)}
}

func withProvenance(fp *artifact.Fingerprint, m *Module, t buildconfig.Target, code compiled) {
	if a := m.LastAnalysis(); a != nil {
		fp.DeriveProvenance = a.Provenance
	}
	fp.CapabilityProvision = ProvidedCapabilities(t)
	fp.CapabilityReasons = code.Reasons
	fp.RuntimeAbi = code.Abi
}
