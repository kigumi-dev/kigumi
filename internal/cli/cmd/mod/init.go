package mod

import (
	"fmt"
	"path/filepath"

	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
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
	if err := driver.WriteManifest(opts.root, name); err != nil {
		return &exit.UsageError{Err: err}
	}
	fmt.Fprintf(io.Msg, "wrote %s\n", filepath.Join(opts.root, driver.ManifestName))
	return nil
}
