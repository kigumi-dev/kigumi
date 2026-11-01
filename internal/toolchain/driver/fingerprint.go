package driver

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"kigumi/internal/backend/llgen"
	mirreach "kigumi/internal/compiler/mir/reach"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/version"
)

// ArtifactFingerprint identifies the exact compiler, target and native ABI
// generation a build output came from; a mismatch needs rebuild or rejection, never a silent link.
type ArtifactFingerprint struct {
	CompilerRelease string `json:"compilerRelease"`
	CompilerCommit  string `json:"compilerCommit"`
	CompilerDirty   bool   `json:"compilerDirty"`
	Target          string `json:"target"`
	AbiRevision     int    `json:"abiRevision"`
	// ZigVersion is "" for a clang build, or a zig build with no pin to check.
	ZigVersion string `json:"zigVersion,omitempty"`
	// AbiBase hashes the op registry and value layout, so it moves when an old runtime object stops fitting this compiler's code.
	AbiBase string `json:"abiBase"`
	// RuntimeAbi adds the runtime flag set (thread profile, unwind, capabilities, user -D); empty where no build ran, as in a plan.
	RuntimeAbi string `json:"runtimeAbi,omitempty"`
	// FingerprintMismatch never compares DeriveProvenance, CapabilityProvision or CapabilityReasons.
	DeriveProvenance    []schema.GeneratedArtifact  `json:"deriveProvenance,omitempty"`
	CapabilityProvision []CapabilityVerdict         `json:"capabilityProvision,omitempty"`
	CapabilityReasons   []mirreach.CapabilityReason `json:"capabilityReasons,omitempty"`
}

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
// targeting t; zigVersion comes from ZigVersionOf.
func CurrentFingerprint(t buildconfig.Target, zigVersion string) ArtifactFingerprint {
	p := version.Current()
	return ArtifactFingerprint{CompilerRelease: p.Release, CompilerCommit: p.Commit, CompilerDirty: p.Dirty, Target: targetIdentity(t), AbiRevision: version.AbiRevision, ZigVersion: zigVersion, AbiBase: llgen.CurrentAbiFingerprint(version.AbiRevision, nil)}
}

// FingerprintMismatch lists, one reason per differing axis, why got was not
// produced by want's exact compiler/target/ABI; nil means they match.
func FingerprintMismatch(got, want ArtifactFingerprint) []string {
	var reasons []string
	if got.CompilerRelease != want.CompilerRelease || got.CompilerCommit != want.CompilerCommit || got.CompilerDirty != want.CompilerDirty {
		reasons = append(reasons, fmt.Sprintf("built by compiler %s@%s%s, this toolchain is %s@%s%s", got.CompilerRelease, short(got.CompilerCommit), dirtySuffix(got.CompilerDirty), want.CompilerRelease, short(want.CompilerCommit), dirtySuffix(want.CompilerDirty)))
	}
	if got.Target != want.Target {
		reasons = append(reasons, fmt.Sprintf("built for target %s, this build targets %s", got.Target, want.Target))
	}
	if got.AbiRevision != want.AbiRevision {
		reasons = append(reasons, fmt.Sprintf("native ABI revision %d, this toolchain uses revision %d", got.AbiRevision, want.AbiRevision))
	}
	if got.AbiBase != want.AbiBase {
		reasons = append(reasons, fmt.Sprintf("runtime ABI %s, this toolchain emits %s", orNone(got.AbiBase), orNone(want.AbiBase)))
	}
	if want.RuntimeAbi != "" && got.RuntimeAbi != want.RuntimeAbi {
		reasons = append(reasons, fmt.Sprintf("runtime fingerprint %s, this build links %s", orNone(got.RuntimeAbi), want.RuntimeAbi))
	}
	if got.ZigVersion != want.ZigVersion {
		reasons = append(reasons, fmt.Sprintf("built with zig %s, this build resolves zig %s", orNone(got.ZigVersion), orNone(want.ZigVersion)))
	}
	return reasons
}

func orNone(v string) string {
	if v == "" {
		return "none"
	}
	return v
}

// FingerprintSidecarPath names the metadata file WriteFingerprintSidecar
// and ReadFingerprintSidecar use alongside a build output such as a bare archive.
func FingerprintSidecarPath(out string) string { return out + ".kgabi.json" }

// WriteFingerprintSidecar records fp next to a build output (an archive,
// or any artifact a later build might consider reusing).
func WriteFingerprintSidecar(out string, fp ArtifactFingerprint) error {
	b, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FingerprintSidecarPath(out), b, 0o644)
}

// ReadFingerprintSidecar reads back what WriteFingerprintSidecar wrote; ok
// is false when out has no sidecar.
func ReadFingerprintSidecar(out string) (fp ArtifactFingerprint, ok bool, err error) {
	b, err := os.ReadFile(FingerprintSidecarPath(out))
	if err != nil {
		if os.IsNotExist(err) {
			return ArtifactFingerprint{}, false, nil
		}
		return ArtifactFingerprint{}, false, err
	}
	if err := json.Unmarshal(b, &fp); err != nil {
		return ArtifactFingerprint{}, false, err
	}
	return fp, true, nil
}

// reportStaleFingerprint never blocks the rebuild; it only surfaces the
// mismatch to stderr.
func reportStaleFingerprint(out string, want ArtifactFingerprint, stderr io.Writer) {
	prev, ok, err := ReadFingerprintSidecar(out)
	if err != nil || !ok {
		return
	}
	if reasons := FingerprintMismatch(prev, want); len(reasons) > 0 {
		fmt.Fprintf(stderr, "note: rebuilding %s: the existing artifact is incompatible (%s)\n", out, reasons[0])
	}
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func dirtySuffix(dirty bool) string {
	if dirty {
		return "+dirty"
	}
	return ""
}

func withProvenance(fp *ArtifactFingerprint, m *Module, t buildconfig.Target, code compiled) {
	if a := m.LastAnalysis(); a != nil {
		fp.DeriveProvenance = a.Provenance
	}
	fp.CapabilityProvision = ProvidedCapabilities(t)
	fp.CapabilityReasons = code.Reasons
	fp.RuntimeAbi = code.Abi
}
