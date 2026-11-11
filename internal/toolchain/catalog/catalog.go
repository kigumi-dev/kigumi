// Package catalog is the machine-readable contract for the packages std and
// exp publish: dependency layer, stability, availability, audience, support
// and visibility, including the declarations a package exposes only as
// internal SPI. The policy rows live in std/catalog/catalog.json; Generate
// completes them from the declarations and `///` docs, and Validate proves
// every std package and layer is covered.
package catalog

import (
	"slices"

	"kigumi/internal/toolchain/version"
)

const (
	// Format names the catalog document so a reader never guesses from the file name.
	Format = "kigumi.catalog"
	// FormatVersion bumps when a field changes meaning; adding a field does not.
	FormatVersion = version.CatalogFormatVersion
)

// Sys is a build mode a package is available in: a hosted OS, `--sys none`,
// or freestanding.
type Sys string

const (
	SysHosted       Sys = "hosted"
	SysNone         Sys = "none"
	SysFreestanding Sys = "freestanding"
)

// Layer is the dependency layer, mirroring sem.Layer. exp packages have no
// layer because they are not part of the compiler's std layer table.
type Layer string

const (
	LayerNone       Layer = ""
	LayerFoundation Layer = "foundation"
	LayerAllocation Layer = "allocation"
	LayerPlatform   Layer = "platform"
	LayerTool       Layer = "tool"
)

// Stability is how much a package's surface may change.
type Stability string

const (
	StabilityStable       Stability = "stable"
	StabilityProvisional  Stability = "provisional"
	StabilityExperimental Stability = "experimental"
	StabilityDeprecated   Stability = "deprecated"
)

// Audience is who a package is for.
type Audience string

const (
	AudienceUser     Audience = "user"
	AudienceTooling  Audience = "tooling"
	AudienceInternal Audience = "internal"
)

// Support is how strongly the toolchain stands behind a package.
type Support string

const (
	SupportSupported   Support = "supported"
	SupportBestEffort  Support = "best-effort"
	SupportUnverified  Support = "unverified"
	SupportUnsupported Support = "unsupported"
)

// Visibility is the package's place in the public surface. A public package
// may still hide individual declarations as SPI; an internal package has no
// user-facing contract at all.
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityInternal Visibility = "internal"
)

// Availability lists the build modes and targets a package covers. Targets is
// "all" or an OS list (linux, macos, windows, wasi).
type Availability struct {
	Sys     []Sys    `json:"sys"`
	Targets []string `json:"targets"`
}

// SPI is one declaration that generated code may call but user code must not:
// the compiler, the derive generators or the build sandbox depend on it.
type SPI struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose,omitempty"`
}

// Declarations counts a package's public top-level declarations. It is
// generated from the sources, never written in the policy file.
type Declarations struct {
	Types      int `json:"types"`
	Interfaces int `json:"interfaces"`
	Functions  int `json:"functions"`
	Constants  int `json:"constants"`
	Statics    int `json:"statics"`
}

// Entry is one package's catalog row.
type Entry struct {
	Path         string       `json:"path"`
	Layer        Layer        `json:"layer"`
	Stability    Stability    `json:"stability"`
	Availability Availability `json:"availability"`
	Audience     Audience     `json:"audience"`
	Support      Support      `json:"support"`
	Visibility   Visibility   `json:"visibility"`
	Declarations Declarations `json:"declarations"`
	SPI          []SPI        `json:"spi,omitempty"`
}

// Catalog is the whole document.
type Catalog struct {
	Format        string  `json:"format"`
	FormatVersion int     `json:"formatVersion"`
	Packages      []Entry `json:"packages"`
}

// IsExp reports whether the entry describes a package outside std. exp
// modules are not in this repository, so only their policy is checked.
func (e Entry) IsExp() bool { return len(e.Path) >= 4 && e.Path[:4] == "exp/" }

// HasSys reports whether the package is available in mode s.
func (e Entry) HasSys(s Sys) bool { return slices.Contains(e.Availability.Sys, s) }
