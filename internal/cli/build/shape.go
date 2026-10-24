package build

import (
	"maps"
	"slices"

	"github.com/spf13/cobra"

	"kigumi/internal/cliutil"
	"kigumi/internal/driver"
)

// outputShape is the --link and --size-min choice of one kigumi build.
type outputShape struct {
	link    driver.LinkMode
	sizeMin bool
}

func shapeFlags(cmd *cobra.Command) func() (outputShape, error) {
	link := cmd.Flags().String("link", "auto", "C library form of a hosted executable: auto (a static musl single binary unless std/dl, a native Link or Source, a sanitizer, a host library flag or a header only the host translates needs the host's C library), static, or dynamic; with --target it must agree with the triple's ABI")
	sizeMin := cmd.Flags().Bool("size-min", false, "minimise the output: -Os, LTO, section GC, no unwind tables, stripped (drops symbol names and the unwind tables backtraces use)")
	return func() (outputShape, error) {
		mode, err := driver.ParseLinkMode(*link)
		if err != nil {
			return outputShape{}, &cliutil.UsageError{Err: err}
		}
		return outputShape{link: mode, sizeMin: *sizeMin}, nil
	}
}

// shapeLines are the summary's linking and size lines, by artifact name ("" for a module without build.kg).
func shapeLines(reports map[string]driver.BuildReport) (linking, size []string) {
	for _, name := range slices.Sorted(maps.Keys(reports)) {
		r := reports[name]
		prefix := ""
		if name != "" {
			prefix = name + ": "
		}
		if l := r.Linkage.String(); l != "" {
			linking = append(linking, prefix+l)
		}
		size = append(size, prefix+r.SizeLine())
	}
	return linking, size
}
