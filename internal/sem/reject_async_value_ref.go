package sem

import "kigumi/internal/syntax"

// A call through an async fn's value lends each `&mut` argument to its Future, as a direct async call does.
func (c *checker) noteAsyncValue(n syntax.NodeID, fn EntityID) {
	c.r.asyncValues = append(c.r.asyncValues, refCellSite{ent: fn, file: c.f, node: n, async: true})
}

// maxAsyncValueArgs is how many arguments a call through a value declares lent or handed over (mir.Inst.LendArgs).
const maxAsyncValueArgs = 31

// rejectReplacingAsyncValues reports E1504 for an async fn value past that arity and E1501 where an
// async fn that replaces a `&mut` parameter as a whole is taken as a value.
func (r *Result) rejectReplacingAsyncValues() {
	for _, s := range r.asyncValues {
		info := r.Fn(s.ent)
		n := len(info.Params)
		if info.Recv != RecvNone {
			n++
		}
		if n > maxAsyncValueArgs {
			r.errAt(s.file, s.node, cAsyncValueArity, r.Entities[s.ent].Name, n)
			continue
		}
		for _, p := range info.Params {
			if !r.refCellParams[p] {
				continue
			}
			if d, ok := r.diagAt(s.file, s.node, cRefCellAsyncArg); ok {
				d.Msg = "`" + r.Entities[s.ent].Name + "` replaces its `&mut` parameter `" + r.Entities[p].Name + "` as a whole, so it cannot be used as a value: a call through the value lends the `&mut` to a Future that outlives the storage the new value would go back to"
				d.Help = []string{"take the value itself instead of a `&mut`, or update the referent in place"}
				r.emitDiag(s.file, d)
			}
			break
		}
	}
}
