package mod

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func runReplace(io session.IO, opts options) error {
	if err := driver.AppendLocalReplace(opts.root, opts.args[0], opts.args[1]); err != nil {
		return errors.WrapErr(err, "edit local overrides")
	}
	fmt.Fprintf(io.Msg, "%s now resolves from %s (%s)\n", opts.args[0], opts.args[1], driver.LocalName)
	return nil
}
