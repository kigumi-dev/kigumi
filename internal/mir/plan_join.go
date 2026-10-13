package mir

func noteUse(uses map[LocalID]map[int]bool, l LocalID, b int) {
	if uses[l] == nil {
		uses[l] = map[int]bool{}
	}
	uses[l][b] = true
}

// joinPoint releases a temporary read outside its defining block; every path there must pass a definition exactly once.
func (c *joinCtx) joinPoint(defs, uses map[int]bool, last use) (Point, bool) {
	if len(defs) == 0 || len(uses) == 0 {
		return Point{}, false
	}
	if len(uses) == 1 {
		b := sole(uses)
		if !defs[b] && b != 0 && c.cutsOff(defs, b) {
			return Point{last.block, last.inst}, true
		}
		return Point{}, false
	}
	if len(defs) != 1 {
		return Point{}, false
	}
	d := sole(defs)
	j, ok := c.immediatePostDom(d)
	if !ok || !c.cutsOff(defs, j) || c.reaches(d, d, j) || uses[j] && last.block != j {
		return Point{}, false
	}
	for b := range uses {
		if b != d && b != j && !c.reaches(d, b, j) {
			return Point{}, false
		}
	}
	if uses[j] {
		return Point{last.block, last.inst}, true
	}
	return Point{j, -1}, true
}

type joinCtx struct {
	f    *Func
	pdom [][]uint64
}

func newJoinCtx(f *Func) *joinCtx { return &joinCtx{f: f} }

// cutsOff reports that every path from the entry to block b, and every loop through b, passes a block of defs.
func (c *joinCtx) cutsOff(defs map[int]bool, b int) bool {
	return (defs[0] || !c.walk(0, b, defs)) && !c.walk(b, b, defs)
}

func (c *joinCtx) walk(from, to int, avoid map[int]bool) bool {
	seen := map[int]bool{}
	var visit func(b int) bool
	visit = func(b int) bool {
		for _, n := range c.f.Blocks[b].Term.Targets {
			if avoid[int(n)] || seen[int(n)] {
				continue
			}
			if int(n) == to {
				return true
			}
			seen[int(n)] = true
			if visit(int(n)) {
				return true
			}
		}
		return false
	}
	return visit(from)
}

// reaches reports whether block to follows block from without passing block via.
func (c *joinCtx) reaches(from, to, via int) bool {
	return c.walk(from, to, map[int]bool{via: true})
}

func sole(set map[int]bool) int {
	for b := range set {
		return b
	}
	return 0
}
