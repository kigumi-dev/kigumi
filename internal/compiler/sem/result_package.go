package sem

import (
	"strings"

	"kigumi/internal/lang/diag"
)

// RenderPackage renders one package's diagnostics as Render does for the module; "" when it is clean or absent.
func (r *Result) RenderPackage(path string) string {
	pkg := r.PackageByPath(path)
	if pkg == 0 {
		return ""
	}
	var sb strings.Builder
	for _, f := range r.Packages[pkg].Files {
		sb.WriteString(diag.RenderAll(r, r.Files[f].Diags))
	}
	return sb.String()
}

// PackageDiagCounts counts a package's diagnostics and the errors among them.
func (r *Result) PackageDiagCounts(path string) (total, errors int) {
	pkg := r.PackageByPath(path)
	if pkg == 0 {
		return 0, 0
	}
	for _, f := range r.Packages[pkg].Files {
		for _, d := range r.Files[f].Diags {
			total++
			if d.Severity == diag.Error {
				errors++
			}
		}
	}
	return total, errors
}

// ComptimePackages holds the paths of the packages with a comptime block.
func (r *Result) ComptimePackages() map[string]bool {
	out := map[string]bool{}
	for _, site := range r.Comptime {
		out[r.Packages[r.Files[site.File].Pkg].Path] = true
	}
	return out
}
