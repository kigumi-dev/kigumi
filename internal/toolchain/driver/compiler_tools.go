package driver

import (
	"kigumi/internal/toolchain/zig"
	"kigumi/internal/toolchain/zig/store"
)

func (o CompilerOptions) ToolLookup() zigtool.Lookup { return zigtool.InDirs(o.PathDirs) }

// CCompiler finds a C compiler able to consume LLVM IR: zig cc or clang.
func (o CompilerOptions) CCompiler() []string {
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
