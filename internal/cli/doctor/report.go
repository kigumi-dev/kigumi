// Package doctor implements `kigumi doctor`: a read-only report of what the CLI resolved and where each value came from.
package doctor

import (
	"os"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/driver"
	"kigumi/internal/version"
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
		return Field{Status: "invalid", Source: "--target/--sys/--allocator"}
	}
	if triple, _ := cmd.Flags().GetString("target"); triple != "" {
		return Field{Value: t.Triple, Source: "--target", Status: "ok"}
	}
	return Field{Value: t.OS + "/" + t.Arch, Source: "host default", Status: "ok"}
}

func ccompilerField() Field {
	argv := driver.CCompiler()
	if len(argv) == 0 {
		return Field{Source: "PATH (zig cc, then clang)", Status: "missing"}
	}
	name := argv[0]
	if len(argv) > 1 {
		name += " " + argv[1]
	}
	return Field{Value: name, Source: "PATH (zig cc, then clang)", Status: "ok"}
}

func modCacheField() Field {
	dir, err := os.UserCacheDir()
	if err != nil {
		return Field{Status: "unknown", Source: "os.UserCacheDir"}
	}
	return Field{Value: dir + "/kigumi/mod", Source: "os.UserCacheDir()/kigumi/mod (no override today)", Status: "ok"}
}
