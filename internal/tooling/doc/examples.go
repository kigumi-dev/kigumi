package doc

import (
	"strings"

	"kigumi/internal/toolchain/driver"
)

// ExamplesCatalogMarkdown renders examples/catalog.json as one Markdown page, grouped by kind.
func ExamplesCatalogMarkdown(examplesRoot string) (string, error) {
	catalog, names, err := driver.LoadExampleCatalog(examplesRoot)
	if err != nil {
		return "", err
	}
	byKind := map[string][]string{}
	for _, name := range names {
		k := catalog[name].Kind
		byKind[k] = append(byKind[k], name)
	}
	var sb strings.Builder
	sb.WriteString("# Examples\n")
	for _, kind := range driver.ValidKinds {
		group := byKind[kind]
		if len(group) == 0 {
			continue
		}
		sb.WriteString("\n## " + strings.ToUpper(kind[:1]) + kind[1:] + "\n")
		for _, name := range group {
			e := catalog[name]
			sb.WriteString("\n### " + name + "\n\n" + e.Shows + "\n\n")
			sb.WriteString("- level: `" + e.Level + "`\n")
			sb.WriteString("- oracle: `" + e.Oracle + "`\n")
			sb.WriteString("- backends: `" + strings.Join(e.Backends, ", ") + "`\n")
			if len(e.Modes) > 0 {
				sb.WriteString("- modes: `" + strings.Join(e.Modes, ", ") + "`\n")
			}
			if len(e.Requires) > 0 {
				sb.WriteString("- requires: `" + strings.Join(e.Requires, ", ") + "`\n")
			}
			sb.WriteString("- covers: `" + strings.Join(e.Features, ", ") + "`\n")
			if e.Reason != "" {
				sb.WriteString("- " + e.Reason + "\n")
			}
		}
	}
	return sb.String(), nil
}
