package config

import (
	"os"

	"kigumi/internal/toolchain/zig"
	"kigumi/internal/toolchain/zig/store"
)

func (o CompilerOptions) ToolLookup() zig.Lookup { return zig.InDirs(o.PathDirs) }

// noCCompilerEnv makes CCompiler report no compiler even when one is on the search path. The
// hermetic CI job sets it so the suite proves it needs no C toolchain instead of silently using
// the runner image's clang.
const noCCompilerEnv = "KIGUMI_TEST_NO_CC"

// CCompiler finds a C compiler able to consume LLVM IR: zig cc or clang.
func (o CompilerOptions) CCompiler() []string {
	if os.Getenv(noCCompilerEnv) != "" {
		return nil
	}
	look := o.ToolLookup()
	if p, err := look("zig"); err == nil {
		return []string{p, "cc"}
	}
	if p, err := look("clang"); err == nil {
		return []string{p}
	}
	return nil
}

func (o CompilerOptions) LookTool(name string) (string, error) { return o.ToolLookup()(name) }

// ZigStoreRoot is the managed zig store: override, else under the options' Home; neither is an error.
func (o CompilerOptions) ZigStoreRoot(override string) (string, error) {
	return store.Root(override, o.Home)
}
