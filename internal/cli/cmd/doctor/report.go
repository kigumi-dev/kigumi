package doctor

import (
	"fmt"
	"os"
	"strings"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/version"
	"kigumi/internal/toolchain/zig"
)

// field is one resolved value, its source, and whether it is usable.
type field struct {
	Value  string `json:"value"`
	Source string `json:"source"`
	Status string `json:"status"`
}

// diagnostic is one version problem the report found: a manifest this
// toolchain cannot read, or a `kigumi` requirement it does not meet.
type diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
}

// manifest is the module manifest's toolchain requirement compared with this
// build. A module's `kigumi` field names the toolchain series it was written
// for, which is also the manifest and metadata format contract it assumes;
// RequiredKigumi and Toolchain are set only on a mismatch, so an equal
// version keeps the field shape previous reports had.
type manifest struct {
	field
	RequiredKigumi string `json:"requiredKigumi,omitempty"`
	Toolchain      string `json:"toolchain,omitempty"`
}

// report is the full doctor snapshot.
type report struct {
	version.Provenance
	Module      manifest     `json:"module"`
	Diagnostics []diagnostic `json:"diagnostics"`
	Std         field        `json:"std"`
	Catalog     catalogField `json:"catalog"`
	Target      field        `json:"target"`
	CCompiler   field        `json:"cCompiler"`
	Zig         field        `json:"zig"`
	ModCache    field        `json:"modCache"`
	Formats     struct {
		Schema    int `json:"schema"`
		ModJSON   int `json:"modJson"`
		BuildPlan int `json:"buildPlan"`
		TestEvent int `json:"testEvent"`
	} `json:"formatVersions"`
}

// buildReport resolves the session and the host environment into a report.
func buildReport(s session.Session) report {
	r := report{Provenance: version.Current()}
	r.Module, r.Diagnostics = manifestField()
	if r.Diagnostics == nil {
		// Always an array: a machine reader should not need a null check.
		r.Diagnostics = []diagnostic{}
	}
	r.Std = stdField(s)
	var diags []diagnostic
	r.Catalog, diags = catalogReport(s.StdRoot())
	r.Diagnostics = append(r.Diagnostics, diags...)
	r.Target = targetField(s)
	r.CCompiler = ccompilerField()
	r.Zig = zigField()
	r.ModCache = modCacheField()
	r.Formats.Schema = version.SchemaFormatVersion
	r.Formats.ModJSON = version.ModJSONFormatVersion
	r.Formats.BuildPlan = version.BuildPlanFormatVersion
	r.Formats.TestEvent = driver.TestEventFormatVersion
	return r
}

func stdField(s session.Session) field {
	path := s.StdRoot()
	source := "default (embedded copy, or a std/ directory found near the executable or working directory)"
	if s.Std != "" {
		source = "--std"
	} else if _, ambient := s.AmbientStd(); ambient {
		source = "$KIGUMI_STD"
	}
	return field{Value: path, Source: source, Status: "ok"}
}

func targetField(s session.Session) field {
	t, err := s.Target()
	if err != nil {
		return field{Status: "invalid", Source: "--target/--sys/--allocator/--profile/--stack-profile"}
	}
	if s.Triple != "" {
		return field{Value: t.Triple, Source: "--target", Status: "ok"}
	}
	return field{Value: t.OS + "/" + t.Arch, Source: "host default", Status: "ok"}
}

func ccompilerField() field {
	argv := session.CompilerOptions().CCompiler()
	if len(argv) == 0 {
		return field{Source: "PATH (zig cc, then clang)", Status: "missing"}
	}
	name := argv[0]
	if len(argv) > 1 {
		name += " " + argv[1]
	}
	return field{Value: name, Source: "PATH (zig cc, then clang)", Status: "ok"}
}

// zigField reads mod.kg directly; no std root is available here to load a full module.
func zigField() field {
	mf, _, _ := module.ReadModFile(".")
	root, err := session.ZigStoreRoot()
	if err != nil {
		root = ""
	}
	r := zig.ForModule(session.CompilerOptions().ToolLookup(), root, session.ZigPath(), mf.Zig)
	switch {
	case r.Path == "":
		status := "missing"
		if mf.Zig == "" {
			return field{Source: r.Source, Status: status}
		}
		return field{Value: fmt.Sprintf("pin %s not resolved: %s", mf.Zig, r.Err), Source: r.Source, Status: status}
	case mf.Zig == "":
		return field{Value: r.Path, Source: r.Source, Status: "ok"}
	case r.Mismatch:
		return field{Value: fmt.Sprintf("%s (%s, pin %s not satisfied)", r.Path, r.Version, mf.Zig), Source: r.Source, Status: "mismatch"}
	case r.Err != nil:
		return field{Value: fmt.Sprintf("%s (version unknown: %s; pin %s)", r.Path, r.Err, mf.Zig), Source: r.Source, Status: "unknown"}
	default:
		return field{Value: fmt.Sprintf("%s (%s, satisfies pin %s)", r.Path, r.Version, mf.Zig), Source: r.Source, Status: "ok"}
	}
}

func modCacheField() field {
	dir, err := os.UserCacheDir()
	if err != nil {
		return field{Status: "unknown", Source: "os.UserCacheDir"}
	}
	return field{Value: dir + "/kigumi/mod", Source: "os.UserCacheDir()/kigumi/mod (no override today)", Status: "ok"}
}

// manifestField compares the manifest's `kigumi` minimum with the running
// toolchain, using the same at-least rule the loader enforces, and reports a
// diagnostic when the toolchain is too old or the manifest cannot be read.
func manifestField() (manifest, []diagnostic) {
	source := "mod.kg"
	mf, ok, err := module.ReadModFile(".")
	if !ok {
		return manifest{field: field{Source: source, Status: "missing"}}, nil
	}
	// ReadModFile still returns the parsed manifest when it rejects it, so
	// a requirement this toolchain misses is reported as a mismatch (with
	// what the module expects and what it runs on) rather than a bare
	// "invalid" row; a manifest written for a newer toolchain is rejected by
	// the loader with exactly this requirement.
	if mf.Kigumi != "" {
		atLeast, verr := version.AtLeast(version.Toolchain, mf.Kigumi)
		if verr == nil && !atLeast {
			msg := fmt.Sprintf("mod.kg requires kigumi %s or newer; this toolchain is %s (schema=%d modJson=%d buildPlan=%d testEvent=%d)",
				mf.Kigumi, version.Toolchain, version.SchemaFormatVersion, version.ModJSONFormatVersion,
				version.BuildPlanFormatVersion, driver.TestEventFormatVersion)
			return manifest{
					field:          field{Value: fmt.Sprintf("needs kigumi %s; this toolchain is %s", mf.Kigumi, version.Toolchain), Source: source, Status: "mismatch"},
					RequiredKigumi: mf.Kigumi,
					Toolchain:      version.Toolchain,
				},
				[]diagnostic{{Code: "toolchain-mismatch", Severity: "error", Message: msg, Source: source}}
		}
	}
	if err != nil {
		msg := firstLine(err.Error())
		return manifest{field: field{Value: msg, Source: source, Status: "invalid"}},
			[]diagnostic{{Code: "manifest-invalid", Severity: "error", Message: msg, Source: source}}
	}
	if mf.Kigumi == "" {
		msg := "mod.kg has no kigumi version, so it was not compared with this toolchain"
		return manifest{field: field{Source: source, Status: "unversioned"}},
			[]diagnostic{{Code: "manifest-unversioned", Severity: "warning", Message: msg, Source: source}}
	}
	return manifest{field: field{Value: mf.Kigumi, Source: source, Status: "ok"}}, nil
}

// firstLine keeps a multi-line parse diagnostic to its first line, where the
// file, code and message are, so the text report stays one row per finding.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
