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
	if checkBuildFile(io, opts, m.Root) || res.HasErrors() {
		return exit.Code(1)
	}
	return nil
}

// checkBuildFile reports root's build.kg diagnostics next to the module's,
// whether or not the module itself checks: a broken build program never keeps
// `kigumi check` from checking the sources it builds.
func checkBuildFile(io session.IO, opts options, root string) bool {
	s := opts.session
	target, err := s.Target()
	if err != nil {
		return false
	}
	bf, ok, err := driver.CheckBuildFile(root, driver.LoadOptions{
		StdRoot:      s.StdRoot(),
		Target:       target,
		Policy:       s.Policy(),
		NoLocal:      s.NoLocal || session.LocalOff(),
		Offline:      s.Offline,
		ZigPath:      session.ZigPath(),
		ZigHome:      session.ZigHome(),
		VerifyPasses: session.VerifyPasses(),
		Compiler:     session.CompilerOptions(),
		PackageCache: session.PackageCache(),
	})
	if err != nil {
		fmt.Fprintf(io.Err, "%s: %v\n", driver.BuildFileName, err)
		return true
	}
	if !ok {
		return false
	}
	fmt.Fprint(io.Err, bf.Render())
	return bf.HasErrors()
}
