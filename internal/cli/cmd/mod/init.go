package mod

import (
	"fmt"
	"path/filepath"

	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
)

func runInit(io session.IO, opts options) error {
	name := ""
	if len(opts.args) == 1 {
		name = opts.args[0]
	} else {
		abs, err := session.CompilerOptions().Abs(opts.root)
		if err != nil {
			return err
		}
		name = filepath.Base(abs)
	}
	if err := module.WriteManifest(opts.root, name); err != nil {
		return &exit.ValueError{Err: err}
	}
	fmt.Fprintf(io.Err, "wrote %s\n", filepath.Join(opts.root, module.ManifestName))
	return nil
}
