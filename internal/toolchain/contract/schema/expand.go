package schema

import (
	"sort"
	"strings"

	"kigumi/internal/compiler/sem"
)

// GeneratedFile is one source file the checker generated, with the derive
// requests that contributed to it; `kigumi expand` prints these.
type GeneratedFile struct {
	Package     string              `json:"package"`
	Name        string              `json:"name"`
	ContentHash string              `json:"contentHash"`
	Text        string              `json:"text"`
	Sources     []GeneratedArtifact `json:"sources"`
}

// GeneratedFiles lists res's generated source files in (package, name) order,
// each with the provenance BuildProvenance records for its package and kind.
func GeneratedFiles(res *sem.Result, byEntity map[sem.EntityID]*Declaration) []GeneratedFile {
	provenance := BuildProvenance(res, byEntity)
	out := []GeneratedFile{}
	for _, g := range res.GeneratedSources() {
		file := GeneratedFile{
			Package:     g.Package,
			Name:        g.Name,
			ContentHash: digestBytes([]byte(g.Text)),
			Text:        g.Text,
			Sources:     []GeneratedArtifact{},
		}
		for _, art := range provenance {
			if art.Package == g.Package && strings.HasSuffix(g.Name, art.File) {
				file.Sources = append(file.Sources, art)
			}
		}
		sort.SliceStable(file.Sources, func(i, j int) bool {
			if a, b := file.Sources[i].Generator, file.Sources[j].Generator; a != b {
				return a < b
			}
			return originID(file.Sources[i]) < originID(file.Sources[j])
		})
		out = append(out, file)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func originID(a GeneratedArtifact) string {
	if a.Origin == nil {
		return ""
	}
	return a.Origin.ID
}
