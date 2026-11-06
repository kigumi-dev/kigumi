package run

import (
	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func run(io session.IO, opts options) error {
	m, err := opts.session.Load(opts.args[0], false, io.Err)
	if err != nil {
		return err
	}
	code, err := driver.RunWith(m, driver.RunOptions{NoAccel: session.NoAccel(), SecretSan: opts.secretSan}, io.Out, io.Err, opts.args)
	if err != nil {
		return errors.WrapErr(err, "run module")
	}
	if code != 0 {
		return exit.Code(code)
	}
	return nil
}
