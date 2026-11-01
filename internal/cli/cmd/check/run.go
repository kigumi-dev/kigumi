package check

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/policy"
)

func run(io session.IO, opts options) error {
	s := opts.session
	m, err := s.LoadSettled(opts.target, false, io.Err)
	if err != nil {
		return err
	}
	extra := append(s.ReleaseViolations(false), driver.LinkIdentityViolations(m, session.CompilerOptions().Host)...)
	report := policy.Evaluate(m.Violations, s.Policy(), extra...)
	fmt.Fprint(io.Err, report.Render())
	if report.HasErrors() {
		return exit.Code(1)
	}
	res, err := driver.CheckUnits(m)
	if err != nil {
		return errors.WrapErr(err, "check module")
	}
	fmt.Fprint(io.Err, res.Render())
	if res.HasErrors() {
		return exit.Code(1)
	}
	return nil
}
