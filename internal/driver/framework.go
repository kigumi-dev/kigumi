package driver

import (
	"fmt"
	"path/filepath"

	"kigumi/internal/diag"
	"kigumi/internal/token"
)

// FrameworkDescriptor is a kigumi.framework.kg: the Framework a directory provides and the tools it registers; only listed tools may run.
type FrameworkDescriptor struct {
	Name, Version string
	Dir           string
	Tools         map[string]ToolProgram
}

// ToolProgram is what a registered tool runs: an executable and, for a subcommand like zig cc, its fixed selecting args ahead of the step's own; Env extends the trusted allowlist.
type ToolProgram struct {
	Path string
	Args []string
	Env  []string
}

// builtinToolchainName is the Framework name ResolveGraph answers for Builder.toolchain from the driver's own linked compiler, not a kigumi.framework.kg.
const builtinToolchainName = "toolchain"

// toolchainDescriptor exposes BuildWith's C compiler as tool "cc" and, for zig, its LLD wrapper as "ld"; its cache dir goes under the build's own output dir since the tool env allowlist has no HOME.
func toolchainDescriptor(o CompilerOptions, cacheDir string) FrameworkDescriptor {
	fd := FrameworkDescriptor{Name: builtinToolchainName, Tools: map[string]ToolProgram{}}
	cc := o.CCompiler()
	if len(cc) == 0 {
		return fd
	}
	var env []string
	if filepath.Base(cc[0]) == "zig" && cacheDir != "" {
		env = []string{"ZIG_GLOBAL_CACHE_DIR=" + cacheDir}
	}
	fd.Tools["cc"] = ToolProgram{Path: cc[0], Args: cc[1:], Env: env}
	if filepath.Base(cc[0]) == "zig" {
		fd.Tools["ld"] = ToolProgram{Path: cc[0], Args: []string{"ld.lld"}, Env: env}
	} else if ld, err := o.LookTool("ld.lld"); err == nil {
		fd.Tools["ld"] = ToolProgram{Path: ld}
	} else if ld, err := o.LookTool("ld"); err == nil {
		fd.Tools["ld"] = ToolProgram{Path: ld}
	}
	return fd
}

var frameworkFields = map[string][]string{
	"Framework": {"name", "version"},
	"Tool":      {"name", "program"},
}

// LoadFrameworks reads the descriptor of every root that has one.
func LoadFrameworks(roots []string) ([]FrameworkDescriptor, error) {
	var out []FrameworkDescriptor
	for _, dir := range roots {
		src, ok, err := readFileMaybe(filepath.Join(dir, FrameworkFileName))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		f := token.NewFile(FrameworkFileName, src)
		recs, ds := parseRecords(f, frameworkFields)
		fd := FrameworkDescriptor{Dir: dir, Tools: map[string]ToolProgram{}}
		seenFramework, seenTool := false, map[string]bool{}
		for _, r := range recs {
			switch r.Kind {
			case "Framework":
				if seenFramework {
					ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "only one Framework record is allowed per descriptor"))
					continue
				}
				seenFramework = true
				fd.Name, fd.Version = r.Fields["name"], r.Fields["version"]
			case "Tool":
				if r.Fields["program"] == "" {
					ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Tool needs a program"))
				}
				name := r.Fields["name"]
				if seenTool[name] {
					ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, fmt.Sprintf("tool %q is declared twice in this descriptor", name)))
					continue
				}
				seenTool[name] = true
				fd.Tools[name] = ToolProgram{Path: r.Fields["program"]}
			}
		}
		if fd.Name == "" && len(ds) == 0 {
			ds = append(ds, diag.Errorf(diag.Location{Span: token.Span{}}, "the descriptor needs a Framework record"))
		}
		if len(ds) > 0 {
			return nil, fmt.Errorf("%s: %s", filepath.Join(dir, FrameworkFileName), diag.RenderAll(f, ds[:1]))
		}
		out = append(out, fd)
	}
	return out, nil
}

// program resolves a registered tool to the ToolProgram to execute; a
// relative Path is taken from the descriptor's own directory.
func (fd FrameworkDescriptor) program(tool string) (ToolProgram, bool) {
	tp, ok := fd.Tools[tool]
	if !ok || filepath.IsAbs(tp.Path) {
		return tp, ok
	}
	tp.Path = filepath.Join(fd.Dir, filepath.FromSlash(tp.Path))
	return tp, true
}
