package sem

// declareContextKeys resolves each `context name: Type` declaration's type;
// a key's identity is its EntityID, never its spelling or value type.
func (r *Result) declareContextKeys() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntContextKey || e.File == 0 || e.Flags&EfPoison != 0 {
			continue
		}
		s := contextKeyDecl(r.tree(e.File), e.Node)
		e.Type = r.resolveType(e.File, r.fileScopes[e.File], s.Type, posField)
	}
}
