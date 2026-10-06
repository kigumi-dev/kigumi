package driver

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"kigumi/internal/schema"
	"kigumi/internal/version"
)

// ArtifactFingerprint identifies the exact compiler, target and native ABI
// generation a build output came from; a mismatch needs rebuild or rejection, never a silent link.
type ArtifactFingerprint struct {
	CompilerRelease string `json:"compilerRelease"`
	CompilerCommit  string `json:"compilerCommit"`
	CompilerDirty   bool   `json:"compilerDirty"`
	Target          string `json:"target"`
	AbiRevision     int    `json:"abiRevision"`
	// FingerprintMismatch never compares DeriveProvenance.
	DeriveProvenance []schema.GeneratedArtifact `json:"deriveProvenance,omitempty"`
}

// targetIdentity falls back to OS/Arch/Sys because HostTarget leaves
// Triple empty.
func targetIdentity(t Target) string {
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
// targeting t.
func CurrentFingerprint(t Target) ArtifactFingerprint {
	p := version.Current()
	return ArtifactFingerprint{CompilerRelease: p.Release, CompilerCommit: p.Commit, CompilerDirty: p.Dirty, Target: targetIdentity(t), AbiRevision: version.AbiRevision}
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
	return reasons
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
func reportStaleFingerprint(out string, t Target, stderr io.Writer) {
	prev, ok, err := ReadFingerprintSidecar(out)
	if err != nil || !ok {
		return
	}
	if reasons := FingerprintMismatch(prev, CurrentFingerprint(t)); len(reasons) > 0 {
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
