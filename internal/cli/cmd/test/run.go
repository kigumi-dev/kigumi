package test

import (
	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func run(io session.IO, opts options) error {
	m, err := opts.session.Load(opts.target, true, io.Err)
	if err != nil {
		return err
	}
	code, err := driver.RunTestsReportingWith(m, opts.tests, io.Out, io.Err)
	if err != nil {
		return errors.WrapErr(err, "run tests")
	}
	if code != 0 {
		return exit.Code(code)
	}
	return nil
}
