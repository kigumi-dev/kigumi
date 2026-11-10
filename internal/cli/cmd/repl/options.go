package repl

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

type options struct {
	sess             session.Session
	forceInteractive bool
	stdinCells       bool
	command          string
	login            bool
	noRC             bool
	noHistory        bool
}

func parseOptions(cmd *cobra.Command, _ []string) (options, error) {
	f := cmd.Flags()
	o := options{sess: session.Read(cmd)}
	o.forceInteractive, _ = f.GetBool("interactive")
	o.stdinCells, _ = f.GetBool("stdin")
	o.command, _ = f.GetString("command")
	o.login, _ = f.GetBool("login")
	o.noRC, _ = f.GetBool("no-rc")
	o.noHistory, _ = f.GetBool("no-history")
	return o, nil
}
