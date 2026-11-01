package get

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
)

type options struct {
	session  session.Session
	target   string
	add      string
	url      string
	update   bool
	lock     bool
	diffJSON bool
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	opts := options{session: session.Read(cmd), target: "."}
	for _, a := range args {
		if st, err := os.Stat(a); err == nil && (st.IsDir() || strings.HasSuffix(a, ".kg")) {
			opts.target = a
		} else if strings.HasSuffix(a, ".kg") {
			return opts, &exit.UsageError{Err: errors.NewErrf("no such file: %s", a)}
		} else if opts.add == "" {
			opts.add = a
		} else {
			return opts, &exit.UsageError{Err: errors.NewErrf("%s is neither a module path nor an existing module root or script", a)}
		}
	}
	f := cmd.Flags()
	opts.update, _ = f.GetBool("update")
	opts.lock, _ = f.GetBool("lock")
	opts.url, _ = f.GetString("url")
	opts.diffJSON, _ = f.GetBool("diff-json")
	return opts, nil
}
