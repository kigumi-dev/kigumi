// Package build implements `kigumi build`: compile a module ahead of time
// through LLVM IR, or print the IR.
package build

import (
	"github.com/spf13/cobra"

	"kigumi/internal/cli/session"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [-o <out>] [--emit-llvm] [--target <triple>] [--reproducible] <module-root|entry.kg> [-- <flags for build.kg>]",
		Short: "Compile a module to a native executable, or an archive for a freestanding or `--sys none` target; a build.kg drives the build instead",
		Args:  cobra.MinimumNArgs(1),
		RunE:  session.RunE(parseOptions, run),
	}
	cmd.Flags().StringP("output", "o", "", "output path (default: a.out, or kigumi.a for a freestanding or --sys none target)")
	cmd.Flags().Bool("emit-llvm", false, "print the LLVM IR instead of linking")
	cmd.Flags().Bool("plan", false, "with a build.kg, print the resolved build graph as JSON and stop")
	cmd.Flags().String("out-dir", ".", "with a build.kg, the directory artifacts and generated files go to")
	cmd.Flags().StringArray("secret", nil, "with a build.kg, name=file of a secret a tool step may read")
	cmd.Flags().Bool("reproducible", false, "build twice, each with a fresh build dir and compiler cache, and fail if the outputs differ")
	cmd.Flags().String("link", "auto", "C library form of a hosted executable: auto (a static musl single binary unless std/dl, a native Link or Source, a sanitizer, a host library flag or a header only the host translates needs the host's C library), static, or dynamic; with --target it must agree with the triple's ABI")
	cmd.Flags().Bool("size-min", false, "minimise the output: -Os, LTO, section GC, no unwind tables, stripped (drops symbol names and the unwind tables backtraces use)")
	session.ReleaseFlag(cmd)
	return cmd
}
