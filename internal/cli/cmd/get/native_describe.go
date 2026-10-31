package get

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"kigumi/internal/toolchain/driver"
)

// printNativeChanges reports every added or removed Source/Link, grouped
// per dependency (the root cause), and the lock diff as JSON when asked.
func printNativeChanges(cmd *cobra.Command, changes []nativeChange, asJSON bool) {
	if len(changes) == 0 {
		return
	}
	cmd.Println("native changes:")
	for _, c := range changes {
		for _, line := range c.Lines {
			cmd.Printf("  %s: %s\n", c.Module, line)
		}
	}
	if asJSON {
		b, _ := json.MarshalIndent(changes, "", "  ")
		cmd.Println(string(b))
	}
}

func describeChange(hasB, hasA bool, b, a driver.ModFile) []string {
	if !hasB {
		return append([]string{"new dependency, trusted for the first time"}, addedLines(a)...)
	}
	if !hasA {
		return []string{"removed; its native declarations no longer apply"}
	}
	var lines []string
	lines = append(lines, diffLinks("+", a.Links, b.Links)...)
	lines = append(lines, diffLinks("-", b.Links, a.Links)...)
	lines = append(lines, diffSources("+", a.Sources, b.Sources)...)
	lines = append(lines, diffSources("-", b.Sources, a.Sources)...)
	return lines
}

func addedLines(mf driver.ModFile) []string {
	var out []string
	for _, l := range mf.Links {
		out = append(out, "+Link "+linkDesc(l))
	}
	for _, src := range mf.Sources {
		out = append(out, "+Source "+src.Path)
	}
	return out
}

func diffLinks(sign string, from, without []driver.Link) []string {
	var out []string
	for _, l := range from {
		if !containsLink(without, l) {
			out = append(out, fmt.Sprintf("%sLink %s", sign, linkDesc(l)))
		}
	}
	return out
}

func diffSources(sign string, from, without []driver.NativeSource) []string {
	var out []string
	for _, s := range from {
		if !containsSource(without, s) {
			out = append(out, fmt.Sprintf("%sSource %s", sign, s.Path))
		}
	}
	return out
}

func containsLink(in []driver.Link, l driver.Link) bool {
	for _, x := range in {
		if x == l {
			return true
		}
	}
	return false
}

func containsSource(in []driver.NativeSource, s driver.NativeSource) bool {
	for _, x := range in {
		if x == s {
			return true
		}
	}
	return false
}

func linkDesc(l driver.Link) string {
	switch {
	case l.Library != "" && l.Search != "":
		return fmt.Sprintf("library=%s search=%s", l.Library, l.Search)
	case l.Library != "":
		return "library=" + l.Library
	default:
		return "search=" + l.Search
	}
}
