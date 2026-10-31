package driver

import (
	"fmt"
	"os"
	"path/filepath"

	buildconfig "kigumi/internal/toolchain/config"
)

// resolveRoot keeps a relative root as given when it names the same directory under Cwd, so process-cwd callers see unchanged paths.
func resolveModuleRoot(o buildconfig.CompilerOptions, root string) (string, error) {
	if filepath.IsAbs(root) {
		return root, nil
	}
	if o.Cwd == "" {
		return "", fmt.Errorf("module root %s is relative and CompilerOptions.Cwd is empty", root)
	}
	abs := filepath.Join(o.Cwd, root)
	st, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("module root %s not found under %s", root, o.Cwd)
	}
	if here, err := os.Stat(root); err == nil && os.SameFile(here, st) {
		return root, nil
	}
	return abs, nil
}

// LoadModule indexes every package under root, resolving a relative root against the options' Cwd.
func LoadModule(root string, opts LoadOptions) (*Module, error) {
	root, err := resolveModuleRoot(opts.Compiler, root)
	if err != nil {
		return nil, err
	}
	return loadModule(root, opts)
}
