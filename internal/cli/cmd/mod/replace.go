package mod

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
)

func runReplace(io session.IO, opts options) error {
	if err := module.AppendLocalReplace(opts.root, opts.args[0], opts.args[1]); err != nil {
		return errors.WrapErr(err, "edit local overrides")
	}
	fmt.Fprintf(io.Msg, "%s now resolves from %s (%s)\n", opts.args[0], opts.args[1], module.LocalName)
	return nil
}
