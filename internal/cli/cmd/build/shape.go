package build

import (
	"maps"
	"slices"

	"kigumi/internal/toolchain/driver"
)

// outputShape is the --link and --size-min choice of one kigumi build.
type outputShape struct {
	link    driver.LinkMode
	sizeMin bool
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
