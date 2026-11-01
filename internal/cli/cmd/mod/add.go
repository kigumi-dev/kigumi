package mod

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
)

func runAdd(io session.IO, opts options) error {
	r := module.Require{Name: opts.args[0], Version: opts.args[1], URL: opts.url}
	if _, _, ok := module.RepoOf(r); !ok {
		return &exit.UsageError{Err: errors.NewErrf("cannot derive where %s lives; give --url", r.Name)}
	}
	if err := module.AppendRequire(opts.root, r); err != nil {
		return &exit.UsageError{Err: err}
	}
	fmt.Fprintf(io.Msg, "added %s %s; run `kigumi get` to fetch it\n", r.Name, r.Version)
	return nil
}
