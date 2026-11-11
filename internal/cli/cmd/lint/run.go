package lint

import (
	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/driver"
)

func run(io session.IO, opts options) error {
	rep, err := session.NewReport(opts.diag, io.Err)
	if err != nil {
		return err
	}
	m, res, err := load(opts)
	if err != nil {
		return err
	}
	if opts.fix {
		if m.HasParseErrors() {
			rep.Text("kigumi: not applying fixes while the module has parse errors\n")
		} else if res != nil {
			changed, err := applySafeFixes(m, append(res.DiagnosticsAll(), m.ParseDiags()...))
			if err != nil {
				return err
			}
			if changed {
				m, res, err = load(opts)
				if err != nil {
					return err
				}
			}
		}
	}
	if ds := m.ParseDiags(); len(ds) > 0 {
		rep.Add(sources(m, res), ds)
	}
	if m.HasParseErrors() {
		return finish(io, rep)
	}
	rep.Add(sources(m, res), res.DiagnosticsAll())
	return finish(io, rep)
}

// load parses the target and checks it unless parsing already failed; lint
// reports the parse diagnostics in that case and stops like check does.
func load(opts options) (*driver.Module, *sem.Result, error) {
	m, err := opts.session.LoadReport(opts.target, false)
	if err != nil {
		return nil, nil, err
	}
	if m.HasParseErrors() {
		return m, nil, nil
	}
	res, err := driver.Check(m)
	if err != nil {
		return nil, nil, errors.WrapErr(err, "check module")
	}
	return m, res, nil
}

func sources(m *driver.Module, res *sem.Result) diag.Sources {
	if m.FileStore != nil {
		return m.FileStore
	}
	if res != nil {
		return res
	}
	return nil
}

// finish writes the baseline and the machine document, then fails on a
// surviving error, or on a warning under --deny-warnings. Warnings alone are
// the ordinary lint result and exit 0.
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
