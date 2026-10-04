package doctor

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"kigumi/internal/cliutil"
	"kigumi/internal/errors"
	"kigumi/internal/version"
)

// Cmd implements `kigumi doctor`: a read-only report of resolved std root, target, C compiler, mod cache and provenance.
func Cmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Report the resolved toolchain, std root, target and cache",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := Build(cmd)
			switch format {
			case "text":
				renderText(cmd.OutOrStdout(), r)
			case "json":
				data, err := json.MarshalIndent(version.Wrap("doctor", version.DoctorFormatVersion, r), "", "  ")
				if err != nil {
					return errors.WrapErr(err, "encode doctor report")
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			default:
				return &cliutil.UsageError{Err: errors.NewErrf("--format: want text or json, got %q", format)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text (default) or json")
	return cmd
}

func renderText(w io.Writer, r Report) {
	fmt.Fprintf(w, "release:    %s\n", r.Release)
	fmt.Fprintf(w, "commit:     %s%s\n", r.Commit, dirtySuffix(r.Dirty))
	fmt.Fprintf(w, "go:         %s\n", r.GoVersion)
	printField(w, "std", r.Std)
	printField(w, "target", r.Target)
	printField(w, "c compiler", r.CCompiler)
	printField(w, "mod cache", r.ModCache)
	fmt.Fprintf(w, "format versions: schema=%d modJson=%d buildPlan=%d testEvent=%d\n",
		r.Formats.Schema, r.Formats.ModJSON, r.Formats.BuildPlan, r.Formats.TestEvent)
}

func dirtySuffix(dirty bool) string {
	if dirty {
		return " (dirty)"
	}
	return ""
}

func printField(w io.Writer, name string, f Field) {
	value := f.Value
	if value == "" {
		value = "-"
	}
	fmt.Fprintf(w, "%-10s  %-40s  [%s] %s\n", name, value, f.Status, f.Source)
}
