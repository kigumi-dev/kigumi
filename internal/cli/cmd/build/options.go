package build

import (
	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

type options struct {
	session      session.Session
	root         string
	extra        []string
	output       string
	emit         bool
	plan         bool
	outDir       string
	secrets      []string
	reproducible bool
	programOnly  bool
	shape        outputShape
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	f := cmd.Flags()
	opts := options{session: session.Read(cmd), root: args[0], extra: args[1:]}
	opts.output, _ = f.GetString("output")
	opts.emit, _ = f.GetBool("emit-llvm")
	opts.plan, _ = f.GetBool("plan")
	opts.outDir, _ = f.GetString("out-dir")
	opts.secrets, _ = f.GetStringArray("secret")
	opts.reproducible, _ = f.GetBool("reproducible")
	opts.programOnly = f.Changed("plan") || f.Changed("out-dir") || f.Changed("secret")
	if opts.reproducible && opts.emit {
		return opts, &exit.UsageError{Err: errors.NewErr("--reproducible and --emit-llvm are incompatible")}
	}
	link, _ := f.GetString("link")
	mode, err := driver.ParseLinkMode(link)
	if err != nil {
		return opts, &exit.ValueError{Err: err}
	}
	opts.shape.link = mode
	opts.shape.sizeMin, _ = f.GetBool("size-min")
	return opts, nil
}
