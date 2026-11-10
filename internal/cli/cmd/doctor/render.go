package doctor

import (
	"fmt"
	"io"
)

func renderText(w io.Writer, r report) {
	fmt.Fprintf(w, "release:    %s\n", r.Release)
	fmt.Fprintf(w, "commit:     %s%s\n", r.Commit, dirtySuffix(r.Dirty))
	fmt.Fprintf(w, "go:         %s\n", r.GoVersion)
	printField(w, "module", r.Module.field)
	printField(w, "std", r.Std)
	printField(w, "target", r.Target)
	printField(w, "c compiler", r.CCompiler)
	printField(w, "zig", r.Zig)
	printField(w, "mod cache", r.ModCache)
	fmt.Fprintf(w, "format versions: schema=%d modJson=%d buildPlan=%d testEvent=%d\n",
		r.Formats.Schema, r.Formats.ModJSON, r.Formats.BuildPlan, r.Formats.TestEvent)
	// Only printed when there is something to fix, so a report whose versions
	// all agree keeps the lines it had before diagnostics existed.
	for _, d := range r.Diagnostics {
		fmt.Fprintf(w, "diagnostic: [%s] %s (%s)\n", d.Severity, d.Message, d.Code)
	}
}

func dirtySuffix(dirty bool) string {
	if dirty {
		return " (dirty)"
	}
	return ""
}

func printField(w io.Writer, name string, f field) {
	value := f.Value
	if value == "" {
		value = "-"
	}
	fmt.Fprintf(w, "%-10s  %-40s  [%s] %s\n", name, value, f.Status, f.Source)
}
