package sem

// pat is the abstract pattern of the usefulness algorithm (Maranget 2007):
// a wildcard, a constructor with sub-patterns, or a literal from an
// unbounded domain.
type pat struct {
	kind uint8
	key  string
	name string
	args []pat
}

const (
	pWild uint8 = iota
	pCtor
	pLit
	pRange
	pErr
)

func hasErr(p pat) bool {
	if p.kind == pErr {
		return true
	}
	for _, a := range p.args {
		if hasErr(a) {
			return true
		}
	}
	return false
}

// ctorDesc is one constructor of a column type with its argument types.
type ctorDesc struct {
	key  string
	name string
	args []TypeID
}

func wilds(n int) []pat {
	out := make([]pat, n)
	return out
}

func (d ctorDesc) pat() pat {
	return pat{kind: pCtor, key: d.key, name: d.name, args: wilds(len(d.args))}
}

// matrix is a pattern matrix grouped by the head constructor of its first
// column: the wildcard rows, and the rows of each constructor key. Grouping
// once per level keeps specialization proportional to the rows it keeps, so a
// flat match over N variants and N arms stays linear instead of scanning every
// row once per variant.
type matrix struct {
	rows  [][]pat
	byKey map[string][]int
	wild  []int
}

func newMatrix(rows [][]pat) matrix {
	m := matrix{rows: rows}
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		if row[0].kind == pWild {
			m.wild = append(m.wild, i)
			continue
		}
		if m.byKey == nil {
			m.byKey = map[string][]int{}
		}
		m.byKey[row[0].key] = append(m.byKey[row[0].key], i)
	}
	return m
}

// specialize keeps the rows whose head is h (or a wildcard) and expands the
// head into its sub-patterns.
func (m matrix) specialize(h pat) [][]pat {
	heads := m.byKey[h.key]
	out := make([][]pat, 0, len(heads)+len(m.wild))
	arity := len(h.args)
	for _, i := range m.wild {
		row := m.rows[i]
		n := make([]pat, 0, arity+len(row)-1)
		n = append(n, wilds(arity)...)
		out = append(out, append(n, row[1:]...))
	}
	for _, i := range heads {
		row := m.rows[i]
		n := make([]pat, 0, len(row[0].args)+len(row)-1)
		n = append(n, row[0].args...)
		out = append(out, append(n, row[1:]...))
	}
	return out
}

// defaultMatrix keeps the wildcard rows of the first column.
func (m matrix) defaultMatrix() [][]pat {
	out := make([][]pat, 0, len(m.wild))
	for _, i := range m.wild {
		out = append(out, m.rows[i][1:])
	}
	return out
}

// allPresent says every constructor of set heads a row.
func (m matrix) allPresent(set []ctorDesc) bool {
	for _, d := range set {
		if _, ok := m.byKey[d.key]; !ok {
			return false
		}
	}
	return true
}

// useful reports whether row q matches some value that no row of P matches,
// over columns of the given types.
func (c *checker) useful(P matrix, q []pat, types []TypeID) bool {
	if len(q) == 0 {
		return len(P.rows) == 0
	}
	h := q[0]
	if h.kind != pWild {
		args := c.argTypes(types[0], h)
		next := make([]TypeID, 0, len(args)+len(types)-1)
		next = append(next, args...)
		next = append(next, types[1:]...)
		return c.useful(newMatrix(P.specialize(h)), append(append([]pat{}, h.args...), q[1:]...), next)
	}
	set, complete := c.ctorSet(types[0])
	if complete && P.allPresent(set) {
		for _, d := range set {
			next := make([]TypeID, 0, len(d.args)+len(types)-1)
			next = append(next, d.args...)
			next = append(next, types[1:]...)
			if c.useful(newMatrix(P.specialize(d.pat())), append(wilds(len(d.args)), q[1:]...), next) {
				return true
			}
		}
		return false
	}
	return c.useful(newMatrix(P.defaultMatrix()), q[1:], types[1:])
}

// missing returns a witness row that P leaves uncovered, or ok when P is
// exhaustive over the column types.
func (c *checker) missing(P matrix, types []TypeID) ([]pat, bool) {
	if len(types) == 0 {
		return nil, len(P.rows) > 0
	}
	set, complete := c.ctorSet(types[0])
	if complete && P.allPresent(set) {
		for _, d := range set {
			w, ok := c.missing(newMatrix(P.specialize(d.pat())), append(append([]TypeID{}, d.args...), types[1:]...))
			if !ok {
				head := pat{kind: pCtor, key: d.key, name: d.name, args: w[:len(d.args)]}
				return append([]pat{head}, w[len(d.args):]...), false
			}
		}
		return nil, true
	}
	w, ok := c.missing(newMatrix(P.defaultMatrix()), types[1:])
	if ok {
		return nil, true
	}
	head := pat{kind: pWild}
	if complete {
		for _, d := range set {
			if _, present := P.byKey[d.key]; !present {
				head = d.pat()
				break
			}
		}
	}
	return append([]pat{head}, w...), false
}
