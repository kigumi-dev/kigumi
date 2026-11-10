package sem

// captureRole is what a closure holds of a captured local.
type captureRole uint8

const (
	// roleOwned: a copy or the moved value, which leaves with the closure.
	roleOwned captureRole = iota
	// roleFrameRead: the local read in place; the frame that made the closure keeps owning it.
	roleFrameRead
	// roleCell: a cell the closure shares with the frame that made it.
	roleCell
)

func roleOf(cp Capture) captureRole {
	switch cp.Mode {
	case capCell:
		return roleCell
	case capRead, capBorrow:
		return roleFrameRead
	}
	return roleOwned
}

// frameOf is the fn or closure whose frame holds local: the innermost lambda whose body declares it, else its fn.
func (r *Result) frameOf(local EntityID) EntityID {
	e := &r.Entities[local]
	if (e.Kind == EntLocal || e.Kind == EntParam) && e.Detail != 0 {
		for s := r.local(local).Scope; s != 0; s = r.Scopes[s].Parent {
			if sc := &r.Scopes[s]; sc.Kind == ScopeLambda {
				if cl := r.Files[sc.File].Defs[sc.Node]; cl != 0 {
					return cl
				}
			}
		}
	}
	return e.Parent
}

// closureThreadWhy names the first capture, in capture order, that keeps a closure from leaving site's frame.
func (r *Result) closureThreadWhy(t TypeID, sync bool, site EntityID) string {
	what := "cannot move to another thread"
	if sync {
		what = "cannot be shared between threads"
	}
	if r.Types.Kind(t) != KClosure {
		return "it " + what
	}
	cl := r.Types.Node(t).Ent
	for _, cp := range r.closure(cl).Captures {
		e := &r.Entities[cp.Local]
		name := "`" + e.Name + "`"
		if r.isCtxProvider(cp.Local) {
			name = "the context provider bound around it"
		}
		switch {
		case roleOf(cp) == roleCell:
			return "it shares " + name + " with the code around it through a cell, which " + what
		case !r.threadProp(e.Type, sync):
			return "it captures " + name + ", a `" + r.TypeString(e.Type) + "`, which " + what
		case sync:
		case roleOf(cp) == roleFrameRead && !r.readReleased(ReadSite{cl, cp.Local}, site):
			return "it reads " + name + " where the code around it keeps it; a `move` closure takes its own"
		case roleOf(cp) == roleOwned:
			for _, rs := range r.ReadSites(e.Type) {
				if !r.readReleased(rs, site) {
					return "it captures " + name + ", which reads `" + r.Entities[rs.Local].Name + "` where the code around it keeps it"
				}
			}
		}
	}
	return "it " + what
}
