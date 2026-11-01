package schema

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/tooling/doc"
)

func run(io session.IO, opts options) error {
	m, err := opts.session.LoadWhole(opts.target, true, io.Err)
	if err != nil {
		return err
	}
	res, err := driver.Check(m)
	if err != nil {
		return errors.WrapErr(err, "check module")
	}
	if res.HasErrors() {
		fmt.Fprint(io.Err, res.Render())
		if !opts.allowErrors {
			return exit.Code(1)
		}
	}
	if !opts.ir {
		fmt.Fprint(io.Out, doc.SchemaJSON(doc.Declarations(m, res, opts.attr)))
		return nil
	}
	tgt, err := opts.session.Target()
	if err != nil {
		return &exit.UsageError{Err: err}
	}
	target := tgt.Triple
	if target == "" {
		target = tgt.OS + "-" + tgt.Arch
	}
	out, err := doc.SchemaIRJSON(m, res, opts.attr, target, tgt.Profile)
	if err != nil {
		return errors.WrapErr(err, "build schema IR")
	}
	fmt.Fprintln(io.Out, string(out))
	return nil
}
