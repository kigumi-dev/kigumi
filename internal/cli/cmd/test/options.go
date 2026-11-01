package test

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

type options struct {
	session session.Session
	target  string
	tests   driver.TestRunOptions
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	f := cmd.Flags()
	opts := options{session: session.Read(cmd), target: args[0]}
	opts.tests.RunOptions = driver.RunOptions{NoAccel: session.NoAccel()}
	opts.tests.List, _ = f.GetBool("list")
	opts.tests.Filter, _ = f.GetString("filter")
	opts.tests.PerTestTimeout, _ = f.GetDuration("timeout")
	opts.tests.SuiteTimeout, _ = f.GetDuration("suite-timeout")
	opts.tests.JSON, _ = f.GetBool("json")
	return opts, nil
}
