package driver

import (
	"fmt"
	"strings"
)

// RenderExamplesTable renders the catalog's README rows in reading order. The
// README and the catalog cannot drift: TestExamplesReadmeMatchesCatalog
// compares this text with the table in examples/README.md.
func RenderExamplesTable(catalog map[string]ExampleMeta, names []string) string {
	var b strings.Builder
	b.WriteString("| Example | Level | Shows | Covers |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, name := range names {
		e := catalog[name]
		fmt.Fprintf(&b, "| [%s](%s/) | %s | %s | %s |\n", e.Name, e.Name, e.Level, e.Shows, strings.Join(e.Features, ", "))
	}
	return b.String()
}
