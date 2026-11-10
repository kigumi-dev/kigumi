// Package version holds the toolchain version that `kigumi --version`
// prints, plus the fuller build provenance `kigumi doctor` and JSON commands report.
package version

import "runtime/debug"

// Toolchain is compared with `Module { kigumi }`, separate from Release below.
const Toolchain = "0.1"

// Release is the human-facing release version.
const Release = "0.1.0"

// Zig is the version std and the examples are pinned to; ZIG_VERSION at the
// repository root mirrors it for CI, which has no Go toolchain built yet.
const Zig = "0.17.0"

// Format versions for machine-readable outputs: each bumps on its own when
// that output's meaning changes; adding a field never bumps it.
const (
	SchemaFormatVersion    = 1
	ModJSONFormatVersion   = 1
	BuildPlanFormatVersion = 1
	DoctorFormatVersion    = 1
	// SchemaIRFormatMajor/Minor version internal/toolchain/contract/schema's envelope; a minor
	// bump only adds fields or node kinds an older reader can ignore.
	SchemaIRFormatMajor = 1
	SchemaIRFormatMinor = 3
)

// AbiRevision names the native Kigumi ABI generation this compiler emits
// (calling convention, layout, drop glue, ...); bump on any incompatible change, independent of Release.
const AbiRevision = 1

// Display is the one-line version `kigumi --version` prints: release, short
// commit when the build has VCS metadata, and a dirty marker.
func Display() string {
	p := Current()
	out := p.Release
	if p.Commit != "" && p.Commit != "unknown" {
		commit := p.Commit
		if len(commit) > 12 {
			commit = commit[:12]
		}
		out += " " + commit
	}
	if p.Dirty {
		out += " dirty"
	}
	return out
}

// Provenance identifies the exact build: commit, dirty working tree and Go
// toolchain, so CLI and LSP both report this instead of hard-coding a version string.
type Provenance struct {
	Release   string `json:"release"`
	Commit    string `json:"commit"`
	Dirty     bool   `json:"dirty"`
	GoVersion string `json:"goVersion"`
}

// Envelope is the common shape of a versioned JSON output: kind names the
// payload, and a consumer within one formatVersion ignores fields it does not know.
type Envelope struct {
	FormatVersion int        `json:"formatVersion"`
	Toolchain     Provenance `json:"toolchain"`
	Kind          string     `json:"kind"`
	Data          any        `json:"data"`
}

// Wrap builds an Envelope around data, stamped with the running build's
// provenance.
func Wrap(kind string, formatVersion int, data any) Envelope {
	return Envelope{FormatVersion: formatVersion, Toolchain: Current(), Kind: kind, Data: data}
}

// Current reads the running binary's embedded module and VCS info; fields
// Go could not determine stay at their zero value ("unknown" for Commit).
func Current() Provenance {
	p := Provenance{Release: Release, Commit: "unknown"}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return p
	}
	p.GoVersion = info.GoVersion
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			p.Commit = s.Value
		case "vcs.modified":
			p.Dirty = s.Value == "true"
		}
	}
	return p
}
