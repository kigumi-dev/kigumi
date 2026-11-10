// Package artifact publishes build outputs atomically, records and compares their fingerprints and checks that a build reproduces.
package artifact

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"kigumi/internal/base/fsx"
	"kigumi/internal/toolchain/contract/schema"
)

// Fingerprint identifies the exact compiler, target and native ABI
// generation a build output came from; a mismatch needs rebuild or rejection, never a silent link.
type Fingerprint struct {
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
	DeriveProvenance    []schema.GeneratedArtifact `json:"deriveProvenance,omitempty"`
	CapabilityProvision []CapabilityVerdict        `json:"capabilityProvision,omitempty"`
	CapabilityReasons   []CapabilityReason         `json:"capabilityReasons,omitempty"`
}

type CapabilityVerdict struct {
	Capability string `json:"capability"`
	Provided   bool   `json:"provided"`
	Why        string `json:"why,omitempty"`
}

// FingerprintMismatch lists, one reason per differing axis, why got was not
// produced by want's exact compiler/target/ABI; nil means they match.
func FingerprintMismatch(got, want Fingerprint) []string {
	var reasons []string
	if got.CompilerRelease != want.CompilerRelease || got.CompilerCommit != want.CompilerCommit || got.CompilerDirty != want.CompilerDirty {
		reasons = append(reasons, fmt.Sprintf("built by compiler %s@%s%s, this toolchain is %s@%s%s", got.CompilerRelease, ShortHash(got.CompilerCommit), dirtySuffix(got.CompilerDirty), want.CompilerRelease, ShortHash(want.CompilerCommit), dirtySuffix(want.CompilerDirty)))
	}
	if got.Target != want.Target {
		reasons = append(reasons, fmt.Sprintf("built for target %s, this build targets %s", got.Target, want.Target))
	}
	if got.AbiRevision != want.AbiRevision {
		reasons = append(reasons, fmt.Sprintf("native ABI revision %d, this toolchain uses revision %d", got.AbiRevision, want.AbiRevision))
	}
	if got.AbiBase != want.AbiBase {
		reasons = append(reasons, fmt.Sprintf("runtime ABI %s, this toolchain emits %s", OrNone(got.AbiBase), OrNone(want.AbiBase)))
	}
	if want.RuntimeAbi != "" && got.RuntimeAbi != want.RuntimeAbi {
		reasons = append(reasons, fmt.Sprintf("runtime fingerprint %s, this build links %s", OrNone(got.RuntimeAbi), want.RuntimeAbi))
	}
	if got.ZigVersion != want.ZigVersion {
		reasons = append(reasons, fmt.Sprintf("built with zig %s, this build resolves zig %s", OrNone(got.ZigVersion), OrNone(want.ZigVersion)))
	}
	return reasons
}

// OrNone renders an empty fingerprint field as "none" in a message.
func OrNone(v string) string {
	if v == "" {
		return "none"
	}
	return v
}

// FingerprintSidecarPath names the metadata file WriteFingerprintSidecar
// and ReadFingerprintSidecar use alongside a build output such as a bare archive.
func FingerprintSidecarPath(out string) string { return out + ".kgabi.json" }

// WriteFingerprintSidecar records fp next to a build output (an archive,
// or any artifact a later build might consider reusing). It replaces the
// sidecar atomically, so a reader never parses half of one.
func WriteFingerprintSidecar(out string, fp Fingerprint) error {
	b, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(FingerprintSidecarPath(out), b, 0o644)
}

// ReadFingerprintSidecar reads back what WriteFingerprintSidecar wrote; ok
// is false when out has no sidecar.
func ReadFingerprintSidecar(out string) (fp Fingerprint, ok bool, err error) {
	b, err := os.ReadFile(FingerprintSidecarPath(out))
	if err != nil {
		if os.IsNotExist(err) {
			return Fingerprint{}, false, nil
		}
		return Fingerprint{}, false, err
	}
	if err := json.Unmarshal(b, &fp); err != nil {
		return Fingerprint{}, false, err
	}
	return fp, true, nil
}

// ReportStale never blocks the rebuild; it only surfaces the
// mismatch to stderr.
func ReportStale(out string, want Fingerprint, stderr io.Writer) {
	prev, ok, err := ReadFingerprintSidecar(out)
	if err != nil || !ok {
		return
	}
	if reasons := FingerprintMismatch(prev, want); len(reasons) > 0 {
		fmt.Fprintf(stderr, "note: rebuilding %s: the existing artifact is incompatible (%s)\n", out, reasons[0])
	}
}

// ShortHash cuts a commit id or content hash to 12 characters for a message.
func ShortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

func dirtySuffix(dirty bool) string {
	if dirty {
		return "+dirty"
	}
	return ""
}
