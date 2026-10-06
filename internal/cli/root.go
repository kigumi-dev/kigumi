// Package cli assembles the kigumi command tree.
package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/build"
	"kigumi/internal/cli/check"
	"kigumi/internal/cli/doc"
	"kigumi/internal/cli/doctor"
	"kigumi/internal/cli/explain"
	"kigumi/internal/cli/fmtcmd"
	"kigumi/internal/cli/get"
	"kigumi/internal/cli/lsp"
	"kigumi/internal/cli/mod"
	"kigumi/internal/cli/parse"
	"kigumi/internal/cli/run"
	"kigumi/internal/cli/shared"
	"kigumi/internal/cli/zig"
	"kigumi/internal/cliutil"
	"kigumi/internal/diag"
	"kigumi/internal/errors"
	"kigumi/internal/version"
)

// RootCmd builds the root; a bare `kigumi file.kg [args]` runs the file,
// which is what a shebang line passes.
func RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kigumi",
		Short:         "The Kigumi compiler, interpreter and tooling",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case len(args) == 0:
				return cmd.Help()
			case strings.HasSuffix(args[0], ".kg"):
				return run.Script(cmd, args)
			}
			return &cliutil.UsageError{Err: errors.NewErrf("unknown command %q", args[0])}
		},
	}
	root.Flags().SetInterspersed(false)
	root.PersistentFlags().String("std", "", "directory holding the std stubs (default: $KIGUMI_STD, ./std, or next to the executable)")
	root.PersistentFlags().BoolP("debug", "d", false, "print stack traces with internal errors")
	root.PersistentFlags().Bool("no-local", false, "ignore mod.local.kg and resolve dependencies as users of the module would")
	root.PersistentFlags().String("target", "", "target triple such as x86_64-linux-musl or x86_64-freestanding (default: the host)")
	root.PersistentFlags().String("sys", "", "runtime system layer, posix or none (default: from the target)")
	root.PersistentFlags().String("allocator", "", "default allocator profile: general or none (none rejects allocation outside `allocator` blocks)")
	root.PersistentFlags().String("profile", "", "allocation-failure profile: fatal-permitted (default) or recoverable-only (rejects any path that terminates on a refused allocation instead of returning it as a value)")
	root.PersistentFlags().String("blocking-executor", "", "async/blocking execution-agent profile: hosted (default, warns on a direct-blocking operation reachable from an async body) or a strict profile that rejects it instead - isr, alloc-free-executor, single-thread-event-loop")
	root.PersistentFlags().String("stack-profile", "", "stack policy: hosted-growable (default, reports the resource summary only) or strict (rejects an entry whose call chain is unbounded or unknown)")
	root.PersistentFlags().Int("stack-budget-kib", 0, "with --stack-profile strict, the available stack capacity in KiB a bounded entry must additionally fit under (default: no numeric budget)")
	root.PersistentFlags().String("color", "auto", "color diagnostics: auto (a terminal without NO_COLOR), always, or never")
	root.PersistentFlags().Bool("offline", false, "never fetch a pinned zig on demand; report a diagnostic instead")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		mode, _ := cmd.Flags().GetString("color")
		color, err := shared.Color(mode, os.Stderr)
		if err != nil {
			return &cliutil.UsageError{Err: err}
		}
		diag.UseColor(color)
		return nil
	}
	root.Version = version.Toolchain
	root.SetVersionTemplate("kigumi version {{.Version}}\n")
	root.AddCommand(parse.Cmd(), fmtcmd.Cmd(), check.Cmd(), run.Cmd(), run.TestCmd(), build.Cmd(), doc.Cmd(), doc.SchemaCmd(), get.Cmd(), mod.Cmd(), lsp.Cmd(), explain.Cmd(), doctor.Cmd(), zig.Cmd())
	return root
}

// Debug reports whether --debug was given on the root.
func Debug(root *cobra.Command) func() bool {
	return func() bool {
		v, _ := root.PersistentFlags().GetBool("debug")
		return v
	}
}
