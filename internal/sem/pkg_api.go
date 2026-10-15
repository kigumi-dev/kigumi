package sem

import (
	"cmp"
	"slices"
)

// PackageAPI is what a package shows its importers, by name and never by ordinal, so it is stable under unrelated edits.
type PackageAPI struct {
	Path  string    `json:"path"`
	Decls []APIDecl `json:"decls"`
}

type APIDecl struct {
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Vis      string         `json:"vis"`
	Sig      string         `json:"sig,omitempty"`
	Generics []string       `json:"generics,omitempty"`
	Params   []string       `json:"params,omitempty"`
	Effects  string         `json:"effects,omitempty"`
	Ops      *OpSummaryJSON `json:"ops,omitempty"`
	Drop     string         `json:"drop,omitempty"`
	Called   []int          `json:"called,omitempty"`
	Ctx      []string       `json:"ctx,omitempty"`
	Body     string         `json:"body,omitempty"`
	Fields   []string       `json:"fields,omitempty"`
	Variants []string       `json:"variants,omitempty"`
	Reqs     []string       `json:"reqs,omitempty"`
	Impls    []string       `json:"impls,omitempty"`
	Layout   string         `json:"layout,omitempty"`
	Abi      string         `json:"abi,omitempty"`
	Export   string         `json:"export,omitempty"`
	LinkName string         `json:"linkName,omitempty"`
	Section  string         `json:"section,omitempty"`
	Value    string         `json:"value,omitempty"`
	Target   string         `json:"target,omitempty"`
}

// PackageAPIOf summarizes the package at path; ok is false when the checked module has none.
func (r *Result) PackageAPIOf(path string) (PackageAPI, bool) {
	pkg := r.PackageByPath(path)
	if pkg == 0 {
		return PackageAPI{}, false
	}
	api := PackageAPI{Path: path}
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Pkg != pkg || e.File == 0 || e.Flags&EfPoison != 0 || e.Vis.Level == VisPrivate {
			continue
		}
		if _, minted := r.instances[EntityID(id)]; minted {
			continue
		}
		if d, ok := r.apiDecl(EntityID(id)); ok {
			api.Decls = append(api.Decls, d)
		}
	}
	slices.SortFunc(api.Decls, func(a, b APIDecl) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Sig, b.Sig))
	})
	return api, true
}
