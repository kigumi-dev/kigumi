package driver

import (
	"os"
	"os/exec"

	"kigumi/internal/zigtool"
	"kigumi/internal/zigtool/store"
)

func (o CompilerOptions) ToolLookup() zigtool.Lookup {
	if o.PathDirs != nil {
		return zigtool.InDirs(o.PathDirs)
	}
	return exec.LookPath
}

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

// CCompiler is CompilerOptions{}.CCompiler: the process PATH decides.
func CCompiler() []string { return CompilerOptions{}.CCompiler() }

func (o CompilerOptions) LookTool(name string) (string, error) { return o.ToolLookup()(name) }

// ZigStoreRoot is the managed zig store: override, else under the options' Home.
func (o CompilerOptions) ZigStoreRoot(override string) (string, error) {
	home := o.Home
	if home == "" && override == "" {
		home, _ = os.UserHomeDir()
	}
	return store.Root(override, home)
}
