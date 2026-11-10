package build

import (
	"fmt"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/policy"
)

func run(io session.IO, opts options) error {
	if !strings.HasSuffix(opts.root, ".kg") {
		if _, hasProgram, err := driver.ReadBuildFile(opts.root); err != nil {
			fmt.Fprint(io.Err, err.Error())
			return exit.Code(1)
		} else if hasProgram {
			if opts.output != "" || opts.emit {
				return &exit.UsageError{Err: errors.NewErr("a module with build.kg takes --out-dir and --plan, not -o or --emit-llvm")}
			}
			return runProgram(io, opts)
		}
	}
	if opts.programOnly {
		return &exit.UsageError{Err: errors.NewErr("a module without build.kg takes -o or --emit-llvm, not --out-dir, --plan, or --secret")}
	}
	if len(opts.extra) > 0 {
		return &exit.UsageError{Err: errors.NewErrf("extra arguments after %s belong to a build.kg, which this module has none of", opts.root)}
	}
	return runModule(io, opts)
}

func runModule(io session.IO, opts options) error {
	s := opts.session
	pol := s.Policy()
	m, err := s.Load(opts.root, false, io.Err)
	if err != nil {
		return err
	}
	extra := s.ReleaseViolations(true)
	// An invalid --target surfaces below with usage context; here it only skips the link-identity check.
	if linkTarget, err := s.Target(); err == nil {
		extra = append(extra, driver.LinkIdentityViolations(m, linkTarget)...)
	}
	report := policy.Evaluate(m.Violations, pol, extra...)
	fmt.Fprint(io.Err, report.Render())
	if report.HasErrors() {
		return exit.Code(1)
	}
	target, err := s.Target()
	if err != nil {
		return &exit.ValueError{Err: err}
	}
	if opts.emit {
		var emitted driver.BuildReport
		ir, ok, err := driver.CompileWith(m, io.Err, driver.BuildOptions{Target: target, CFlags: session.CFlags(), Link: opts.shape.link, SizeMin: opts.shape.sizeMin, Report: &emitted})
		if err != nil {
			return errors.WrapErr(err, "compile module")
		}
		if !ok {
			return exit.Code(1)
		}
		fmt.Fprint(io.Out, ir)
		fmt.Fprint(io.Err, buildSummary(s, m, pol, false, nil, map[string]driver.BuildReport{"": emitted}).Render())
		return nil
	}
	out := opts.output
	if out == "" {
		out = "a.out"
		if target.Bare() {
			out = "kigumi.a"
		}
	}
	var built driver.BuildReport
	ok, err := driver.BuildWith(m, out, io.Err, driver.BuildOptions{Target: target, CFlags: session.CFlags(), Reproducible: opts.reproducible, Env: session.CCEnv(), Link: opts.shape.link, SizeMin: opts.shape.sizeMin, Report: &built})
	if err != nil {
		return errors.WrapErr(err, "build module")
	}
	if !ok {
		return exit.Code(1)
	}
	fmt.Fprint(io.Err, buildSummary(s, m, pol, opts.reproducible, nil, map[string]driver.BuildReport{"": built}).Render())
	return nil
}
