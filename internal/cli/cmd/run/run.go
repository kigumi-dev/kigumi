package run

import (
	stderrors "errors"

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
		// A module with no entry file is the caller's target problem, not a VM refusal: `check` accepts the same module,
		// so `run` has to name what is missing instead of leaking the interpreter's wording.
		if stderrors.Is(err, driver.ErrNoEntry) {
			return errors.NewErrf("no entry file (main.kg) in module %s", m.Root)
		}
		return errors.WrapErr(err, "run module")
	}
	if code != 0 {
		return exit.Code(code)
	}
	return nil
}
