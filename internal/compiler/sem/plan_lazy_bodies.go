package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// implicitNames are the std methods operators, `||` and shell plans call without spelling them.
var implicitNames = []string{"equals", "compareTo", "hash", "branch", "planNew", "planPush"}

// planLazyBodies marks the bodies of Module.LazyBodies packages that no eagerly checked body can name, so checkBodies skips them.
func (r *Result) planLazyBodies() {
	if r.mod.LazyBodies == nil {
		return
	}
	r.lazyPkg = make([]bool, len(r.Packages))
	for i := 1; i < len(r.Packages); i++ {
		r.lazyPkg[i] = r.mod.LazyBodies(r.Packages[i].Path)
	}
	r.skipBody = make([]bool, len(r.Entities))
	d := &bodyDemand{r: r, byName: map[string][]EntityID{}, seen: map[demandKey]bool{}, declTok: map[FileID][]bool{}}
	var forced []EntityID
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.File == 0 {
			continue
		}
		if !r.lazyPkg[e.Pkg] {
			d.markDecl(e.File, e.Tok)
			continue
		}
		switch e.Kind {
		case EntTest:
			r.skipBody[id] = true
		case EntFn:
			info := r.Fn(EntityID(id))
			if info.Body == 0 || r.Entities[e.Parent].Kind == EntInterface {
				continue
			}
			if info.Export != "" || info.Naked {
				forced = append(forced, EntityID(id))
			}
			r.skipBody[id] = true
			d.byName[e.Name] = append(d.byName[e.Name], EntityID(id))
		}
	}
	for _, name := range implicitNames {
		d.note([]byte(name), 0)
	}
	for f := 1; f < len(r.Files); f++ {
		if t := r.Files[f].Tree; !r.lazyPkg[r.Files[f].Pkg] {
			d.noteToks(FileID(f), 1, uint32(len(t.Toks)))
		}
	}
	// Drop glue calls a drop fn no body spells.
	for i := range r.TypeDecls {
		forced = append(forced, r.TypeDecls[i].Drop)
	}
	for _, id := range forced {
		if r.BodySkipped(id) {
			r.skipBody[id] = false
			d.noteBody(id)
		}
	}
	d.drain()
}

type bodyDemand struct {
	r      *Result
	byName map[string][]EntityID
	seen   map[demandKey]bool
	queue  []demandKey
	// declTok marks the name tokens of an eager file's declarations, which name nothing in another package.
	declTok map[FileID][]bool
}

// demandKey is a name spelled in pkg, which reaches pkg's private functions as well as every open one; pkg 0 reaches every function.
type demandKey struct {
	name string
	pkg  PackageID
}

func (d *bodyDemand) drain() {
	for len(d.queue) > 0 {
		k := d.queue[len(d.queue)-1]
		d.queue = d.queue[:len(d.queue)-1]
		for _, id := range d.byName[k.name] {
			if d.r.skipBody[id] && (k.pkg == 0 || d.r.Entities[id].Pkg == k.pkg || d.open(id)) {
				d.r.skipBody[id] = false
				d.noteBody(id)
			}
		}
	}
}

// open is a function another package may reach by name: one not private, or a method, which an interface can dispatch to.
func (d *bodyDemand) open(id EntityID) bool {
	return d.r.Entities[id].Vis.Level != VisPrivate || d.r.Fn(id).Owner != 0
}

func (d *bodyDemand) markDecl(f FileID, tok uint32) {
	if tok == 0 {
		return
	}
	marks := d.declTok[f]
	if marks == nil {
		marks = make([]bool, len(d.r.tree(f).Toks))
		d.declTok[f] = marks
	}
	if int(tok) < len(marks) {
		marks[tok] = true
	}
}

func (d *bodyDemand) noteBody(id EntityID) {
	f := d.r.Entities[id].File
	t := d.r.tree(f)
	lo, hi := tokRange(t, d.r.Fn(id).Body)
	// A member name or a closing token follows the last node token.
	d.noteToks(f, lo, min(hi+3, uint32(len(t.Toks))))
}

func tokRange(t *syntax.Tree, n syntax.NodeID) (lo, hi uint32) {
	lo = uint32(len(t.Toks))
	var walk func(v syntax.View)
	walk = func(v syntax.View) {
		if tok := v.Tok().Index(); tok != 0 {
			lo, hi = min(lo, tok), max(hi, tok)
		}
		for ch := range v.Children() {
			walk(ch)
		}
	}
	walk(t.View(n))
	return lo, hi
}

func (d *bodyDemand) noteToks(f FileID, lo, hi uint32) {
	t := d.r.tree(f)
	pkg := d.r.Files[f].Pkg
	if !d.r.lazyPkg[pkg] {
		// Checked code that names another package's private function owes its error.
		pkg = 0
	}
	marks := d.declTok[f]
	src := t.File.Src
	for i := lo; i < hi; i++ {
		tok := t.Toks[i]
		switch tok.Kind {
		case token.EOF:
			// The parser appends synthetic path tokens after EOF.
			return
		case token.Int, token.Float, token.Char:
		case token.String, token.ByteString:
			d.noteWords(src[tok.Start:tok.End], pkg)
		default:
			if int(i) >= len(marks) || !marks[i] {
				d.note(src[tok.Start:tok.End], pkg)
			}
		}
	}
}

// noteWords reads the names an interpolated string may call.
func (d *bodyDemand) noteWords(s []byte, pkg PackageID) {
	start := -1
	for i := 0; i <= len(s); i++ {
		word := i < len(s) && (s[i] == '_' || s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || s[i] >= '0' && s[i] <= '9')
		switch {
		case word && start < 0:
			start = i
		case !word && start >= 0:
			d.note(s[start:i], pkg)
			start = -1
		}
	}
}

// note skips a name no lazy function bears without allocating.
func (d *bodyDemand) note(name []byte, pkg PackageID) {
	if _, ok := d.byName[string(name)]; !ok {
		return
	}
	if k := (demandKey{string(name), pkg}); !d.seen[k] {
		d.seen[k] = true
		d.queue = append(d.queue, k)
	}
}
