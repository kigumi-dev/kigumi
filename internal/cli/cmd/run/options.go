package run

import (
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
)

type options struct {
	session   session.Session
	args      []string
	secretSan bool
}

func parseOptions(cmd *cobra.Command, args []string) (options, error) {
	if err := rejectSwallowedFlags(cmd, args); err != nil {
		return options{}, err
	}
	secretSan, err := cmd.Flags().GetBool("secret-san")
	if err != nil {
		return options{}, err
	}
	return options{session: session.Read(cmd), args: dropDashDash(args), secretSan: secretSan}, nil
}

func scriptOptions(cmd *cobra.Command, args []string) (options, error) {
	return options{session: session.Read(cmd), args: args}, nil
}

// rejectSwallowedFlags reports the first token in args[1:] naming a flag of cmd, which SetInterspersed(false) would
// silently forward to the script; a literal "--" ends the scan.
func rejectSwallowedFlags(cmd *cobra.Command, args []string) error {
	for _, a := range args[1:] {
		if a == "--" {
			break
		}
		var name string
		switch {
		case strings.HasPrefix(a, "--"):
			name = strings.TrimPrefix(a, "--")
			if i := strings.IndexByte(name, '='); i >= 0 {
				name = name[:i]
			}
		case strings.HasPrefix(a, "-") && a != "-":
			name = a[1:2]
		default:
			continue
		}
		if f := cmd.Flags().Lookup(name); f != nil {
			return &exit.UsageError{Err: errors.NewErrf("--%s must come before %s: flags after the entry file are passed to the script, not to kigumi", f.Name, args[0])}
		}
		if len(name) == 1 {
			if f := cmd.Flags().ShorthandLookup(name); f != nil {
				return &exit.UsageError{Err: errors.NewErrf("-%s must come before %s: flags after the entry file are passed to the script, not to kigumi", name, args[0])}
			}
		}
	}
	return nil
}

// dropDashDash removes the first literal "--" in args[1:]: with SetInterspersed(false) pflag leaves it in args,
// where it would leak into the script's argv.
func dropDashDash(args []string) []string {
	for i, a := range args[1:] {
		if a == "--" {
			out := make([]string, 0, len(args)-1)
			out = append(out, args[:i+1]...)
			return append(out, args[i+2:]...)
		}
	}
	return args
}
