package catalog

import "kigumi/internal/base/errors"

// Validate checks the document shape and every row's enums. Tree-derived
// facts (coverage, layers, SPI names) need Validate(stdRoot, c).
func (c *Catalog) Validate() error {
	if c.Format != Format {
		return errors.NewErrf("catalog: format is %q, want %q", c.Format, Format)
	}
	if c.FormatVersion != FormatVersion {
		return errors.NewErrf("catalog: formatVersion is %d, want %d", c.FormatVersion, FormatVersion)
	}
	seen := make(map[string]bool, len(c.Packages))
	var problems []error
	for _, e := range c.Packages {
		if err := e.validate(); err != nil {
			problems = append(problems, err)
		}
		if seen[e.Path] {
			problems = append(problems, errors.NewErrf("catalog: %s appears twice", e.Path))
		}
		seen[e.Path] = true
	}
	return errors.Join(problems...)
}

func (e Entry) validate() error {
	var problems []error
	add := func(format string, args ...any) {
		problems = append(problems, errors.NewErrf("catalog: %s: "+format, append([]any{e.Path}, args...)...))
	}
	switch {
	case e.Path == "":
		add("empty path")
	case e.IsExp():
	case len(e.Path) < 4 || e.Path[:4] != "std/":
		add("path must start with std/ or exp/, got %q", e.Path)
	}
	if !e.Layer.valid() {
		add("unknown layer %q", e.Layer)
	}
	if !e.Stability.valid() {
		add("unknown stability %q", e.Stability)
	}
	if !e.Audience.valid() {
		add("unknown audience %q", e.Audience)
	}
	if !e.Support.valid() {
		add("unknown support %q", e.Support)
	}
	if !e.Visibility.valid() {
		add("unknown visibility %q", e.Visibility)
	}
	if len(e.Availability.Sys) == 0 {
		add("availability.sys is empty")
	}
	for _, s := range e.Availability.Sys {
		switch s {
		case SysHosted, SysNone, SysFreestanding:
		default:
			add("unknown availability sys %q", s)
		}
	}
	if len(e.Availability.Targets) == 0 {
		add("availability.targets is empty")
	}
	for _, t := range e.Availability.Targets {
		switch t {
		case "all", "linux", "macos", "windows", "wasi":
		default:
			add("unknown availability target %q", t)
		}
	}
	if e.Visibility == VisibilityInternal && e.Audience == AudienceUser {
		add("internal visibility cannot have user audience")
	}
	spi := map[string]bool{}
	for _, s := range e.SPI {
		if s.Name == "" {
			add("SPI entry has no name")
		}
		if spi[s.Name] {
			add("SPI %s appears twice", s.Name)
		}
		spi[s.Name] = true
	}
	return errors.Join(problems...)
}

func (l Layer) valid() bool {
	switch l {
	case LayerNone, LayerFoundation, LayerAllocation, LayerPlatform, LayerTool:
		return true
	}
	return false
}

func (s Stability) valid() bool {
	switch s {
	case StabilityStable, StabilityProvisional, StabilityExperimental, StabilityDeprecated:
		return true
	}
	return false
}

func (a Audience) valid() bool {
	switch a {
	case AudienceUser, AudienceTooling, AudienceInternal:
		return true
	}
	return false
}

func (s Support) valid() bool {
	switch s {
	case SupportSupported, SupportBestEffort, SupportUnverified, SupportUnsupported:
		return true
	}
	return false
}

func (v Visibility) valid() bool {
	switch v {
	case VisibilityPublic, VisibilityInternal:
		return true
	}
	return false
}
