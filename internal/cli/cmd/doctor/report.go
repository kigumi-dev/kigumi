package doctor

import (
	"fmt"
	"os"

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

// report is the full doctor snapshot.
type report struct {
	version.Provenance
	Std       field `json:"std"`
	Target    field `json:"target"`
	CCompiler field `json:"cCompiler"`
	Zig       field `json:"zig"`
	ModCache  field `json:"modCache"`
	Formats   struct {
		Schema    int `json:"schema"`
		ModJSON   int `json:"modJson"`
		BuildPlan int `json:"buildPlan"`
		TestEvent int `json:"testEvent"`
	} `json:"formatVersions"`
}

// buildReport resolves the session and the host environment into a report.
func buildReport(s session.Session) report {
	r := report{Provenance: version.Current()}
	r.Std = stdField(s)
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
	r := zigtool.ForModule(session.CompilerOptions().ToolLookup(), root, session.ZigPath(), mf.Zig)
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
