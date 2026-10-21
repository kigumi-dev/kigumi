package driver

import (
	"fmt"
	"sort"

	"kigumi/internal/buildgraph"
)

func ToolCapabilities(g *buildgraph.Graph) []string {
	names := map[int]string{}
	used := map[string]bool{}
	for _, f := range g.Frameworks {
		names[f.ID] = f.Name
		used["framework "+f.Name] = true
	}
	for _, s := range g.ToolSteps {
		if s.Kind == "" {
			used[fmt.Sprintf("tool %s.%s", names[s.Framework], s.Tool)] = true
		}
	}
	out := make([]string, 0, len(used))
	for u := range used {
		out = append(out, "build "+u)
	}
	sort.Strings(out)
	return out
}
