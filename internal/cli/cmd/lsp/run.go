package lsp

import (
	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	langsrv "kigumi/internal/tooling/lsp"
)

func run(_ session.IO, opts options) error {
	target, err := opts.session.Target()
	if err != nil {
		return err
	}
	return errors.WrapErr(langsrv.Serve(opts.session.StdRoot(), target, session.PackageCache(), session.CompilerOptions(), session.UnitCheck()), "language server")
}
