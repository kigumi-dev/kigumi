package get

import (
	"encoding/json"
	"fmt"
	"io"

	"kigumi/internal/toolchain/module"
)

// printNativeChanges reports every added or removed Source/Link per dependency, and the lock diff as JSON when asked.
func printNativeChanges(w io.Writer, changes []nativeChange, asJSON bool) {
	if len(changes) == 0 {
		return
	}
	fmt.Fprintln(w, "native changes:")
	for _, c := range changes {
		for _, line := range c.Lines {
			fmt.Fprintf(w, "  %s: %s\n", c.Module, line)
		}
	}
	if asJSON {
		b, _ := json.MarshalIndent(changes, "", "  ")
		fmt.Fprintln(w, string(b))
	}
}

func describeChange(hasB, hasA bool, b, a module.ModFile) []string {
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

func addedLines(mf module.ModFile) []string {
	var out []string
	for _, l := range mf.Links {
		out = append(out, "+Link "+linkDesc(l))
	}
	for _, src := range mf.Sources {
		out = append(out, "+Source "+src.Path)
	}
	return out
}

func diffLinks(sign string, from, without []module.Link) []string {
	var out []string
	for _, l := range from {
		if !containsLink(without, l) {
			out = append(out, fmt.Sprintf("%sLink %s", sign, linkDesc(l)))
		}
	}
	return out
}

func diffSources(sign string, from, without []module.NativeSource) []string {
	var out []string
	for _, s := range from {
		if !containsSource(without, s) {
			out = append(out, fmt.Sprintf("%sSource %s", sign, s.Path))
		}
	}
	return out
}

func containsLink(in []module.Link, l module.Link) bool {
	for _, x := range in {
		if x == l {
			return true
		}
	}
	return false
}

func containsSource(in []module.NativeSource, s module.NativeSource) bool {
	for _, x := range in {
		if x == s {
			return true
		}
	}
	return false
}

func linkDesc(l module.Link) string {
	switch {
	case l.Library != "" && l.Search != "":
		return fmt.Sprintf("library=%s search=%s", l.Library, l.Search)
	case l.Library != "":
		return "library=" + l.Library
	default:
		return "search=" + l.Search
	}
}
