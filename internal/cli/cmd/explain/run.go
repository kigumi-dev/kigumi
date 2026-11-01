package explain

import (
	"fmt"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/compiler/sem"
)

func run(io session.IO, opts options) error {
	switch {
	case opts.fn != "":
		return explainOp(io, opts.session, opts.fn, opts.args[0])
	case opts.capabilities:
		return explainCapabilities(io, opts.session, opts.args[0])
	case len(opts.args) == 0:
		for _, c := range sem.Codes() {
			fmt.Fprintf(io.Out, "%s  %s\n", c.Num, c.Name)
		}
		return nil
	}
	name := opts.args[0]
	want := strings.ToUpper(name)
	for _, c := range sem.Codes() {
		if c.Num != want && c.Name != name {
			continue
		}
		fmt.Fprintf(io.Out, "%s (%s)\n\n  %s\n", c.Num, c.Name, c.Template)
		if c.Help != "" {
			fmt.Fprintf(io.Out, "\n  help: %s\n", c.Help)
		}
		return nil
	}
	return &exit.UsageError{Err: errors.NewErrf("unknown diagnostic code %q; `kigumi explain` lists them", name)}
}
