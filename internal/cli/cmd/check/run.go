package check

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/policy"
)

func run(io session.IO, opts options) error {
	// The default path answers from the package-unit cache and prints exactly
	// what it always has; a machine format or an acceptance filter needs the
	// diagnostics' structure, so it checks the module as a whole.
	if opts.diag.Filtering() {
		return runReport(io, opts)
	}
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

// runReport checks the module with the shared diagnostic report: the machine
// schema, the allow list, the baseline and --deny-warnings all read the same
// structured diagnostics, and the exit code follows what survives them.
func runReport(io session.IO, opts options) error {
	s := opts.session
	rep, err := session.NewReport(opts.diag, io.Err)
	if err != nil {
		return err
	}
	m, err := s.LoadReport(opts.target, false)
	if err != nil {
		return err
	}
	if ds := m.ParseDiags(); len(ds) > 0 {
		var src diag.Sources
		if m.FileStore != nil {
			src = m.FileStore
		}
		rep.Add(src, ds)
	}
	if m.HasParseErrors() {
		return finish(io, rep)
	}
	for _, v := range m.UnusedReplaces {
		rep.Text(fmt.Sprintf("warning[%s]: %s\n", v.ID, v.Subject))
		rep.AddMachine(nil, []diag.Diagnostic{{Severity: diag.Warning, Code: v.ID, Msg: v.Subject}})
	}
	extra := append(s.ReleaseViolations(false), driver.LinkIdentityViolations(m, session.CompilerOptions().Host)...)
	rep.AddPolicy(policy.Evaluate(m.Violations, s.Policy(), extra...))
	if rep.HasErrors() {
		// A policy error refuses the build before the checker runs, as the
		// default path does.
		return finish(io, rep)
	}
	res, err := driver.Check(m)
	if err != nil {
		return errors.WrapErr(err, "check module")
	}
	rep.Add(res, res.DiagnosticsAll())
	addBuildFile(io, opts, m.Root, rep)
	return finish(io, rep)
}

// finish writes the baseline and the machine document, then decides the exit:
// a surviving error, or a warning under --deny-warnings, fails the command.
// --write-baseline records the current state and exits successfully.
func finish(io session.IO, rep *session.Report) error {
	if err := rep.WriteBaseline(); err != nil {
		return errors.WrapErr(err, "write baseline")
	}
	if err := rep.Emit(io.Out); err != nil {
		return errors.WrapErr(err, "write diagnostics")
	}
	if rep.WritingBaseline() {
		return nil
	}
	if rep.Failed() {
		return exit.Code(1)
	}
	return nil
}

// checkBuildFile reports root's build.kg diagnostics next to the module's,
// whether or not the module itself checks: a broken build program never keeps
// `kigumi check` from checking the sources it builds.
func checkBuildFile(io session.IO, opts options, root string) bool {
	bf, ok, err := loadBuildFile(opts, root)
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

// addBuildFile is checkBuildFile for the shared report: the same diagnostics,
// in the human layout and the machine schema.
func addBuildFile(io session.IO, opts options, root string, rep *session.Report) {
	bf, ok, err := loadBuildFile(opts, root)
	if err != nil {
		msg := fmt.Sprintf("%s: %v", driver.BuildFileName, err)
		rep.Text(msg + "\n")
		rep.AddMachine(nil, []diag.Diagnostic{{Severity: diag.Error, Msg: msg}})
		return
	}
	if !ok {
		return
	}
	rep.Add(buildFileSources(bf), bf.Diagnostics())
}

// buildFileSources resolves build.kg's diagnostics: the checked result when
// the program parsed, else the tree's own file.
func buildFileSources(bf *driver.BuildFile) diag.Sources {
	if bf.Res != nil {
		return bf.Res
	}
	if bf.Tree != nil {
		return bf.Tree.File
	}
	return nil
}

func loadBuildFile(opts options, root string) (*driver.BuildFile, bool, error) {
	s := opts.session
	target, err := s.Target()
	if err != nil {
		return nil, false, nil
	}
	return driver.CheckBuildFile(root, driver.LoadOptions{
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
}
