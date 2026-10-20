// Package doctor implements `kigumi doctor`: a read-only report of what the CLI resolved and where each value came from.
package doctor

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/driver"
	"kigumi/internal/version"
	"kigumi/internal/zigtool"
)

// Field is one resolved value, its source, and whether it is usable.
type Field struct {
	Value  string `json:"value"`
	Source string `json:"source"`
	Status string `json:"status"`
}

// Report is the full doctor snapshot.
type Report struct {
	version.Provenance
	Std       Field `json:"std"`
	Target    Field `json:"target"`
	CCompiler Field `json:"cCompiler"`
	Zig       Field `json:"zig"`
	ModCache  Field `json:"modCache"`
	Formats   struct {
		Schema    int `json:"schema"`
		ModJSON   int `json:"modJson"`
		BuildPlan int `json:"buildPlan"`
		TestEvent int `json:"testEvent"`
	} `json:"formatVersions"`
}

// Build resolves cmd's flags and the host environment into a Report.
func Build(cmd *cobra.Command) Report {
	r := Report{Provenance: version.Current()}
	r.Std = stdField(cmd)
	r.Target = targetField(cmd)
	r.CCompiler = ccompilerField()
	r.Zig = zigField()
	r.ModCache = modCacheField()
	r.Formats.Schema = version.SchemaFormatVersion
	r.Formats.ModJSON = version.ModJSONFormatVersion
	r.Formats.BuildPlan = version.BuildPlanFormatVersion
	r.Formats.TestEvent = driver.TestEventFormatVersion
	return r
}

func stdField(cmd *cobra.Command) Field {
	path := shared.StdRoot(cmd)
	source := "default (embedded copy, or a std/ directory found near the executable or working directory)"
	if flag, _ := cmd.Flags().GetString("std"); flag != "" {
		source = "--std"
	} else if _, ambient := shared.AmbientStd(cmd); ambient {
		source = "$KIGUMI_STD"
	}
	return Field{Value: path, Source: source, Status: "ok"}
}

func targetField(cmd *cobra.Command) Field {
	t, err := shared.Target(cmd)
	if err != nil {
		return Field{Status: "invalid", Source: "--target/--sys/--allocator/--profile/--stack-profile"}
	}
	if triple, _ := cmd.Flags().GetString("target"); triple != "" {
		return Field{Value: t.Triple, Source: "--target", Status: "ok"}
	}
	return Field{Value: t.OS + "/" + t.Arch, Source: "host default", Status: "ok"}
}

func ccompilerField() Field {
	argv := shared.CompilerOptions().CCompiler()
	if len(argv) == 0 {
		return Field{Source: "PATH (zig cc, then clang)", Status: "missing"}
	}
	name := argv[0]
	if len(argv) > 1 {
		name += " " + argv[1]
	}
	return Field{Value: name, Source: "PATH (zig cc, then clang)", Status: "ok"}
}

// zigField reads mod.kg directly; no std root is available here to load a full module.
func zigField() Field {
	mf, _, _ := driver.ReadModFile(".")
	root, err := shared.ZigStoreRoot()
	if err != nil {
		root = ""
	}
	r := zigtool.ForModule(shared.CompilerOptions().ToolLookup(), root, shared.ZigPath(), mf.Zig)
	switch {
	case r.Path == "":
		status := "missing"
		if mf.Zig == "" {
			return Field{Source: r.Source, Status: status}
		}
		return Field{Value: fmt.Sprintf("pin %s not resolved: %s", mf.Zig, r.Err), Source: r.Source, Status: status}
	case mf.Zig == "":
		return Field{Value: r.Path, Source: r.Source, Status: "ok"}
	case r.Mismatch:
		return Field{Value: fmt.Sprintf("%s (%s, pin %s not satisfied)", r.Path, r.Version, mf.Zig), Source: r.Source, Status: "mismatch"}
	case r.Err != nil:
		return Field{Value: fmt.Sprintf("%s (version unknown: %s; pin %s)", r.Path, r.Err, mf.Zig), Source: r.Source, Status: "unknown"}
	default:
		return Field{Value: fmt.Sprintf("%s (%s, satisfies pin %s)", r.Path, r.Version, mf.Zig), Source: r.Source, Status: "ok"}
	}
}

func modCacheField() Field {
	dir, err := os.UserCacheDir()
	if err != nil {
		return Field{Status: "unknown", Source: "os.UserCacheDir"}
	}
	return Field{Value: dir + "/kigumi/mod", Source: "os.UserCacheDir()/kigumi/mod (no override today)", Status: "ok"}
}
